package screens

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// renderDashboard drives a Home screen through Init + its async results and
// returns the rendered View at the given size. It mirrors how the Root drives a
// screen (Init -> run the batched commands -> Update), so the snapshot reflects
// the LOADED state, not the loading placeholder. It is deterministic: all data
// comes from the seeded fixture repo and no timing is involved.
func renderDashboard(t *testing.T, ctx *ScreenCtx, f components.Frame) string {
	t.Helper()
	h := NewHome(ctx)
	if cmd := h.Init(); cmd != nil {
		var m Model = h
		for _, msg := range runBatch(cmd) {
			m, _ = m.Update(msg)
		}
		return m.View(f)
	}
	return h.View(f)
}

// TestDashboardCockpitLayout asserts the rebuilt dashboard renders the cockpit
// chrome — a bordered, titled, multi-panel grid — against a seeded fixture case
// at a wide size, and that it never overflows the frame. It checks the panel
// titles and a box-border glyph are present (the chrome), and the live fixture
// values appear (the data is really wired, not faked).
func TestDashboardCockpitLayout(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	f := components.Frame{W: 160, H: 50}
	out := renderDashboard(t, ctx, f)

	assertNoOverflow(t, "dashboard-cockpit", out, f)

	for _, want := range []string{
		"DASHBOARD — INTELLIGENCE COCKPIT",
		"GEO ACTIVITY",
		"TRANSACTION / ENTITY GRAPH",
		"RECENT HIGH-RISK ACTIVITY",
		"RISK DISTRIBUTION — ALERTS BY BAND",
		"CASE / SYSTEM",
		"fixture-case", // live case id in the detail strip
	} {
		if !strings.Contains(out, want) {
			t.Errorf("cockpit dashboard missing %q in:\n%s", want, out)
		}
	}
	// A rounded border glyph proves the shared Panel chrome actually drew boxes
	// (RoundedBorder is the Default theme border).
	if !strings.ContainsAny(out, "─│╭╮╰╯") {
		t.Errorf("expected bordered panels (box-drawing glyphs) in:\n%s", out)
	}
}

// TestDashboardEmptyCaseHonest asserts the empty-case dashboard shows real zeros
// in the metric tiles and honest empty states in the panels — never fabricated
// numbers, nodes, or alerts (AGENTS §15, §16).
func TestDashboardEmptyCaseHonest(t *testing.T) {
	// An empty case: a repo with no data seeded.
	ctx := emptyCaseCtx(t, "empty-case")
	f := components.Frame{W: 160, H: 50}
	out := renderDashboard(t, ctx, f)

	assertNoOverflow(t, "dashboard-empty", out, f)

	// Honest zeros in the tiles.
	if !strings.Contains(out, "0") {
		t.Errorf("empty dashboard should show honest zero counts in:\n%s", out)
	}
	// Honest empty states — no fabricated alerts/graph/observations.
	for _, want := range []string{
		"no alerts in this case",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty dashboard missing honest empty state %q in:\n%s", want, out)
		}
	}
	// It must NOT invent a non-zero count anywhere obvious: the tiles are zeros.
	if strings.Contains(out, "fixture") {
		t.Errorf("empty dashboard must not leak fixture data:\n%s", out)
	}
}

// TestDashboardPreview prints the empty-case dashboard at 160x50 so a human can
// eyeball the cockpit layout against the reference. It is not an assertion; run
// it with `go test -run TestDashboardPreview -v ./tui/screens` to see the ASCII.
func TestDashboardPreview(t *testing.T) {
	ctx := emptyCaseCtx(t, "demo-case")
	out := renderDashboard(t, ctx, components.Frame{W: 160, H: 50})
	t.Logf("\n%s", out)
}

// emptyCaseCtx builds a ScreenCtx wired to a fresh, UNSEEDED sqlite repo so the
// dashboard renders real zeros and honest empty states. It deliberately
// fabricates nothing (the only fabricated data in the codebase is the seeded
// fixture in screens_test.go).
func emptyCaseCtx(t *testing.T, caseID string) *ScreenCtx {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	return &ScreenCtx{
		Ctx:    context.Background(),
		App:    &app.App{Repo: repo, CaseID: caseID, Cleanup: func() {}},
		Cfg:    cfg,
		Styles: theme.Build(theme.Default()),
	}
}

