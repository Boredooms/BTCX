package tui

import (
	"context"
	"strings"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/network/state"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/screens"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// app.go is the top-level Bubble Tea state machine (design §4). The Root owns
// GLOBAL state only — the active case, the selected subject, the global
// notification queue, and the live network/acquisition/model/geoip status.
// Everything else is ephemeral and lives in the active Screen/components. The
// Root never mutates canonical forensic state; it dispatches tea.Cmds and
// renders the tea.Msgs they return (AGENTS §12).

// FocusRegion is which region currently owns the keyboard (design §A.2). The
// Root owns focus because it is the only place that knows the global geometry
// (Nav column vs Body workspace vs an overlay).
type FocusRegion int

const (
	// FocusNav: the SideNav selection cursor is live (default at startup).
	FocusNav FocusRegion = iota
	// FocusBody: the active screen's Update receives keys.
	FocusBody
	// FocusModal: an overlay (palette/search/modal) owns the keyboard.
	FocusModal
)

// overlayKind names which Root-owned overlay is open (design §A.6).
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayPalette
	overlaySearch
)

// overlayState is the Root-owned modal/input overlay. The palette and the
// global search box are promoted to Root overlays so they capture the keyboard
// (FocusModal) and render centered over the body.
type overlayState struct {
	palette components.CommandPalette
	search  components.SearchBox
	kind    overlayKind
}

// Mode is a top-level state-machine state. It tracks which screen class is
// active plus the overlay (modal) state.
type Mode string

const (
	ModeHome        Mode = "home"
	ModeSearch      Mode = "search"
	ModeWallet      Mode = "wallet"
	ModeTransaction Mode = "transaction"
	ModeEntity      Mode = "entity"
	ModeGraph       Mode = "graph"
	ModeNetwork     Mode = "network"
	ModeMap         Mode = "map"
	ModeDetection   Mode = "detection"
	ModeAlerts      Mode = "alerts"
	ModeMonitoring  Mode = "monitoring"
	ModeReports     Mode = "reports"
	ModeData        Mode = "data"
	ModeSettings    Mode = "settings"
	ModeHelp        Mode = "help"
	ModeModal       Mode = "modal"
)

// AcqState is the presentation view of acquisition availability. It is derived
// from the config mode + the offline gate; it is never faked (design §8).
type AcqState string

const (
	AcqEnabled   AcqState = "ENABLED"
	AcqPaused    AcqState = "PAUSED"
	AcqOffline   AcqState = "OFFLINE"
	AcqAirgapped AcqState = "AIRGAPPED"
)

// ModelStatus is the presentation view of the ML registry manifest state (§8).
type ModelStatus string

const (
	ModelsLoaded         ModelStatus = "LOADED"
	ModelsMissing        ModelStatus = "MISSING"
	ModelsSchemaMismatch ModelStatus = "SCHEMA-MISMATCH"
)

// GeoStatus is the presentation view of the local GeoIP registry (§8).
type GeoStatus string

const (
	GeoInstalled    GeoStatus = "INSTALLED"
	GeoNotInstalled GeoStatus = "NOT INSTALLED"
)

// GlobalState is the root-owned global state (design §4). Screens read it
// through the AppCtx but never write canonical fields; the root updates it in
// response to service results and nav events.
type GlobalState struct {
	Case        schema.Case
	Subject     Subject
	Network     state.Status
	Acquisition AcqState
	Models      ModelStatus
	GeoIP       GeoStatus
	Notes       []Notification
	Size        Breakpoint
}

// Root is the top-level model. It composes the AppShell, exactly one active
// Screen (built from the registry), and the modal/notification overlays.
type Root struct {
	ctx    context.Context
	app    *app.App
	styles theme.Styles

	mode   Mode
	stack  []ScreenID
	cur    ScreenID
	active Screen

	shell components.AppShell
	state GlobalState

	// Interaction state (design §A). focus is the region that owns the keyboard
	// (FocusNav at startup); navCursor is the SideNav selection index into
	// NavScreens(); priorFocus is restored when an overlay closes; navForced is
	// the Compact sidebar toggle (`b`); overlay holds the Root-owned
	// palette/search modals.
	focus      FocusRegion
	navCursor  int
	priorFocus FocusRegion
	navForced  bool
	overlay    overlayState

	// nowFn is the clock seam (design §C.4). topBar() reads it instead of
	// time.Now directly so golden tests can freeze the clock. Defaults to
	// time.Now in NewRoot.
	nowFn func() time.Time

	// summarizer is the OPTIONAL local-LLM narrator (design §E). It is injected
	// via WithSummarizer at the cli/commands composition root and flows into
	// every per-screen AppCtx so it reaches ScreenCtx via toScreenCtx. It is
	// transport-free here: tui imports only package llm, never llm/ollama. A nil
	// value defaults to llm.NewDeterministic() in NewRoot.
	summarizer llm.Summarizer
	// acquire is the OPTIONAL network-acquisition seam injected at the
	// cli/commands root (WithAcquireService). nil => offline-only TUI.
	acquire app.AcquireService

	width, height int
}

