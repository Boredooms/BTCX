package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// pasteMsg builds a bracketed-paste KeyMsg (Paste=true) carrying the whole
// string, the shape a terminal paste delivers.
func pasteMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true}
}

// TestPasteIntoGlobalSearchOverlay asserts a paste into the open `/` search
// overlay is inserted as text (not routed to nav), even though the pasted id
// starts with a digit that is a registered jump key.
func TestPasteIntoGlobalSearchOverlay(t *testing.T) {
	r := NewRoot(context.Background(), nil)
	r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	// Open the global search overlay (`/`).
	r.openSearch()
	if r.focus != FocusModal {
		t.Fatalf("search overlay should take FocusModal, got %v", r.focus)
	}
	// Paste a txid-shaped id that begins with '1' (a nav jump key).
	id := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	r.Update(pasteMsg(id))
	if got := r.overlay.search.Value(); got != id {
		t.Fatalf("paste should fill the search box with %q, got %q", id, got)
	}
	// The paste must NOT have navigated away (still the home screen under the
	// overlay, overlay still open).
	if r.overlay.kind != overlaySearch {
		t.Errorf("paste must not close/redirect the search overlay")
	}
}

// TestPasteIntoBodyInputDoesNotNavigate asserts that while the body owns the
// keyboard, a paste is forwarded to the active screen rather than triggering a
// nav jump on the pasted text's first character.
func TestPasteIntoBodyInputDoesNotNavigate(t *testing.T) {
	r := NewRoot(context.Background(), nil)
	r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	r.focus = FocusBody
	before := r.cur
	// Paste a string starting with '3' (the Wallet jump key) — must not jump.
	r.Update(pasteMsg("3FupZp77ySr7jwoLYEJ9mwzJpvoNBXsBnE"))
	if r.cur != before {
		t.Errorf("paste into body must not navigate; screen changed %q -> %q", before, r.cur)
	}
}

// TestPasteNoFocusIsNoop asserts a stray paste with nothing focused does not
// crash and does not navigate.
func TestPasteNoFocusIsNoop(t *testing.T) {
	r := NewRoot(context.Background(), nil)
	r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	before := r.cur
	r.focus = FocusNav
	r.Update(pasteMsg("1234"))
	if r.cur != before {
		t.Errorf("paste with nav focus must not navigate; got %q", r.cur)
	}
}

var _ = strings.TrimSpace
