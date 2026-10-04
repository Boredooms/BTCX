package tui

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
	"github.com/bctx/bctx/tui/geoip"
	"github.com/bctx/bctx/tui/mapdata"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// interactive_gate_test.go is the committed interaction gate (FEAT-005, design
// Test Plan tail). TestInteractiveGate drives a scripted []tea.KeyMsg sequence
// through Root.Update and asserts the full interaction contract shipped by
// FEAT-001..004:
//
//   - focus region transitions (Tab cycles Nav<->Body; Esc layering),
//   - SideNav navigation (cursor move + Enter, and the number jump),
//   - command-palette open/filter/choose/close (':' overlay) and the global
//     search overlay ('/') seeding the Search screen,
//   - geo-map viewport interaction (focus delivery + pan/zoom/select/reset),
//   - and the HARD no-overflow invariant after every single keypress at a
//     fixed terminal size.
//
// It is backed by `go test -run TestInteractiveGate` and invoked by
// scripts/phase8_tui_interactive.sh, which prints the committed gate summary.
// The gate prints its own `== SUMMARY: PASS=N FAIL=M ==` line (via t.Log) so
// the script surfaces the same PASS/FAIL shape as the other five gates.

// gateW/gateH is the fixed terminal size the gate asserts at. 160x50 is a
// production-representative wide shell that renders every region bordered.
const (
	gateW = 160
	gateH = 50
)

// gate accumulates PASS/FAIL counts and renders the committed summary line.
type gate struct {
	t    *testing.T
	root *Root
	pass int
	fail int
}

// check records a boolean assertion as a PASS/FAIL line (mirrors the shell gate
// harness so the script output is uniform across all gates).
func (g *gate) check(name string, ok bool, detail string) {
	if ok {
		g.pass++
		g.t.Logf("PASS: %s", name)
		return
	}
	g.fail++
	g.t.Errorf("FAIL: %s — %s", name, detail)
}

// step feeds one key through Root.Update, drains the returned command so async
// nav/data loads settle deterministically, and asserts the hard no-overflow
// invariant on the resulting frame. Every scripted keystroke is overflow-gated
// so a broken box anywhere fails the gate.
func (g *gate) step(name string, k tea.KeyMsg) {
	g.drive(k)
	g.assertNoOverflow(name)
}

// drive applies a key and folds every message its command emits back into the
// Root (bounded, non-recursive beyond one batch level — the TUI's loads do not
// chain), so a nav that triggers a screen Init renders populated, not loading.
func (g *gate) drive(msg tea.Msg) {
	_, cmd := g.root.Update(msg)
	for _, m := range drainGateCmd(cmd) {
		g.root.Update(m)
	}
}

// assertNoOverflow renders the full shell and asserts no line exceeds gateW and
// the line count does not exceed gateH (design §6 hard no-box-break invariant).
func (g *gate) assertNoOverflow(name string) {
	out := g.root.View()
	lines := strings.Split(out, "\n")
	if len(lines) > gateH {
		g.check("no-overflow:"+name, false,
			lineCount(len(lines), gateH))
		return
	}
	for i, ln := range lines {
		if w := lipgloss.Width(ln); w > gateW {
			g.check("no-overflow:"+name, false,
				"line "+itoa(i)+" width "+itoa(w)+" exceeds "+itoa(gateW))
			return
		}
	}
	g.check("no-overflow:"+name, true, "")
}

func lineCount(got, max int) string {
	return "line count " + itoa(got) + " exceeds height " + itoa(max)
}

// drainGateCmd flattens a (possibly batched) tea.Cmd into the messages it
// emits, running each leaf command once. It understands tea.BatchMsg so a
// tea.Batch of data-load commands is fully resolved; it does not recurse into
// commands the resulting messages themselves might schedule.
func drainGateCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, drainGateCmd(c)...)
		}
	case nil:
	default:
		out = append(out, m)
	}
	return out
}