// RootOption configures the Root at construction (design §E.3/E.4). The
// variadic tail keeps every existing 2-arg NewRoot(ctx, a) call site valid.
type RootOption func(*Root)

// WithSummarizer injects the OPTIONAL local-LLM summarizer. The concrete value
// is built at the cli/commands composition root (the sole llm/ollama importer);
// a nil summarizer is normalized to llm.NewDeterministic() so the pane always
// works and tui never needs the transport.
func WithSummarizer(s llm.Summarizer) RootOption {
	return func(r *Root) {
		if s != nil {
			r.summarizer = s
		}
	}
}

// WithAcquireService injects the OPTIONAL network-acquisition seam. The concrete
// value (which constructs the explorer provider + engine) is built at the
// cli/commands composition root, so no provider/net import enters tui's closure.
// A nil service leaves the TUI offline-only (it will not offer acquisition).
func WithAcquireService(a app.AcquireService) RootOption {
	return func(r *Root) { r.acquire = a }
}

// NewRoot builds the root model over the shared app seam. The ctx is the
// program context (tea.WithContext); long operations derive children from it.
// The initial theme is Default; a WindowSizeMsg selects the breakpoint and a
// color-profile probe may switch to the degraded theme later.
func NewRoot(ctx context.Context, a *app.App, opts ...RootOption) *Root {
	th := theme.Default()
	styles := theme.Build(th)
	r := &Root{
		ctx:        ctx,
		app:        a,
		styles:     styles,
		mode:       ModeHome,
		cur:        ScreenHome,
		shell:      components.NewAppShell(styles),
		state:      GlobalState{Network: state.Disconnected, Acquisition: AcqPaused},
		focus:      FocusNav, // sidebar is immediately drivable (design §A.2)
		nowFn:      time.Now,
		summarizer: llm.NewDeterministic(), // default; replaced by WithSummarizer
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	r.navCursor = navIndex(ScreenHome)
	r.overlay = overlayState{
		palette: components.NewCommandPalette(styles, buildPaletteCommands()),
		search:  components.NewSearchBox(styles, "search — wallet / txid / IP"),
		kind:    overlayNone,
	}
	r.active = Build(ScreenHome, r.ctxFor())
	return r
}

// navIndex returns the index of a screen id within NavScreens() (the SideNav
// order), or 0 if it is not a nav screen, so the cursor starts on the current
// screen (design §A.2).
func navIndex(id ScreenID) int {
	for i, d := range NavScreens() {
		if d.ID == id {
			return i
		}
	}
	return 0
}

// buildPaletteCommands builds the palette's command list from the registry
// (single source of truth) plus a fixed verb slice (design §B.1). Screen jumps
// use the "nav:<id>" id form; verbs are bare tokens the Root parses. The LLM
// verb :explain and the :open subject verb are listed here but fully wired by
// later FEATs; the palette only emits the chosen id — the Root routes it.
func buildPaletteCommands() []components.Command {
	var cmds []components.Command
	for _, d := range Screens() {
		cmds = append(cmds, components.Command{ID: "nav:" + string(d.ID), Label: d.Title})
	}
	for _, v := range []components.Command{
		{ID: ":explain", Label: ":explain — plain-language summary"},
		{ID: ":search", Label: ":search — find wallet / tx / IP"},
		{ID: ":help", Label: ":help — keyboard reference"},
		{ID: ":quit", Label: ":quit — exit"},
		{ID: ":open", Label: ":open <id> — open a subject"},
	} {
		cmds = append(cmds, v)
	}
	return cmds
}

// ctxFor builds the AppCtx handed to screen factories for the current theme and
// breakpoint. A fresh AppCtx is produced per screen build so each screen sees
// the current layout class, the live program context, and the selected subject.
func (r *Root) ctxFor() *AppCtx {
	return &AppCtx{
		App:        r.app,
		Styles:     r.styles,
		Breakpoint: r.state.Size,
		Program:    r.ctx,
		Subject:    r.state.Subject,
		Summarizer: r.summarizer,
		Acquire:    r.acquire,
	}
}

// Init starts the clock tick, loads the live TopBar status, and kicks off the
// active screen's own data loads.
func (r *Root) Init() tea.Cmd {
	r.refreshStatus()
	cmds := []tea.Cmd{tickCmd()}
	if r.active != nil {
		cmds = append(cmds, r.active.Init())
	}
	return tea.Batch(cmds...)
}

// refreshStatus recomputes the global status fields from live services (design
// §8). Every field is read from a real source — none is hard-coded. It is cheap
// (local reads only) and called at startup, on each tick, and after a nav.
func (r *Root) refreshStatus() {
	if r.app == nil {
		return
	}
	// Active case (guard a nil Cases manager so a bare app in tests / early
	// boot degrades honestly instead of panicking).
	if r.app.Cases != nil {
		if c, ok := r.app.Cases.Active(); ok {
			r.state.Case = c
		} else {
			r.state.Case = schema.Case{}
		}
	}
	// Network status, air-gapped-first. BCTX is an OFFLINE forensic workstation:
	// it does not hold or advertise a live connection at rest, and it never
	// live-probes the internet just to paint a "CONNECTED" headline (misleading
	// for an air-gapped tool). The NET field reflects the operating POSTURE:
	//   airgap mode        -> AIRGAPPED (hard policy lock, no socket ever)
	//   everything else    -> DISCONNECTED (offline-first resting state; the
	//                         network is touched only transiently during an
	//                         explicit acquire, which the acquire flow surfaces).
	if r.app.Engine != nil {
		if r.app.Engine.Config().Network.Mode == configs.ModeAirgap {
			r.state.Network = state.Airgapped
		} else {
			r.state.Network = state.Disconnected
		}
	}
	// Acquisition + MODELS + GEOIP from the shared seam.
	var cfg *configs.Config
	if r.app.Engine != nil {
		cfg = r.app.Engine.Config()
	}
	r.state.Acquisition = acqStateFor(cfg)
	r.state.Models = modelStatusFor(screens.ModelsStatus(cfg))
	r.state.GeoIP = geoStatusFor(screens.GeoIPStatus())
}

// topBar builds the live TopBar from the current global state. Values are
// presentation strings; the component sheds the least-critical fields in
// compact widths.
func (r *Root) topBar() components.TopBar {
	tb := components.NewTopBar(r.styles)
	tb.Case = r.state.Case.ID
	tb.Network = string(r.state.Network)
	tb.Acquisition = string(r.state.Acquisition)
	tb.Models = string(r.state.Models)
	tb.GeoIP = string(r.state.GeoIP)
	tb.DB = "NO CASE"
	if r.app != nil && r.app.Repo != nil {
		tb.DB = "LOCAL"
	}
	if r.app != nil && r.app.Engine != nil {
		// Provider is a configured name, but it is user-controlled config, so
		// pass it through the redaction guard (design §12 invariant 5; AGENTS
		// §21) in case an endpoint-with-token was placed here.
		tb.Provider = Redact(r.app.Engine.Config().Acquisition.Provider)
	}
	tb.Subject = r.state.Subject.ID
	tb.Clock = r.now().Format("15:04:05")
	return tb
}

// now returns the current time through the clock seam, defaulting to time.Now
// when nowFn is unset (a zero-value Root in a test).
func (r *Root) now() time.Time {
	if r.nowFn != nil {
		return r.nowFn()
	}
	return time.Now()
}

// navItems builds the SideNav entries from the registry (single source of
// truth), marking the active screen.
func (r *Root) navItems() []components.NavItem {
	var items []components.NavItem
	for _, d := range NavScreens() {
		items = append(items, components.NavItem{
			Key: d.Key, Title: d.Title, Active: d.ID == r.cur,
		})
	}
	return items
}

// acqStateFor derives the presentation acquisition state from the config
// (design §8). It never fakes "ENABLED": offline/airgap modes report honestly.
func acqStateFor(cfg *configs.Config) AcqState {
	if cfg == nil {
		return AcqPaused
	}
	switch cfg.Network.Mode {
	case configs.ModeAirgap:
		return AcqAirgapped
	case configs.ModeOffline:
		return AcqOffline
	}
	if cfg.AcquisitionAllowed() {
		return AcqEnabled
	}
	return AcqPaused
}

// modelStatusFor maps the screens.ModelsStatus string to the typed ModelStatus.
func modelStatusFor(s string) ModelStatus {
	switch s {
	case "LOADED":
		return ModelsLoaded
	case "SCHEMA-MISMATCH":
		return ModelsSchemaMismatch
	default:
		return ModelsMissing
	}
}

// geoStatusFor maps the geoip status string to the typed GeoStatus for display.
func geoStatusFor(s string) GeoStatus {
	if s == "NOT INSTALLED" {
		return GeoNotInstalled
	}
	return GeoStatus(s)
}

// navTo pushes the current screen onto the stack and swaps in the target built
// from the registry (design §4). Unregistered targets are ignored.
//
// Subject resolution: an explicit non-empty subject always wins and becomes the
// global selection. When no subject is passed (SideNav Enter / a number jump)
// and the target is a subject-scoped screen with no global subject yet, the
// Root resolves a sensible DEFAULT subject from the case (the most recent
// transaction, or its primary address for wallet-shaped screens) so those
// screens open populated instead of on a "no subject selected" placeholder. The
// default is a cheap, bounded, offline repo read — never fabricated.
func (r *Root) navTo(id ScreenID, subject Subject) tea.Cmd {
	def := Lookup(id)
	if def == nil {
		return nil
	}
	if r.cur != "" {
		r.stack = append(r.stack, r.cur)
	}
	switch {
	case !subject.Empty():
		r.state.Subject = subject
	case r.state.Subject.Empty():
		// No explicit and no prior subject: try a case default for the target.
		if def := r.defaultSubjectFor(id); !def.Empty() {
			r.state.Subject = def
		}
	case !subjectKindFits(id, r.state.Subject.Kind):
		// A subject IS selected, but its kind does not fit the target screen
		// (e.g. carrying a wallet address into the Transaction screen, which
		// GetTransaction can't resolve -> "not found"). Resolve a fresh default
		// of the right kind so the screen opens populated instead of erroring.
		if def := r.defaultSubjectFor(id); !def.Empty() {
			r.state.Subject = def
		}
	}
	r.cur = id
	r.active = def.New(r.ctxFor())
	r.mode = modeForScreen(id)
	r.refreshStatus()
	var initCmd tea.Cmd
	if r.active != nil {
		initCmd = r.active.Init()
	}
	// The Analysis window is a type-first screen: it focuses its subject box in
	// Init, so opening it must move keyboard focus to the Body or the user's
	// keystrokes go to nav navigation and the box stays empty — the "can't type
	// / stuck on querying" bug. This is scoped to Analysis specifically so other
	// screens keep their SideNav-first focus behavior.
	if id == ScreenSearch && r.inputFocused() {
		r.focus = FocusBody
	}
	return initCmd
}

// subjectScopedScreen reports whether a screen renders a specific subject (and
// therefore benefits from a default-subject fallback when none is selected).
func subjectScopedScreen(id ScreenID) bool {
	switch id {
	case ScreenWallet, ScreenTransaction, ScreenEntity, ScreenGraph, ScreenDetection:
		return true
	default:
		return false
	}
}

// subjectKindFits reports whether a selected subject of kind k is usable by the
// target screen. The Transaction screen needs a tx id (a wallet id would make
// GetTransaction report "not found"); the wallet-shaped screens (Wallet /
// Entity / Detection) need a wallet id; the Graph screen centers on any node so
// it accepts either. Non-subject-scoped screens never carry a mismatch.
func subjectKindFits(id ScreenID, k SubjectKind) bool {
	switch id {
	case ScreenTransaction:
		return k == SubjectTx
	case ScreenWallet, ScreenEntity, ScreenDetection:
		return k == SubjectWallet || k == SubjectEntity
	case ScreenGraph:
		return k == SubjectTx || k == SubjectWallet || k == SubjectEntity
	default:
		return true
	}
}

// defaultSubjectFor resolves a case-default subject for a subject-scoped screen
// when nothing is selected yet. It reads the most recent transaction (a cheap,
// bounded, offline index read) and derives:
//   - Transaction / Graph: the tx itself (the graph centers on it fine).
//   - Wallet / Entity / Detection: the tx's primary input address (or, lacking
//     inputs, its first output) so these wallet-shaped screens analyze a real
//     address.
//
// It returns an empty Subject (and the screen keeps its honest placeholder) when
// there is no repo, no case, or no transactions yet — never a fabricated id.
func (r *Root) defaultSubjectFor(id ScreenID) Subject {
	if !subjectScopedScreen(id) {
		return Subject{}
	}
	if r.app == nil || r.app.Repo == nil {
		return Subject{}
	}
	txs, err := r.app.Repo.RecentTransactions(r.ctx, 1)
	if err != nil || len(txs) == 0 {
		return Subject{}
	}
	tx := txs[0]
	switch id {
	case ScreenTransaction, ScreenGraph:
		return Subject{ID: tx.TxID, Kind: SubjectTx}
	default: // Wallet / Entity / Detection — pick a representative address.
		if addr := primaryAddress(tx); addr != "" {
			return Subject{ID: addr, Kind: SubjectWallet}
		}
		return Subject{ID: tx.TxID, Kind: SubjectTx}
	}
}

// primaryAddress returns a representative address for a transaction: its first
// input address, or (lacking inputs) its first output address, or "" when the
// transaction carries neither.
func primaryAddress(tx schema.Transaction) string {
	for _, in := range tx.Inputs {
		if in.Address != "" {
			return in.Address
		}
	}
	for _, o := range tx.Outputs {
		if o.Address != "" {
			return o.Address
		}
	}
	return ""
}

// navBack pops the navigation stack (Esc-back). It is a no-op at the root.
func (r *Root) navBack() tea.Cmd {
	if len(r.stack) == 0 {
		return nil
	}
	prev := r.stack[len(r.stack)-1]
	r.stack = r.stack[:len(r.stack)-1]
	r.cur = prev
	if def := Lookup(prev); def != nil {
		r.active = def.New(r.ctxFor())
		r.mode = modeForScreen(prev)
		if r.active != nil {
			return r.active.Init()
		}
	}
	return nil
}

// modeForScreen maps a ScreenID to the top-level Mode.
func modeForScreen(id ScreenID) Mode {
	switch id {
	case ScreenHome:
		return ModeHome
	case ScreenSearch:
		return ModeSearch
	case ScreenWallet:
		return ModeWallet
	case ScreenTransaction:
		return ModeTransaction
	case ScreenEntity:
		return ModeEntity
	case ScreenGraph:
		return ModeGraph
	case ScreenNetwork:
		return ModeNetwork
	case ScreenGeoMap:
		return ModeMap
	case ScreenDetection:
		return ModeDetection
	case ScreenAlerts:
		return ModeAlerts
	case ScreenMonitoring:
		return ModeMonitoring
	case ScreenReports:
		return ModeReports
	case ScreenData:
		return ModeData
	case ScreenSettings:
		return ModeSettings
	case ScreenHelp:
		return ModeHelp
	default:
		return ModeHome
	}
}

// Update is the root reducer. Global keys are dispatched first (focus-aware),
// then window-size and data messages, then the active screen gets a turn.
func (r *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = m.Width, m.Height
		r.state.Size = Classify(m)
		return r, nil

	case TickMsg:
		r.refreshStatus()
		return r, tickCmd()

	case screens.NavSearch:
		return r, r.navTo(navForSearch(m.Kind), Subject{ID: m.ID, Kind: subjectKind(m.Kind)})

	case screens.NavScreen:
		// A screen asked to jump to another screen (e.g. a detail screen's `g`
		// opening the Graph), carrying its current subject so the destination is
		// scoped to it. Only known registry ids navigate; an unknown id is an
		// honest no-op.
		if def := Lookup(ScreenID(m.Screen)); def != nil {
			return r, r.navTo(def.ID, Subject{ID: m.Subject.ID, Kind: SubjectKind(m.Subject.Kind)})
		}
		return r, nil

	case components.RowSelected:
		// A table row was activated (Enter). For the navigable subject kinds
		// (wallet / tx / ip / block) the Root drills into the matching detail
		// screen — so selecting a row on ANY board (Wallet txs, Network IPs,
		// etc.) opens its subject end-to-end without each screen re-wiring the
		// navigation. Other kinds (report, extension, dataset) are screen-local
		// (preview / enable) and are forwarded to the active screen unchanged.
		switch m.Kind {
		case "wallet", "tx", "ip", "block", "height":
			if m.ID != "" {
				return r, r.navTo(navForSearch(m.Kind), Subject{ID: m.ID, Kind: subjectKind(m.Kind)})
			}
		}
		if r.active != nil {
			next, cmd := r.active.Update(msg)
			r.active = next
			return r, cmd
		}
		return r, nil

	case Notification:
		r.state.Notes = append(r.state.Notes, m)
		return r, nil

	case NavMsg:
		return r, r.navTo(m.To, m.Subject)

	case NavBackMsg:
		return r, r.navBack()

	case components.CommandChosen:
		// Only consumed by the Root while the palette overlay is open; a stray
		// one otherwise must still reach the active screen (do NOT swallow it in
		// the switch — fall through to the active-screen forward below).
		if r.overlay.kind == overlayPalette {
			return r, r.onCommandChosen(m)
		}
		if r.active != nil {
			next, cmd := r.active.Update(msg)
			r.active = next
			return r, cmd
		}
		return r, nil

	case components.SearchSubmitted:
		// A SearchSubmitted while the global search overlay is open is Root-owned
		// (navigates to the Analysis screen seeded with the query). Any OTHER
		// SearchSubmitted comes from an in-screen box (the Analysis window, the
		// dashboard subject input, the monitor target) and MUST be forwarded to
		// the active screen — otherwise it is swallowed here and the screen's
		// lookup never runs (the 'querying… forever' bug). A matched case with a
		// false inner guard does NOT fall through in Go, so forward explicitly.
		if r.overlay.kind == overlaySearch {
			return r, r.onSearchSubmitted(m)
		}
		if r.active != nil {
			next, cmd := r.active.Update(msg)
			r.active = next
			return r, cmd
		}
		return r, nil

	case tea.KeyMsg:
		return r.handleKey(m)
	}

	// While an overlay owns the keyboard, non-key ticks (textinput blink) must
	// also reach the overlay so the cursor keeps blinking (design §A.6).
	if r.focus == FocusModal && r.overlay.kind != overlayNone {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			return r, r.updateOverlay(msg)
		}
	}

	// Everything else (DataLoaded/DataError/component msgs) flows to the screen.
	if r.active != nil {
		next, cmd := r.active.Update(msg)
		r.active = next
		return r, cmd
	}
	return r, nil
}

