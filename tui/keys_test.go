package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestDispatchGlobalWhenUnfocused(t *testing.T) {
	km := NewGlobalKeyMap()
	cases := map[string]string{
		"j":     "down",
		"k":     "up",
		"q":     "quit",
		"/":     "search",
		":":     "palette",
		"?":     "help",
		"enter": "enter",
		"esc":   "esc",
		"tab":   "tab",
	}
	for in, want := range cases {
		if got := km.Dispatch(keyMsg(in), false); got != want {
			t.Errorf("Dispatch(%q, unfocused) = %q, want %q", in, got, want)
		}
	}
}

func TestDispatchSuppressedWhenInputFocused(t *testing.T) {
	km := NewGlobalKeyMap()
	// Single-letter nav must be suppressed so typing works.
	for _, in := range []string{"j", "k", "q", "/", ":", "?", "g"} {
		if got := km.Dispatch(keyMsg(in), true); got != "" {
			t.Errorf("Dispatch(%q, focused) = %q, want suppressed", in, got)
		}
	}
	// Esc and Enter survive focus so the input can submit/cancel.
	if got := km.Dispatch(keyMsg("esc"), true); got != "esc" {
		t.Errorf("Dispatch(esc, focused) = %q, want esc", got)
	}
	if got := km.Dispatch(keyMsg("enter"), true); got != "enter" {
		t.Errorf("Dispatch(enter, focused) = %q, want enter", got)
	}
}

func TestDispatchJumpKeys(t *testing.T) {
	km := NewGlobalKeyMap()
	// A registered nav key resolves to a jump token carrying the literal key.
	got := km.Dispatch(keyMsg("3"), false)
	if got != "jump:3" {
		t.Errorf("Dispatch(3) = %q, want jump:3", got)
	}
	if def := LookupKey("3"); def == nil || def.ID != ScreenWallet {
		t.Errorf("key 3 should map to wallet screen")
	}
}
