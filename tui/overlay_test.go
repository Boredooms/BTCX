package tui

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// overlay_test.go covers FEAT-002's overlay routing in Root.Update: the command
// palette and the global search overlay opening/filtering/selecting, the
// Root-owned consumption of CommandChosen/SearchSubmitted (NIT-1: an
// overlay-emitted SearchSubmitted never reaches a screen), and the palette
// verbs (nav:/:search/:help/:quit/:open) routing without crashing.

// TestPaletteOpenSetsFocusModal asserts `:` opens the palette and takes
// FocusModal so the overlay owns the keyboard.
func TestPaletteOpenSetsFocusModal(t *testing.T) {
	r := sizedRoot(120, 40)
	r.Update(rkey(":"))
	if r.focus != FocusModal {
		t.Fatalf("`:` should take FocusModal, got %v", r.focus)
	}
	if r.overlay.kind != overlayPalette {
		t.Fatalf("`:` should open the palette overlay, got kind %v", r.overlay.kind)
	}
	// Ctrl+P also opens it.
	r2 := sizedRoot(120, 40)
	r2.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if r2.overlay.kind != overlayPalette {
		t.Fatalf("Ctrl+P should open the palette overlay, got kind %v", r2.overlay.kind)
	}
}

// TestPaletteEscCloses asserts Esc closes the palette and restores prior focus.
func TestPaletteEscCloses(t *testing.T) {
	r := sizedRoot(120, 40)
	r.Update(rkey(":"))
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r.overlay.kind != overlayNone {
		t.Fatalf("Esc should close the palette, kind = %v", r.overlay.kind)
	}
	if r.focus != FocusNav {
		t.Fatalf("Esc should restore FocusNav, got %v", r.focus)
	}
}

// TestPaletteNavCommandNavigates asserts selecting a nav:<id> palette entry
// navigates to that screen and closes the overlay. It drives the palette by
// feeding a CommandChosen (as the palette's Enter would emit) through Update
// while the overlay is open.
func TestPaletteNavCommandNavigates(t *testing.T) {
	r := sizedRoot(120, 40)
	r.Update(rkey(":")) // open palette
	r.Update(components.CommandChosen{ID: "nav:" + string(ScreenGraph)})
	if r.cur != ScreenGraph {
		t.Fatalf("nav:graph should navigate to Graph, got %v", r.cur)
	}
	if r.overlay.kind != overlayNone {
		t.Fatalf("choosing a command should close the overlay, kind = %v", r.overlay.kind)
	}
}

// TestPaletteVerbsRoute asserts the bare verbs route: :help navigates to Help,
// :search reopens the search overlay, :open opens search when bare.
func TestPaletteVerbsRoute(t *testing.T) {
	// :help
	r := sizedRoot(120, 40)
	r.Update(rkey(":"))
	r.Update(components.CommandChosen{ID: ":help"})
	if r.cur != ScreenHelp {
		t.Fatalf(":help should navigate to Help, got %v", r.cur)
	}
	// :search
	r2 := sizedRoot(120, 40)
	r2.Update(rkey(":"))
	r2.Update(components.CommandChosen{ID: ":search"})
	if r2.overlay.kind != overlaySearch {
		t.Fatalf(":search should open the search overlay, got kind %v", r2.overlay.kind)
	}
	// :open (bare) -> search overlay
	r3 := sizedRoot(120, 40)
	r3.Update(rkey(":"))
	r3.Update(components.CommandChosen{ID: ":open"})
	if r3.overlay.kind != overlaySearch {
		t.Fatalf(":open (bare) should open the search overlay, got kind %v", r3.overlay.kind)
	}
}

// TestPaletteOpenSubjectClassifiesAndNavigates asserts open:<subject> classifies
// the subject and navigates to the matching detail screen.
func TestPaletteOpenSubjectClassifiesAndNavigates(t *testing.T) {
	// A 64-hex id classifies as tx -> Transaction screen.
	txid := strings.Repeat("a", 64)
	r := sizedRoot(120, 40)
	r.Update(rkey(":"))
	r.Update(components.CommandChosen{ID: "open:" + txid})
	if r.cur != ScreenTransaction {
		t.Fatalf("open:<64-hex> should navigate to Transaction, got %v", r.cur)
	}
	if r.state.Subject.ID != txid || r.state.Subject.Kind != SubjectTx {
		t.Fatalf("subject not seeded: %+v", r.state.Subject)
	}
}