// handleKey routes a key through the focus-aware global table (design §A.5).
// EVERY token GlobalKeyMap.Dispatch can emit — up/down/enter/esc/tab/shift+tab/
// search/palette/help/quit/jump:/"" — has exactly ONE defined destination per
// focus region, so no token falls through un-handled and no key can wedge or
// crash the model. The precedence is: overlay (FocusModal) first, then the
// focus region. The only no-ops are deliberate and documented: an unmatched key
// in Nav/Modal, and a list-boundary cursor move.
func (r *Root) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Bracketed paste: a terminal paste (right-click / Ctrl+Shift+V) arrives as
	// one KeyMsg with Paste=true carrying the whole pasted string. Route it
	// straight to whatever owns input — the open overlay, else the active
	// screen — BEFORE nav dispatch, so a pasted id (which may start with a digit
	// or letter that is a nav/jump key) is inserted as text instead of
	// triggering a screen jump. When nothing is focused the paste is a no-op.
	if msg.Paste {
		if r.focus == FocusModal && r.overlay.kind != overlayNone {
			return r, r.updateOverlay(msg)
		}
		if r.active != nil {
			next, cmd := r.active.Update(msg)
			r.active = next
			return r, cmd
		}
		return r, nil
	}

	km := NewGlobalKeyMap()
	// inputFocused suppresses single-letter nav in Dispatch while an overlay or
	// an in-screen input owns the keyboard, so typing works.
	action := km.Dispatch(msg, r.inputSuppressed())

	// Ctrl+C always quits, in every focus region, before any other routing.
	if action == "quit" {
		return r, tea.Quit
	}

	// Modal overlay owns the keyboard first (design §A.5 "overlay first").
	if r.focus == FocusModal {
		return r.handleKeyModal(msg, action)
	}

	// jump:<key> is handled before the focus fan-out so number/letter jumps work
	// from either Nav or Body (design §A.5).
	if len(action) > 5 && action[:5] == "jump:" {
		if def := LookupKey(action[5:]); def != nil {
			cmd := r.navTo(def.ID, Subject{})
			r.navCursor = navIndex(def.ID)
			// Keep FocusNav UNLESS navTo moved focus to the Body because the
			// target screen focused an in-screen input (e.g. Analysis). This is
			// what lets a jump to the Analysis window land the cursor in its
			// subject box so the user can type immediately.
			if r.focus != FocusBody {
				r.focus = FocusNav
			}
			return r, cmd
		}
		return r, nil // unregistered jump key: documented no-op
	}

	switch action {
	case "help":
		cmd := r.navTo(ScreenHelp, Subject{})
		return r, cmd
	case "search":
		return r, r.openSearch()
	case "palette":
		return r, r.openPalette()
	case "tab", "shift+tab":
		// Tab/Shift+Tab are ALWAYS the global Nav<->Body focus cycle, consumed
		// before any screen sees them (screens never consume Tab, design §A.5).
		r.toggleNavBody()
		return r, nil
	}

	// Breakpoint sidebar toggle: 'b' is a free key (not a registry jump key).
	if msg.String() == "b" {
		r.navForced = !r.navForced
		return r, nil
	}

	if r.focus == FocusNav {
		return r.handleKeyNav(action)
	}
	return r.handleKeyBody(msg, action)
}

