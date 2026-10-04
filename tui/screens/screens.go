// Package screens holds one Bubble Tea sub-model per registered BCTX screen
// (design §2). A screen is PRESENTATION + INTERACTION only (AGENTS §12): it
// builds from tui/components, holds only ephemeral presentation state, and
// requests all work through the shared app/ seam via cancellable command
// factories. No screen runs SQL, dials the network, builds a graph, or computes
// risk — it renders the structured results the services hand back.
//
// Every value a screen shows comes from a live service seam. The only fabricated
// data permitted anywhere is a test fixture, and that lives in *_test.go.
//
// Screens are constructed by the registry factories in tui/router.go, which pass
// a *ScreenCtx carrying the shared app seam, the resolved theme styles, the
// current breakpoint, and the cancellable program context. A screen must behave
// honestly when the app seam is absent (e.g. no active case, or a bare AppCtx in
// tests): it shows an empty/degraded state rather than panicking.
package screens

import (
	"context"
	"errors"
	"strings"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// errNoCase is returned by a command when no active case repository is wired.
// Screens surface it honestly ("no active case") rather than fabricating data.
var errNoCase = errors.New("no active case — open or create one with 'bctx case'")

// errNoSubject is shown when a subject-scoped screen has no selected subject.
var errNoSubject = errors.New("no subject selected")

// errReportNotFound is surfaced when a selected report row cannot be loaded
// (e.g. it was removed between listing and selection).
var errReportNotFound = errors.New("report not found in active case")

// modelsDir resolves the ML models directory for the orchestrator/registry,
// delegating to the shared app seam. A nil cfg (tests) yields "" so model loads
// fail closed and MODELS reports MISSING honestly.
func modelsDir(cfg *configs.Config) string {
	if cfg == nil {
		return ""
	}
	return app.ModelsDir(cfg)
}

// acquisitionAllowed reports whether acquisition may run for the active config,
// via the shared offline gate (app.AcquisitionAllowed). The Monitoring (start
// session) and Data (sync) screens call this BEFORE any provider construction
// (design §12 invariant 1); on error they block with an honest message and
// never attempt a fetch. A nil cfg is treated as not-allowed (fail closed).
func (c *ScreenCtx) acquisitionAllowed() error {
	if c == nil || c.Cfg == nil {
		return app.AcquisitionAllowed(mustOfflineCfg())
	}
	return app.AcquisitionAllowed(c.Cfg)
}

// mustOfflineCfg returns a config whose mode forbids acquisition, used as the
// fail-closed default when no cfg is wired.
func mustOfflineCfg() *configs.Config {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	return cfg
}

// Model is the screen contract this package implements. It is structurally the
// same as tui.Screen but defined here so the screens package never imports tui
// (tui imports screens for its registry factories; the reverse would be an
// import cycle). tui wraps a Model in a thin adapter that satisfies tui.Screen.
// Update returns a Model so a screen can swap itself (bubbletea value-model
// idiom) without the adapter losing the concrete type.
type Model interface {
	Init() tea.Cmd
	Update(tea.Msg) (Model, tea.Cmd)
	View(components.Frame) string
	ShortHelp() []key.Binding
}

// ScreenCtx is the context every screen factory receives. It mirrors the fields
// tui.AppCtx carries; tui.Build adapts its AppCtx into this struct so the
// screens package never imports tui (which would create an import cycle:
// tui imports screens via its registry factories).
type ScreenCtx struct {
	// Ctx is the program context (tea.WithContext). Screens derive child
	// contexts from it for cancellable service commands; it is cancelled on
	// quit so no goroutine outlives the program.
	Ctx context.Context
	// App is the shared bootstrap seam. It may be nil (tests / no wiring); a
	// screen must degrade honestly rather than dereference a nil App.
	App *app.App
	// Cfg is the active, flag-folded configuration (offline/airgap already
	// applied upstream by loadConfig). It drives the offline gate and the
	// models-dir resolution. May be nil in tests.
	Cfg *configs.Config
	// Styles are the resolved theme styles for the active profile.
	Styles theme.Styles
	// Subject is the globally selected investigation subject at build time.
	Subject Subject
	// Compact reports whether the current breakpoint collapses split panes.
	Compact bool
	// Summarizer is the OPTIONAL local-LLM narrator for the :explain pane
	// (design §E). It is copied from tui.AppCtx by toScreenCtx. The screens
	// package imports ONLY package llm (the interface), never llm/ollama, so no
	// transport enters the tui/screens closure. A nil value is treated as
	// llm.NewDeterministic() by explainCmd, so the pane works with no wiring.
	Summarizer llm.Summarizer
	// Acquire is the OPTIONAL network-acquisition seam (app.AcquireService). It
	// is the ONLY network-touching dependency a screen may hold, and only via
	// the interface — the concrete provider-constructing impl is injected from
	// cli/commands so no provider/net import enters the tui/screens closure. A
	// nil value means offline-only: screens must NOT offer network acquisition.
	Acquire app.AcquireService
}

// SubjectBlock is the block subject kind (block hash or height).
const SubjectBlock = "block"

// Subject mirrors the globally selected subject (tui.Subject). It is duplicated
// as a tiny value type so the screens package depends only on app/components,
// never on tui.
type Subject struct {
	ID   string
	Kind string
}

// Subject kinds, matching tui.SubjectKind string values.
const (
	SubjectWallet = "wallet"
	SubjectTx     = "tx"
	SubjectIP     = "ip"
	SubjectEntity = "entity"
)

// bgCtx returns the screen context or a background context so a command never
// dereferences a nil context in tests.
func (c *ScreenCtx) bgCtx() context.Context {
	if c == nil || c.Ctx == nil {
		return context.Background()
	}
	return c.Ctx
}

// repo returns the active-case repository, or nil when no case/app is wired.
func (c *ScreenCtx) repo() sdk.Repository {
	if c == nil || c.App == nil {
		return nil
	}
	return c.App.Repo
}

// caseID returns the active case id, or "" when none is open.
func (c *ScreenCtx) caseID() string {
	if c == nil || c.App == nil {
		return ""
	}
	return c.App.CaseID
}

// summarizer returns the context's OPTIONAL local-LLM summarizer, or nil when
// none is wired (a bare ScreenCtx in tests). explainCmd normalizes a nil value
// to llm.NewDeterministic(), so the pane always works with no network call.
func (c *ScreenCtx) summarizer() llm.Summarizer {
	if c == nil {
		return nil
	}
	return c.Summarizer
}

// styles returns the context styles, defaulting to the standard theme when the
// context is nil (keeps a zero-value test build renderable).
func (c *ScreenCtx) styles() theme.Styles {
	if c == nil {
		return theme.Build(theme.Default())
	}
	return c.Styles
}

// shortID truncates a long id for compact display (mirrors the component helper).
func shortID(id string) string {
	if len(id) <= 14 {
		return id
	}
	return id[:8] + "…" + id[len(id)-3:]
}

// placeholderView renders a titled, honest "no data / needs a subject" body so a
// screen that has not been given a subject yet (or has no wired services) shows
// a clear state rather than a blank or fabricated panel. It never invents data.
func placeholderView(styles theme.Styles, title, msg string, f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.Title.Render(title))
	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render(msg))
	return clampBlockLocal(b.String(), f)
}

