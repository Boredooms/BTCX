package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
)

// StatusBar renders a one-line status band: left-aligned keyboard hints and a
// right-aligned transient status/error token. It is used by the ContextBar and
// by screens that want a local status line. It never fabricates state.
type StatusBar struct {
	styles theme.Styles
}

// NewStatusBar builds a StatusBar bound to the active styles.
func NewStatusBar(styles theme.Styles) StatusBar { return StatusBar{styles: styles} }

// View renders hints on the left and status on the right within the frame. If
// both do not fit, hints are truncated first so the status remains visible.
func (s StatusBar) View(f Frame, hints, status string, statusRole theme.Role) string {
	if f.Empty() {
		return ""
	}
	right := s.styles.Role(statusRole).Render(status)
	rightW := lipglossWidth(right)
	leftBudget := f.W - rightW - 1
	if leftBudget < 0 {
		leftBudget = 0
	}
	left := clampLine(s.styles.Role(theme.RoleLabel).Render(hints), leftBudget)
	gap := f.W - lipglossWidth(left) - rightW
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat(" ", gap) + right
	return clampBlock(line, f)
}