// handleKeyNav routes a key while the SideNav owns the keyboard (design §A.5).
func (r *Root) handleKeyNav(action string) (tea.Model, tea.Cmd) {
	switch action {
	case "up":
		r.navMoveCursor(-1)
	case "down":
		r.navMoveCursor(1)
	case "enter":
		if def := r.navScreenUnderCursor(); def != nil {
			return r, r.navTo(def.ID, Subject{})
		}
	case "esc":
		// Nav-focused Esc pops the nav stack (design §A.5).
		return r, r.navBack()
	}
	// up/down at a list boundary, or any unmatched key, is a documented no-op.
	return r, nil
}

// handleKeyBody forwards a key to the active screen while the Body owns the
// keyboard, except esc which returns focus to Nav (design §A.5).
func (r *Root) handleKeyBody(msg tea.KeyMsg, action string) (tea.Model, tea.Cmd) {
	if action == "esc" {
		r.focus = FocusNav
		return r, nil
	}
	// up/down/enter and any unmatched key ("") are forwarded raw to the screen.
	if r.active != nil {
		next, cmd := r.active.Update(msg)
		r.active = next
		return r, cmd
	}
	return r, nil
}

// handleKeyModal routes a key while an overlay owns the keyboard (design §A.5).
func (r *Root) handleKeyModal(msg tea.KeyMsg, action string) (tea.Model, tea.Cmd) {
	if action == "esc" {
		r.closeOverlay()
		return r, nil
	}
	// Every other key (including enter/up/down and raw edits) is forwarded to the
	// overlay; search/palette/help/jump are ignored while an overlay is open.
	return r, r.updateOverlay(msg)
}