// clampBlockLocal is a screens-local clamp mirroring the component guard so a
// screen's own composed strings never overflow their Frame.
func clampBlockLocal(s string, f components.Frame) string {
	if f.Empty() {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > f.H {
		lines = lines[:f.H]
	}
	for i := range lines {
		lines[i] = clampLineLocal(lines[i], f.W)
	}
	return strings.Join(lines, "\n")
}

// clampLineLocal truncates a line to at most w DISPLAY CELLS using lipgloss
// width accounting, so ANSI escape sequences (colour/border styling) are not
// counted as visible width. The previous rune-count implementation counted the
// escape bytes as runes, so a styled line that was visually <= w cells measured
// far larger and got cut mid-content (leaving dangling escapes and borders that
// ran off the frame) — the dashboard's broken/empty right panels at every size.
func clampLineLocal(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r)) > w {
		r = r[:len(r)-1]
	}
	return string(r)
}

// noBinding is a reusable empty ShortHelp result.
var noBinding = []key.Binding(nil)

// paneInner returns the body height available inside a shared components.Panel
// (which spends 2 rows on the border + 1 on its title). It floors at 1 so a
// tight pane still renders a line instead of nothing.
func paneInner(h int) int {
	inner := h - 3
	if inner < 1 {
		inner = 1
	}
	return inner
}