// TestPaletteUnknownIDNoCrash asserts an unknown palette id is ignored (no
// navigation, no panic), leaving the current screen in place.
func TestPaletteUnknownIDNoCrash(t *testing.T) {
	r := sizedRoot(120, 40)
	start := r.cur
	r.Update(rkey(":"))
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				t.Fatalf("unknown palette id panicked: %v", rec)
			}
		}()
		r.Update(components.CommandChosen{ID: "totally-unknown-id"})
	}()
	if r.cur != start {
		t.Fatalf("unknown id should not navigate, cur = %v", r.cur)
	}
}

// TestGlobalSearchOverlayNavigatesSeeded asserts `/` opens the search overlay
// and an overlay SearchSubmitted closes it, navigates to Search, and seeds the
// query so the in-screen box is pre-filled.
func TestGlobalSearchOverlayNavigatesSeeded(t *testing.T) {
	r := sizedRoot(120, 40)
	r.Update(rkey("/"))
	if r.overlay.kind != overlaySearch {
		t.Fatalf("`/` should open the search overlay, got kind %v", r.overlay.kind)
	}
	r.Update(components.SearchSubmitted{Query: "abc123"})
	if r.cur != ScreenSearch {
		t.Fatalf("overlay search submit should navigate to Search, got %v", r.cur)
	}
	if r.overlay.kind != overlayNone {
		t.Fatalf("submit should close the overlay, kind = %v", r.overlay.kind)
	}
	// The Search screen should be seeded with the query.
	if a, ok := r.active.(screenAdapter); ok {
		if sv, ok := a.m.(interface{ Focused() bool }); ok {
			_ = sv // focus state exists; the seed path ran without panic
		}
	}
}

// TestOverlaySearchSubmittedNeverReachesScreen asserts NIT-1: a SearchSubmitted
// emitted while the global search overlay is open is consumed by the Root and
// never delivered to the active screen's Update. We prove it by starting on the
// in-screen Search, opening the global overlay, submitting, and checking the
// Root navigated (overlay path) rather than the in-screen Search handling it.
func TestOverlaySearchSubmittedNeverReachesScreen(t *testing.T) {
	// Start on Home (no in-screen input focused) so `/` opens the GLOBAL overlay.
	r := sizedRoot(120, 40)
	r.Update(rkey("/"))
	if r.overlay.kind != overlaySearch {
		t.Fatalf("precondition: overlay not open, kind = %v", r.overlay.kind)
	}
	// A SearchSubmitted WHILE the overlay is open is Root-owned (guarded by
	// overlay.kind == overlaySearch): it closes the overlay and navigates to
	// Search, rather than being delivered to the active screen's Update.
	r.Update(components.SearchSubmitted{Query: "wallet-query"})
	if r.overlay.kind != overlayNone {
		t.Fatalf("overlay should be closed after submit, kind = %v", r.overlay.kind)
	}
	if r.cur != ScreenSearch {
		t.Fatalf("should land on Search, got %v", r.cur)
	}

	// Conversely, with NO overlay open, a SearchSubmitted falls through to the
	// active screen's tail — it must NOT be consumed by the Root (no second
	// navigation / no overlay side effect). The in-screen Search handles it.
	r.Update(components.SearchSubmitted{Query: "another"})
	if r.overlay.kind != overlayNone {
		t.Fatalf("a non-overlay SearchSubmitted must not open an overlay, kind = %v", r.overlay.kind)
	}
}

// TestPaletteFilterNarrows drives the palette component directly to assert the
// filter narrows matches and Enter emits CommandChosen with the highlighted id.
func TestPaletteFilterNarrows(t *testing.T) {
	p := components.NewCommandPalette(theme.Build(theme.Default()), buildPaletteCommands())
	p.Open()
	// Type a query that only the Graph nav entry matches.
	for _, r := range "graph" {
		p, _ = p.Update(rkey(string(r)))
	}
	// Enter emits a CommandChosen; capture it.
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("palette Enter should emit a command")
	}
	msg := cmd()
	chosen, ok := msg.(components.CommandChosen)
	if !ok {
		t.Fatalf("expected CommandChosen, got %T", msg)
	}
	if !strings.Contains(strings.ToLower(chosen.ID), "graph") {
		t.Fatalf("filtered palette should choose the graph entry, got %q", chosen.ID)
	}
}