// toggleNavBody flips focus between Nav and Body (the two-region global cycle).
// It never gets stuck: from Nav -> Body, from Body -> Nav (design §A.5).
func (r *Root) toggleNavBody() {
	if r.focus == FocusBody {
		r.focus = FocusNav
	} else {
		r.focus = FocusBody
	}
}

// navMoveCursor moves the SideNav cursor by delta, clamped to the nav list.
func (r *Root) navMoveCursor(delta int) {
	n := len(NavScreens())
	if n == 0 {
		return
	}
	r.navCursor += delta
	if r.navCursor < 0 {
		r.navCursor = 0
	}
	if r.navCursor >= n {
		r.navCursor = n - 1
	}
}

// navScreenUnderCursor returns the registry entry the nav cursor points at.
func (r *Root) navScreenUnderCursor() *ScreenDef {
	nav := NavScreens()
	if r.navCursor < 0 || r.navCursor >= len(nav) {
		return nil
	}
	return Lookup(nav[r.navCursor].ID)
}

// openPalette opens the command palette overlay and takes FocusModal.
func (r *Root) openPalette() tea.Cmd {
	r.priorFocus = r.focus
	r.focus = FocusModal
	r.overlay.kind = overlayPalette
	return r.overlay.palette.Open()
}

// openSearch opens the global search overlay and takes FocusModal.
func (r *Root) openSearch() tea.Cmd {
	r.priorFocus = r.focus
	r.focus = FocusModal
	r.overlay.kind = overlaySearch
	r.overlay.search.SetValue("")
	return r.overlay.search.Focus()
}

