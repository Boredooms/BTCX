package components

import (
	"strings"
	"testing"
)

// navFixture builds a SideNav with three items for cursor/marker tests.
func navFixture() SideNav {
	n := NewSideNav(styles())
	n.SetItems([]NavItem{
		{Key: "1", Title: "Dashboard", Active: true},
		{Key: "2", Title: "Search"},
		{Key: "3", Title: "Wallet"},
	})
	return n
}

// TestSideNavCursorClampsBothEnds asserts MoveUp/MoveDown never run off either
// end of the list (design §A.4).
func TestSideNavCursorClampsBothEnds(t *testing.T) {
	n := navFixture()

	// Up from the top is a no-op.
	n.MoveUp()
	if got := n.Cursor(); got != 0 {
		t.Fatalf("MoveUp at top: cursor = %d, want 0", got)
	}

	// Walk to the bottom and one past.
	n.MoveDown()
	n.MoveDown()
	n.MoveDown() // one past the end
	if got := n.Cursor(); got != 2 {
		t.Fatalf("MoveDown past end: cursor = %d, want 2 (clamped)", got)
	}

	// SetCursor clamps out-of-range inputs.
	n.SetCursor(99)
	if got := n.Cursor(); got != 2 {
		t.Errorf("SetCursor(99): cursor = %d, want 2", got)
	}
	n.SetCursor(-5)
	if got := n.Cursor(); got != 0 {
		t.Errorf("SetCursor(-5): cursor = %d, want 0", got)
	}
}

// TestSideNavSelectedKey asserts SelectedKey returns the key at the cursor and
// "" when the list is empty (design §A.4).
func TestSideNavSelectedKey(t *testing.T) {
	n := navFixture()
	n.SetCursor(1)
	if got := n.SelectedKey(); got != "2" {
		t.Errorf("SelectedKey at cursor 1 = %q, want 2", got)
	}

	empty := NewSideNav(styles())
	if got := empty.SelectedKey(); got != "" {
		t.Errorf("SelectedKey on empty nav = %q, want empty", got)
	}
}

// TestSideNavSelectedLabelVisible asserts the focused+selected row still
// renders its KEY and TITLE text (regression for the cyan-on-cyan bug where the
// Selected style's foreground matched its background and the label vanished).
// The literal glyphs must survive the styling wrap.
func TestSideNavSelectedLabelVisible(t *testing.T) {
	n := navFixture()
	n.SetCursor(2) // Wallet
	n.SetFocused(true)
	out := n.View(Frame{W: 30, H: 10})
	if !strings.Contains(out, "Wallet") {
		t.Errorf("selected row must keep its title text; got:\n%s", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("selected row must keep its key; got:\n%s", out)
	}
}

// TestSideNavFocusedVsActiveMarkers asserts the focused cursor row and the
// active screen row render distinct markers (design §A.3): the cursor glyph ▸
// marks the selection, and the active glyph ◂ marks the open screen.
func TestSideNavFocusedVsActiveMarkers(t *testing.T) {
	n := navFixture()
	n.SetCursor(1) // Search is the cursor; Dashboard is Active.
	n.SetFocused(true)
	out := n.View(Frame{W: 30, H: 10})

	if !strings.Contains(out, "▸") {
		t.Error("focused nav should render the cursor marker ▸")
	}
	if !strings.Contains(out, "◂") {
		t.Error("active screen should keep its active marker ◂")
	}

	// Unfocused: the cursor marker is still drawn (the row is highlighted dim),
	// but the component is not focused — assert it still renders without panic
	// and keeps the active marker.
	n.SetFocused(false)
	out2 := n.View(Frame{W: 30, H: 10})
	if strings.TrimSpace(out2) == "" {
		t.Error("unfocused nav should still render")
	}
	if !strings.Contains(out2, "◂") {
		t.Error("unfocused nav should keep the active marker ◂")
	}
}
