package components

import "github.com/bctx/bctx/tui/theme"

// ContextBar is the bottom band of the shell (design §3): left = the active
// screen's keyboard hints, center/right = the last event/alert and any
// transient status or error. It wraps a StatusBar for the hint/status split.
type ContextBar struct {
	styles theme.Styles
	status StatusBar

	// Fields set by the root/active screen each frame.
	Focus      string // focus-state label: NAV / BODY / overlay name (design §A.3)
	Hints      string
	Event      string
	Status     string
	StatusRole theme.Role
}

// NewContextBar builds a ContextBar bound to the active styles.
func NewContextBar(styles theme.Styles) ContextBar {
	return ContextBar{styles: styles, status: NewStatusBar(styles), StatusRole: theme.RoleInfo}
}

// View renders the context band. The event text (if any) is appended to the
// hints on the left; the status token stays right-aligned.
func (c ContextBar) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	hints := c.Hints
	if c.Focus != "" {
		// Lead with the focus-state label so the current focus reads in text,
		// not only colour (keeps the degraded/ASCII theme usable, design §A.3).
		focus := c.styles.Role(theme.RoleSelected).Render("[" + c.Focus + "]")
		if hints != "" {
			hints = focus + " " + hints
		} else {
			hints = focus
		}
	}
	if c.Event != "" {
		hints = hints + "   [" + c.Event + "]"
	}
	role := c.StatusRole
	if role == "" {
		role = theme.RoleInfo
	}
	return c.status.View(f, hints, c.Status, role)
}