// closeOverlay closes any open overlay and restores the prior focus region.
func (r *Root) closeOverlay() {
	r.overlay.palette.Close()
	r.overlay.search.Blur()
	r.overlay.kind = overlayNone
	r.focus = r.priorFocus
}

// updateOverlay forwards a message to the open overlay sub-model.
func (r *Root) updateOverlay(msg tea.Msg) tea.Cmd {
	switch r.overlay.kind {
	case overlayPalette:
		p, cmd := r.overlay.palette.Update(msg)
		r.overlay.palette = p
		return cmd
	case overlaySearch:
		s, cmd := r.overlay.search.Update(msg)
		r.overlay.search = s
		return cmd
	}
	return nil
}

// onCommandChosen routes a palette selection (design §B.1): nav:<id> navigates;
// a bare verb runs the Root action; open:<subject> classifies then navigates;
// unknown ids are ignored honestly. The overlay is closed and prior focus
// restored first (NIT-1: consume in the Root, never leak to a screen).
func (r *Root) onCommandChosen(m components.CommandChosen) tea.Cmd {
	r.closeOverlay()
	id := m.ID
	switch {
	case strings.HasPrefix(id, "nav:"):
		return r.navTo(ScreenID(id[len("nav:"):]), Subject{})
	case strings.HasPrefix(id, "open:"):
		// open:<subject> — classify the trimmed id and navigate, or no-op when
		// it matches no rule (honest, not a crash).
		return r.openSubject(strings.TrimSpace(id[len("open:"):]))
	case id == ":search":
		return r.openSearch()
	case id == ":help":
		return r.navTo(ScreenHelp, Subject{})
	case id == ":quit":
		return tea.Quit
	case id == ":open":
		// :open with no inline subject opens the global search overlay so the
		// user can type one; honest, not a crash.
		return r.openSearch()
	case id == ":explain":
		// Route the explain verb to the active screen exactly as the 'L' key
		// does (design §E.4). The screen owns the pane + the honest no-op when no
		// subject/result is loaded; the Root never computes or renders a summary.
		return r.explainActive()
	}
	return nil
}