// gateRoot builds a Root over a seeded in-memory case with a FROZEN clock and a
// throwaway HOME holding the synthetic GeoIP City fixture + the world-geometry
// asset, so the geo-map scene resolves + plots offline. No network, no real
// ~/.bctx. Mirrors geoFixtureCtx (tui/screens) at the Root level.
func gateRoot(t *testing.T) *Root {
	t.Helper()
	installInteractiveFixtures(t)

	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "gate.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	ctx := context.Background()
	seedInteractiveCase(t, repo)

	// Build a real offline engine so the Root has a live Cases manager + Config;
	// navTo -> refreshStatus reads Cases.Active(), which panics on a nil manager.
	a := &app.App{Repo: repo, CaseID: "gate-case", Cleanup: func() {}}
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	built, err := app.Build(cfg)
	if err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	a.Cases = built.Cases
	a.Engine = built.Engine

	r := NewRoot(ctx, a)
	r.nowFn = frozenClock()
	runScreenInit(r)
	r.Update(tea.WindowSizeMsg{Width: gateW, Height: gateH})
	return r
}

// installInteractiveFixtures installs the committed GeoIP City fixture and the
// world-geometry asset under a throwaway HOME (offline, deterministic). The
// fixtures live under the sibling package testdata directories.
func installInteractiveFixtures(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	geoDir := filepath.Join(home, ".bctx", "geoip")
	if _, err := geoip.Install(geoDir, filepath.Join("screens", "testdata", "city-fixture.mmdb"),
		geoip.RegistryEntry{Source: "BCTX synthetic City fixture", Version: "fixture"}); err != nil {
		t.Fatalf("install city fixture: %v", err)
	}
	mapDir := filepath.Join(home, ".bctx", "mapdata")
	if _, err := mapdata.Install(mapDir, filepath.Join("mapdata", "testdata", "world-110m.asset"),
		mapdata.RegistryEntry{Source: "Natural Earth (fixture)", Version: "fixture", License: "public domain"}); err != nil {
		t.Fatalf("install map asset: %v", err)
	}
}

