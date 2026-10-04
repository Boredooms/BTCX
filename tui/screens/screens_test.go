package screens

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// lipglossWidthTest measures a line's display width accounting for wide runes
// and ignoring ANSI styling, matching the component overflow guard.
func lipglossWidthTest(s string) int { return lipgloss.Width(s) }

// acceptanceSizes are the five terminal rectangles every screen must render
// into without overflowing (design §6, acceptance §19.4).
var acceptanceSizes = []components.Frame{
	{W: 80, H: 24}, {W: 100, H: 30}, {W: 120, H: 40}, {W: 160, H: 50}, {W: 200, H: 60},
}

// seedFixtureRepo opens a temp sqlite repo and seeds one wallet transaction and
// one network observation so screens have real local data to render. Fixtures
// are the ONLY fabricated data in the codebase and live here, in a test
// (AGENTS §15/§16).
func seedFixtureRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	ctx := context.Background()
	tx := schema.Transaction{
		TxID:      "TXFIXTURE1",
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		FeeBTC:    0.0001,
		Inputs:    []schema.TransactionInput{{Address: "WFIXTURE", AmountBTC: 1.0, Index: 0}},
		Outputs:   []schema.TransactionOutput{{Address: "WOTHER", AmountBTC: 0.99, Index: 0}},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	obs := schema.NetworkObservation{
		ID: "OBS1", TxID: "TXFIXTURE1", SrcIP: "203.0.113.9", DstIP: "198.51.100.2",
		Country: "DE", ASN: "AS3320", Timestamp: time.Now().UTC(),
	}
	if err := repo.SaveNetworkObservations(ctx, []schema.NetworkObservation{obs}); err != nil {
		t.Fatalf("seed obs: %v", err)
	}
	return repo
}

// fixtureCtx builds a ScreenCtx wired to a seeded repo with a wallet subject.
func fixtureCtx(t *testing.T, subject Subject) *ScreenCtx {
	t.Helper()
	repo := seedFixtureRepo(t)
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline // deterministic offline gate
	return &ScreenCtx{
		Ctx:     context.Background(),
		App:     &app.App{Repo: repo, CaseID: "fixture-case", Cleanup: func() {}},
		Cfg:     cfg,
		Styles:  theme.Build(theme.Default()),
		Subject: subject,
	}
}

// drive runs a model's Init then feeds each message to Update, returning the
// final model. It mirrors how the Root drives a screen.
func drive(m Model, msgs ...tea.Msg) Model {
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = m.Update(msg)
		}
	}
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

// allScreenFactories returns one constructor per screen so a single table-driven
// test can exercise every screen at every acceptance size.
func allScreenFactories() map[string]func(*ScreenCtx) Model {
	return map[string]func(*ScreenCtx) Model{
		"home":        func(c *ScreenCtx) Model { return NewHome(c) },
		"search":      func(c *ScreenCtx) Model { return NewSearch(c) },
		"wallet":      func(c *ScreenCtx) Model { return NewWallet(c) },
		"transaction": func(c *ScreenCtx) Model { return NewTransaction(c) },
		"entity":      func(c *ScreenCtx) Model { return NewEntity(c) },
		"graph":       func(c *ScreenCtx) Model { return NewGraph(c) },
		"network":     func(c *ScreenCtx) Model { return NewNetwork(c) },
		"geomap":      func(c *ScreenCtx) Model { return NewGeoMap(c) },
		"detection":   func(c *ScreenCtx) Model { return NewDetection(c) },
		"alerts":      func(c *ScreenCtx) Model { return NewAlerts(c) },
		"monitoring":  func(c *ScreenCtx) Model { return NewMonitoring(c) },
		"data":        func(c *ScreenCtx) Model { return NewData(c) },
		"reports":     func(c *ScreenCtx) Model { return NewReports(c) },
		"settings":    func(c *ScreenCtx) Model { return NewSettings(c) },
		"extensions":  func(c *ScreenCtx) Model { return NewExtensions(c) },
		"help":        func(c *ScreenCtx) Model { return NewHelp(c) },
		"timeline":    func(c *ScreenCtx) Model { return NewTimeline(c) },
	}
}

// TestScreensRenderWithinFrameAtAllSizes asserts every screen renders without
// overflowing its Frame at the five acceptance sizes, with a live fixture case
// (no network socket, no raw SQL beyond fixture seeding).
func TestScreensRenderWithinFrameAtAllSizes(t *testing.T) {
	for name, factory := range allScreenFactories() {
		for _, sz := range acceptanceSizes {
			ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
			m := factory(ctx)
			// Init may issue a command; run it once synchronously so loaded
			// state is rendered (not just the loading placeholder).
			if cmd := m.Init(); cmd != nil {
				if msg := cmd(); msg != nil {
					m, _ = m.Update(msg)
				}
			}
			out := m.View(sz)
			assertNoOverflow(t, name, out, sz)
		}
	}
}

// TestScreensEmptyFrameRendersNothing asserts the honest empty-frame contract:
// below drawable area a screen renders "" (never a panic or stray glyph).
func TestScreensEmptyFrameRendersNothing(t *testing.T) {
	for name, factory := range allScreenFactories() {
		ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
		m := factory(ctx)
		if out := m.View(components.Frame{W: 0, H: 0}); out != "" {
			t.Errorf("%s: empty frame should render nothing, got %q", name, out)
		}
	}
}

// assertNoOverflow checks no rendered line exceeds the frame width and the block
// does not exceed the frame height (lipgloss width accounting).
func assertNoOverflow(t *testing.T, name, out string, f components.Frame) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > f.H {
		t.Errorf("%s @ %dx%d: %d lines exceed height %d", name, f.W, f.H, len(lines), f.H)
	}
	for i, ln := range lines {
		if w := lipglossWidthTest(ln); w > f.W {
			t.Errorf("%s @ %dx%d: line %d width %d exceeds %d: %q", name, f.W, f.H, i, w, f.W, ln)
		}
	}
}