// explainActive forwards the :explain verb to the active screen as the same 'L'
// key the screen handles directly (design §E.4). The screen decides whether to
// dispatch explainCmd (a result is loaded) or show the honest "open a subject
// first" no-op (NIT-4). The Root never builds or renders a summary itself.
func (r *Root) explainActive() tea.Cmd {
	if r.active == nil {
		return nil
	}
	next, cmd := r.active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	r.active = next
	return cmd
}

// openSubject classifies a free-typed id and navigates to the matching detail
// screen (design §B.3). An id that matches no classifier rule is a documented
// no-op so a garbage palette/verb arg never navigates to a dead screen.
func (r *Root) openSubject(id string) tea.Cmd {
	kind, ok := screens.ClassifySubject(id)
	if !ok {
		return nil
	}
	return r.navTo(navForSearch(kind), Subject{ID: id, Kind: subjectKind(kind)})
}

// onSearchSubmitted closes the global search overlay, restores focus, then
// navigates to the Search screen seeding the query so the in-screen box opens
// pre-filled and already looking up (design §A.6/B.2). The overlay consumes the
// SearchSubmitted here (NIT-1) so it never reaches Search.Update directly.
func (r *Root) onSearchSubmitted(m components.SearchSubmitted) tea.Cmd {
	q := m.Query
	r.closeOverlay()
	cmd := r.navTo(ScreenSearch, Subject{})
	seedCmd := r.seedSearch(q)
	return tea.Batch(cmd, seedCmd)
}

