package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
)

// NavItem is one SideNav entry. The root builds these from the screen registry
// (the single source of truth) and hands them to the SideNav; the component
// does not know about the registry directly, which keeps components a leaf.
type NavItem struct {
	Key    string
	Title  string
	Active bool
}

// SideNav is the vertical list of registry screens (Nav=true). It highlights
// the active screen and shows nav key hints. In compact widths it collapses to
// a one-column key rail (design §3, §6). The Root drives the selection cursor
// and focus flag; the component only renders them (design §A.4), so it stays a
// leaf with no registry knowledge.
type SideNav struct {
	styles  theme.Styles
	items   []NavItem
	cursor  int  // selected row index (driven by the Root)
	focused bool // render the cursor with the focused highlight
}

// NewSideNav builds an empty SideNav bound to the active styles.
func NewSideNav(styles theme.Styles) SideNav { return SideNav{styles: styles} }

// SetItems replaces the nav items (ordered as the registry lists them). The
// cursor is re-clamped so it never dangles past the new list.
func (n *SideNav) SetItems(items []NavItem) {
	n.items = items
	n.clampCursor()
}

// SetCursor moves the selection to i, clamped to [0,len(items)-1].
func (n *SideNav) SetCursor(i int) {
	n.cursor = i
	n.clampCursor()
}

// Cursor returns the current selection index.
func (n SideNav) Cursor() int { return n.cursor }

// SetFocused toggles whether the cursor renders with the focused highlight.
func (n *SideNav) SetFocused(f bool) { n.focused = f }

// MoveUp moves the cursor up one row, clamped at the top.
func (n *SideNav) MoveUp() {
	if n.cursor > 0 {
		n.cursor--
	}
}

// MoveDown moves the cursor down one row, clamped at the bottom.
func (n *SideNav) MoveDown() {
	if n.cursor < len(n.items)-1 {
		n.cursor++
	}
}

// SelectedKey returns the registry key of the row under the cursor, or "" when
// the nav is empty. The Root maps this to a ScreenID via the registry.
func (n SideNav) SelectedKey() string {
	if n.cursor < 0 || n.cursor >= len(n.items) {
		return ""
	}
	return n.items[n.cursor].Key
}

// clampCursor keeps the cursor within [0,len(items)-1]; an empty list pins it
// at 0 so SelectedKey degrades to "".
func (n *SideNav) clampCursor() {
	if n.cursor < 0 {
		n.cursor = 0
	}
	if n.cursor >= len(n.items) {
		n.cursor = len(n.items) - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
}

// Width returns the SideNav column width for a terminal width. Compact hides
// the labeled nav (width 0; the rail is drawn inside the workspace instead),
// matching tui layout's NavWidth.
func (n SideNav) Width(termWidth int) int {
	switch {
	case termWidth >= 200:
		return 28
	case termWidth >= 160:
		return 24
	case termWidth >= 100:
		return 20
	default:
		return 0
	}
}

// View renders the nav list into its frame. With zero width (compact) it
// renders nothing — the compact key rail is a workspace concern.
func (n SideNav) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(n.styles.Role(theme.RoleLabel).Render("NAV"))
	b.WriteString("\n")
	for i, it := range n.items {
		// Active marker (◂) names the open screen; cursor marker (▸) names what
		// Enter will open. Both read at a glance, so "where am I" and "what will
		// Enter do" stay distinct (design §A.3).
		marker := "  "
		if it.Active {
			marker = "◂ "
		}
		cursorMark := "  "

		switch {
		case i == n.cursor && n.focused:
			// Focused selection: the ENTIRE row (key + title + markers) gets the
			// inverse-on-cyan highlight fill so the label stays readable and the
			// selection reads as one solid bar. Styling the whole row — not just
			// the title — fixes the disjoint "cyan key, highlighted title" look
			// and the vanishing label.
			cursorMark = "▸ "
			row := cursorMark + it.Key + " " + it.Title + " " + marker
			b.WriteString(n.styles.Selected.Render(row))
			b.WriteString("\n")
			continue
		case i == n.cursor:
			// Nav not focused but this is the cursor row: dim selected foreground
			// (no fill), with the cursor glyph.
			cursorMark = "▸ "
			line := cursorMark + n.styles.Role(theme.RoleInfo).Render(it.Key) + " " +
				n.styles.Role(theme.RoleSelected).Render(it.Title) + " " + marker
			b.WriteString(line)
			b.WriteString("\n")
			continue
		case it.Active:
			line := cursorMark + n.styles.Role(theme.RoleInfo).Render(it.Key) + " " +
				n.styles.Role(theme.RoleSelected).Render(it.Title) + " " + marker
			b.WriteString(line)
			b.WriteString("\n")
			continue
		}

		line := cursorMark + n.styles.Role(theme.RoleInfo).Render(it.Key) + " " +
			n.styles.Role(theme.RoleValue).Render(it.Title) + " " + marker
		b.WriteString(line)
		b.WriteString("\n")
	}
	return clampBlock(b.String(), f)
}
