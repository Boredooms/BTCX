package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSearchBoxAcceptsPaste asserts the SearchBox inserts a pasted string
// delivered as a bracketed-paste KeyMsg (Type=KeyRunes, Paste=true, many runes)
// — the shape a terminal paste (right-click / Ctrl+Shift+V) produces.
func TestSearchBoxAcceptsPaste(t *testing.T) {
	s := NewSearchBox(styles(), "addr")
	s.Focus()
	addr := "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"
	paste := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(addr), Paste: true}
	s, _ = s.Update(paste)
	if s.Value() != addr {
		t.Fatalf("paste not inserted: got %q want %q", s.Value(), addr)
	}
}

// TestSearchBoxAcceptsMultiRuneKey asserts a plain multi-rune KeyMsg (no Paste
// flag) is still inserted whole — some terminals deliver paste this way.
func TestSearchBoxAcceptsMultiRuneKey(t *testing.T) {
	s := NewSearchBox(styles(), "addr")
	s.Focus()
	txid := "fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14"
	s, _ = s.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(txid)})
	if s.Value() != txid {
		t.Fatalf("multi-rune key not inserted: got %q want %q", s.Value(), txid)
	}
}