// seedSearch pre-fills the just-navigated Search screen with a query and kicks
// off its lookup, if the active screen exposes a Seed hook. It is a no-op when
// the active screen is not the Search screen (defensive; navTo put it there).
func (r *Root) seedSearch(query string) tea.Cmd {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	if a, ok := r.active.(screenAdapter); ok {
		if s, ok := a.m.(interface{ Seed(string) tea.Cmd }); ok {
			return s.Seed(query)
		}
	}
	return nil
}

// inputSuppressed reports whether single-letter nav should be suppressed in
// Dispatch: true when an overlay owns the keyboard, or when the active screen
// reports an in-screen input is focused (design §A.5/A.7). This generalizes the
// old inputFocused hook rather than removing it.
func (r *Root) inputSuppressed() bool {
	if r.focus == FocusModal {
		return true
	}
	// An in-screen input only captures keys while the BODY owns the keyboard.
	// When focus is on the Nav (even if the screen's box is still internally
	// focused — e.g. after Esc returned focus to Nav without blurring), nav keys
	// must drive the SideNav, not be suppressed. This keeps `:`/`/`/number jumps
	// working after leaving a type-first screen.
	if r.focus != FocusBody {
		return false
	}
	return r.inputFocused()
}

// inputFocused reports whether the active screen's in-screen input owns the
// keyboard. The Search screen exposes Focused() via the adapter; any other
// screen returns false.
func (r *Root) inputFocused() bool {
	if a, ok := r.active.(screenAdapter); ok {
		if fs, ok := a.m.(interface{ Focused() bool }); ok {
			return fs.Focused()
		}
	}
	return false
}

// navForSearch maps a search-result kind to its detail screen id.
func navForSearch(kind string) ScreenID {
	switch kind {
	case "tx":
		return ScreenTransaction
	case "ip":
		return ScreenNetwork
	case "block", "height":
		return ScreenBlock
	default:
		return ScreenWallet
	}
}

// subjectKind maps a search-result kind string to a SubjectKind.
func subjectKind(kind string) SubjectKind {
	switch kind {
	case "tx":
		return SubjectTx
	case "ip":
		return SubjectIP
	case "entity":
		return SubjectEntity
	case "block", "height":
		return SubjectBlock
	default:
		return SubjectWallet
	}
}

// View renders the shell with the active screen inside its workspace frame. The
// AppShell owns the too-small floor and region geometry; the root supplies the
// live TopBar, the registry-driven SideNav, and the active screen's body.
func (r *Root) View() string {
	body := ""
	lay := ComputeLayoutNav(r.width, r.height, r.navForced)
	if !lay.TooSmall && r.active != nil {
		body = r.active.View(components.Frame(lay.Workspace))
	}
	title := ""
	if def := Lookup(r.cur); def != nil {
		title = def.Title
	}
	// Populate chrome from live state before rendering.
	r.shell.SetTopBar(r.topBar())
	r.shell.SetNavItems(r.navItems())

	// Render any open overlay centered over the body (design §A.6).
	overlay := ""
	if !lay.TooSmall {
		overlay = r.overlayView(components.Frame(lay.Workspace))
	}

	return r.shell.View(components.ShellFrame{
		Width:      r.width,
		Height:     r.height,
		Title:      title,
		Body:       body,
		NavWidth:   lay.SideNav.W,
		NavCursor:  r.navCursor,
		NavFocused: r.focus == FocusNav,
		Focused:    r.shellRegion(),
		Overlay:    overlay,
		Hints:      r.hints(),
	})
}

// shellRegion maps the Root focus to the shell's presentation region enum.
func (r *Root) shellRegion() components.ShellRegion {
	switch r.focus {
	case FocusNav:
		return components.ShellFocusNav
	case FocusBody:
		return components.ShellFocusBody
	case FocusModal:
		return components.ShellFocusModal
	default:
		return components.ShellFocusNone
	}
}

// overlayView renders the open overlay (palette/search) into the workspace
// frame, or "" when none is open.
func (r *Root) overlayView(f components.Frame) string {
	switch r.overlay.kind {
	case overlayPalette:
		return r.overlay.palette.View(f)
	case overlaySearch:
		return r.overlay.search.View(components.Frame{W: f.W, H: 1})
	}
	return ""
}

// hints is the ContextBar hint text for the active screen's own key help.
func (r *Root) hints() string {
	return ""
}
