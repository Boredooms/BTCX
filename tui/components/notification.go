package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
)

// Toast is one transient notification. Level drives the color role; it is never
// faked and never implies forensic meaning (design §11.1).
type Toast struct {
	Level string // info / success / warning / error
	Text  string
}

// NotificationLayer renders a capped stack of transient toasts as an overlay.
// It auto-expires entries (the parent trims by age) and never steals focus.
type NotificationLayer struct {
	styles theme.Styles
	toasts []Toast
}

// NewNotificationLayer builds an empty layer.
func NewNotificationLayer(styles theme.Styles) NotificationLayer {
	return NotificationLayer{styles: styles}
}

const maxToasts = 5

// SetToasts replaces the toast stack, capping at 5 (newest kept).
func (n *NotificationLayer) SetToasts(ts []Toast) {
	if len(ts) > maxToasts {
		ts = ts[len(ts)-maxToasts:]
	}
	n.toasts = ts
}

func toastRole(level string) theme.Role {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "success":
		return theme.RoleHealthy
	case "warning":
		return theme.RoleWarning
	case "error":
		return theme.RoleCritical
	default:
		return theme.RoleInfo
	}
}

// View renders the toast stack top-right within the frame.
func (n NotificationLayer) View(f Frame) string {
	if f.Empty() || len(n.toasts) == 0 {
		return ""
	}
	var lines []string
	for _, t := range n.toasts {
		glyph := "●"
		line := n.styles.Role(toastRole(t.Level)).Render(glyph+" ") +
			n.styles.Role(theme.RoleValue).Render(t.Text)
		lines = append(lines, clampLine(line, f.W))
	}
	return clampBlock(joinLines(lines), f)
}
