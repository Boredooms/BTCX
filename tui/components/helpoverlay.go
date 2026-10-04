package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay renders a full keymap reference generated from key bindings
// (design §3, §5). It is opened with `?` and closed with Esc; the parent owns
// the open/close state and supplies the binding grid so the overlay stays a
// pure renderer over the single keymap source of truth.
type HelpOverlay struct {
	styles theme.Styles
	groups [][]key.Binding
}

// NewHelpOverlay builds a help overlay from a binding grid (e.g. a keymap's
// FullHelp()).
func NewHelpOverlay(styles theme.Styles, groups [][]key.Binding) HelpOverlay {
	return HelpOverlay{styles: styles, groups: groups}
}

// SetGroups replaces the binding grid.
func (h *HelpOverlay) SetGroups(groups [][]key.Binding) { h.groups = groups }

// View renders the overlay centered within the frame.
func (h HelpOverlay) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(h.styles.Role(theme.RoleTitle).Render("Keyboard Reference"))
	b.WriteString("\n\n")
	for _, group := range h.groups {
		for _, bind := range group {
			hk := bind.Help()
			if hk.Key == "" {
				continue
			}
			row := h.styles.Role(theme.RoleInfo).Render(padRight(hk.Key, 12)) +
				h.styles.Role(theme.RoleValue).Render(hk.Desc)
			b.WriteString(clampLine(row, f.W-4) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(h.styles.Role(theme.RoleLabel).Render("[esc] close"))

	boxW := min(f.W-4, 50)
	if boxW < 10 {
		boxW = f.W
	}
	box := h.styles.Panel.Width(boxW).Render(b.String())
	return lipgloss.Place(f.W, f.H, lipgloss.Center, lipgloss.Center, box)
}