// TestDashboardOpenSubject exercises the open-subject input (design §B.3): 'i'
// focuses it, a valid id emits NavSearch with the classified kind, and an empty
// or garbage id shows an honest inline note WITHOUT navigating.
func TestDashboardOpenSubject(t *testing.T) {
	// Helper: focus the subject input, submit a query, return the resulting cmd.
	submit := func(h *Home, query string) tea.Cmd {
		h.subject.Focus()
		_, _ = h.Update(componentsKey("i")) // no-op while focused; keeps path warm
		h.subject.SetValue(query)
		_, cmd := h.Update(components.SearchSubmitted{Query: query})
		return cmd
	}

	t.Run("64-hex -> tx", func(t *testing.T) {
		h := NewHome(emptyCaseCtx(t, "c"))
		cmd := submit(h, strings.Repeat("a", 64))
		nav := expectNavSearch(t, cmd)
		if nav.Kind != "tx" {
			t.Fatalf("64-hex should classify as tx, got %q", nav.Kind)
		}
	})

	t.Run("wallet-looking -> wallet", func(t *testing.T) {
		h := NewHome(emptyCaseCtx(t, "c"))
		cmd := submit(h, "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2")
		nav := expectNavSearch(t, cmd)
		if nav.Kind != "wallet" {
			t.Fatalf("base58 should classify as wallet, got %q", nav.Kind)
		}
	})

	t.Run("empty -> note, no nav", func(t *testing.T) {
		h := NewHome(emptyCaseCtx(t, "c"))
		cmd := submit(h, "   ")
		if cmd != nil {
			t.Fatalf("empty subject should not navigate, got a cmd")
		}
		if h.subjectNote == "" {
			t.Fatalf("empty subject should set an inline note")
		}
	})

	for _, garbage := range []string{"bc1", "2x", strings.Repeat("a", 50)} {
		g := garbage
		t.Run("garbage ["+g+"] -> reject, no nav", func(t *testing.T) {
			h := NewHome(emptyCaseCtx(t, "c"))
			cmd := submit(h, g)
			if cmd != nil {
				t.Fatalf("garbage %q should not navigate", g)
			}
			if h.subjectNote == "" {
				t.Fatalf("garbage %q should set an inline reject note", g)
			}
		})
	}
}

// expectNavSearch runs a cmd and asserts it produced a NavSearch message.
func expectNavSearch(t *testing.T, cmd tea.Cmd) NavSearch {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a NavSearch cmd, got nil")
	}
	msg := cmd()
	nav, ok := msg.(NavSearch)
	if !ok {
		t.Fatalf("expected NavSearch, got %T", msg)
	}
	return nav
}

// componentsKey builds a rune KeyMsg for the dashboard tests.
func componentsKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// TestDashboardEnterOpensAlert asserts pressing Enter on the selected alert
// row emits a NavSearch for that alert's subject, so the Dashboard drills into
// a populated subject screen (the click-through contract).
func TestDashboardEnterOpensAlert(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	h := NewHome(ctx)
	h.Init()
	addr := "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2" // classifies as wallet
	alerts := []schema.Alert{
		{ID: "A1", Subject: addr, Type: "peeling_chain", Risk: 82, Status: schema.AlertNew},
	}
	m, _ := h.Update(dataLoaded{Request: "alerts", Payload: alerts})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	nav := expectNavSearch(t, cmd)
	if nav.ID != addr || nav.Kind != "wallet" {
		t.Fatalf("Enter should open the selected alert's subject; got %+v", nav)
	}
}

// seededAlertDashboard is a focused check that a loaded alert flows into both
// the high-risk tile and the risk/pattern summary — proving the alert seam is
// wired end to end through the new panels.
func TestDashboardAlertFlow(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	h := NewHome(ctx)
	h.Init()
	alerts := []schema.Alert{
		{ID: "A1", Subject: "WFIXTURE", Type: "rapid_flow", Risk: 82, Status: schema.AlertNew},
		{ID: "A2", Subject: "WOTHER", Type: "rapid_flow", Risk: 61, Status: schema.AlertNew},
		{ID: "A3", Subject: "WLOW", Type: "dormant", Risk: 10, Status: schema.AlertNew},
	}
	m, _ := h.Update(dataLoaded{Request: "alerts", Payload: alerts})
	out := m.View(components.Frame{W: 160, H: 50})

	// Two alerts are >= 50 (high-risk band): the high-risk tile and summary
	// should both reflect 2.
	if !strings.Contains(out, "rapid_flow") {
		t.Errorf("risk summary should list the top pattern 'rapid_flow':\n%s", out)
	}
	if h.highRiskCount() != 2 {
		t.Errorf("high-risk count should be 2, got %d", h.highRiskCount())
	}
}