// seedInteractiveCase seeds one transaction plus a few network observations
// whose IPs the City fixture resolves, so Search returns a hit and the Geo Map
// plots clusters. Fixtures live only in tests (AGENTS §15/§16).
func seedInteractiveCase(t *testing.T, repo *sqlite.Repository) {
	t.Helper()
	ctx := context.Background()
	t0 := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	tx := schema.Transaction{
		TxID:      "TXGATE1",
		Timestamp: t0,
		FeeBTC:    0.0001,
		Inputs:    []schema.TransactionInput{{Address: "WGATE", AmountBTC: 1.0, Index: 0}},
		Outputs:   []schema.TransactionOutput{{Address: "WOUT", AmountBTC: 0.99, Index: 0}},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	obs := []schema.NetworkObservation{
		{ID: "g1", TxID: "TXGATE1", SrcIP: "81.2.69.142", DstIP: "151.101.1.69", Timestamp: t0},
		{ID: "g2", TxID: "TXGATE1", SrcIP: "81.2.69.143", DstIP: "151.101.1.69", Timestamp: t0.Add(time.Hour)},
	}
	if err := repo.SaveNetworkObservations(ctx, obs); err != nil {
		t.Fatalf("seed obs: %v", err)
	}
}

// TestInteractiveGate is the committed interaction gate. It is deterministic,
// offline, and asserts the shipped focus/nav/overlay/map contract plus the hard
// no-overflow invariant after every keypress. scripts/phase8_tui_interactive.sh
// invokes it; the printed `== SUMMARY ==` line is the gate's committed count.
func TestInteractiveGate(t *testing.T) {
	g := &gate{t: t, root: gateRoot(t)}

	// --- Focus model: startup, Tab cycle, Esc layering (FEAT-001, §A.5) ---
	g.check("startup focus is Nav", g.root.focus == FocusNav, focusName(g.root.focus))
	g.assertNoOverflow("startup")

	g.step("tab -> Body", tea.KeyMsg{Type: tea.KeyTab})
	g.check("Tab cycles Nav->Body", g.root.focus == FocusBody, focusName(g.root.focus))
	g.step("tab -> Nav", tea.KeyMsg{Type: tea.KeyTab})
	g.check("Tab cycles Body->Nav", g.root.focus == FocusNav, focusName(g.root.focus))
	g.step("shift+tab -> Body", tea.KeyMsg{Type: tea.KeyShiftTab})
	g.check("Shift+Tab cycles Nav->Body", g.root.focus == FocusBody, focusName(g.root.focus))
	g.step("esc Body -> Nav", tea.KeyMsg{Type: tea.KeyEsc})
	g.check("Esc in Body returns focus to Nav", g.root.focus == FocusNav, focusName(g.root.focus))

	// --- SideNav navigation: cursor move + Enter opens the screen (§A.5) ---
	// Move the cursor to a NON-type-first screen (Wallet, index 2) so this
	// checks the GENERAL nav-Enter focus behavior: a normal screen keeps Nav
	// focus for the next move. (The Analysis screen at index 1 is a special
	// type-first screen checked separately below.)
	startCur := g.root.navCursor
	g.step("nav down", tea.KeyMsg{Type: tea.KeyDown})
	g.step("nav down", tea.KeyMsg{Type: tea.KeyDown})
	g.check("Nav down moves the SideNav cursor", g.root.navCursor == startCur+2,
		"cursor "+itoa(g.root.navCursor))
	g.step("nav enter", tea.KeyMsg{Type: tea.KeyEnter})
	g.check("Nav Enter navigates to the cursor screen", g.root.cur != ScreenHome,
		"cur "+string(g.root.cur))
	g.check("Nav Enter on a normal screen keeps Nav focus", g.root.focus == FocusNav,
		focusName(g.root.focus))

	// --- Analysis is type-first: opening it takes Body focus so the user can
	// type into its subject box immediately (regression for the "can't type /
	// stuck on querying" bug) ---
	g.step("jump to Analysis (2)", rkey("2"))
	g.check("number jump selects the Analysis screen", g.root.cur == ScreenSearch,
		"cur "+string(g.root.cur))
	g.check("opening Analysis takes Body focus for typing", g.root.focus == FocusBody,
		focusName(g.root.focus))
	// Esc from the Analysis body returns focus to Nav (does not pop the stack).
	g.step("esc Analysis body -> Nav", tea.KeyMsg{Type: tea.KeyEsc})
	g.check("Esc from Analysis body returns to Nav", g.root.focus == FocusNav,
		focusName(g.root.focus))

	// --- Command palette overlay: open, filter, choose, close (FEAT-002) ---
	g.step("open palette (:)", rkey(":"))
	g.check("':' opens the command palette (FocusModal)", g.root.focus == FocusModal,
		focusName(g.root.focus))
	g.check("palette overlay is the open kind", g.root.overlay.kind == overlayPalette,
		"kind")
	g.step("palette esc closes", tea.KeyMsg{Type: tea.KeyEsc})
	g.check("Esc closes the palette and restores focus", g.root.overlay.kind == overlayNone,
		"kind")
	g.check("palette close restores the prior focus region", g.root.focus != FocusModal,
		focusName(g.root.focus))

	// Palette nav: verb routes to a screen (choose a nav: command directly).
	g.drive(rkey(":"))
	g.drive(components.CommandChosen{ID: "nav:" + string(ScreenGeoMap)})
	g.assertNoOverflow("palette nav:geomap")
	g.check("palette nav: command navigates", g.root.cur == ScreenGeoMap,
		"cur "+string(g.root.cur))
	g.check("palette consumed the choice (overlay closed)", g.root.overlay.kind == overlayNone,
		"kind")

	// --- Geo map viewport: focus delivery + pan/zoom/select/reset (FEAT-003) ---
	g.drive(rkey("8")) // jump to Geo Map (Nav), then focus Body so keys arrive
	g.assertNoOverflow("jump geomap")
	g.check("number jump selects the Geo Map screen", g.root.cur == ScreenGeoMap,
		"cur "+string(g.root.cur))
	g.step("focus Body on the map", tea.KeyMsg{Type: tea.KeyTab})
	g.check("Tab delivers keys to the map Body", g.root.focus == FocusBody, focusName(g.root.focus))
	// The viewport keys must not overflow and must not crash: select, zoom, pan,
	// open-detail, reset. Each is overflow-gated by step().
	g.step("map select next (n)", rkey("n"))
	g.step("map zoom in (+)", rkey("+"))
	g.step("map zoom in (+)", rkey("+"))
	g.step("map pan right (l)", rkey("l"))
	g.step("map pan up (k)", rkey("k"))
	g.step("map open detail (enter)", tea.KeyMsg{Type: tea.KeyEnter})
	g.step("map reset view (0)", rkey("0"))
	g.check("map stays on the Geo Map screen through the viewport keys",
		g.root.cur == ScreenGeoMap, "cur "+string(g.root.cur))

	// --- Global search overlay: '/' opens, submit seeds the Search screen ---
	// Return to Nav focus on a plain screen first (the map Body does not hold an
	// in-screen input, so '/' opens the global overlay rather than typing).
	g.step("esc map Body -> Nav", tea.KeyMsg{Type: tea.KeyEsc})
	g.drive(rkey("1")) // back to a known Nav screen
	g.step("open search overlay (/)", rkey("/"))
	g.check("'/' opens the search overlay (FocusModal)", g.root.focus == FocusModal,
		focusName(g.root.focus))
	g.check("search overlay is the open kind", g.root.overlay.kind == overlaySearch,
		"kind")
	// Type a known subject and submit; the overlay closes and seeds Search.
	for _, r := range "WGATE" {
		g.drive(rkey(string(r)))
	}
	g.assertNoOverflow("search typing")
	g.drive(tea.KeyMsg{Type: tea.KeyEnter})
	g.assertNoOverflow("search submit")
	g.check("search submit closes the overlay", g.root.overlay.kind == overlayNone, "kind")
	g.check("search submit navigates to the Search screen", g.root.cur == ScreenSearch,
		"cur "+string(g.root.cur))

	// --- A broad key sweep in Body must never overflow or wedge (§A.5) ---
	for _, k := range sweepKeys() {
		g.step("sweep", k)
	}
	g.check("model stays responsive after the key sweep", g.root.View() != "", "empty frame")

	t.Logf("== SUMMARY: PASS=%d FAIL=%d ==", g.pass, g.fail)
}

// focusName renders a FocusRegion for failure messages.
func focusName(f FocusRegion) string {
	switch f {
	case FocusNav:
		return "FocusNav"
	case FocusBody:
		return "FocusBody"
	case FocusModal:
		return "FocusModal"
	default:
		return "FocusUnknown"
	}
}

// sweepKeys is a broad, deterministic keystroke set the gate replays in Body to
// prove no key overflows or wedges the shell (a committed echo of the
// TestNoKeyCrashes contract, here under the no-overflow assertion).
func sweepKeys() []tea.KeyMsg {
	var keys []tea.KeyMsg
	for c := 'a'; c <= 'z'; c++ {
		keys = append(keys, rkey(string(c)))
	}
	for c := '0'; c <= '9'; c++ {
		keys = append(keys, rkey(string(c)))
	}
	keys = append(keys,
		tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyRight},
		tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyEsc},
		rkey("?"), rkey(" "), tea.KeyMsg{Type: tea.KeyPgUp}, tea.KeyMsg{Type: tea.KeyPgDown},
	)
	return keys
}
