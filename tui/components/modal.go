package components

import (
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalKind classifies a modal so the parent can act on the result.
type ModalKind string

const (
	// ModalConfirm is a yes/no confirmation (e.g. "start monitor?" network gate).
	ModalConfirm ModalKind = "confirm"
	// ModalError expands a one-line error into its full text.
	ModalError ModalKind = "error"
	// ModalInfo is a dismissable informational dialog.
	ModalInfo ModalKind = "info"
)

// Modal is one overlay dialog. All overlays route through the single ModalStack
// so only one modal shows at a time and Esc-closing is uniform (design §3).
type Modal struct {
	Kind  ModalKind
	Title string
	Body  string
}

// ModalResult is emitted when a confirm modal is answered.
type ModalResult struct {
	Kind      ModalKind
	Confirmed bool
}

// ModalStack is the single modal subsystem. It holds at most one visible modal;
// pushing a second stacks it behind so Esc reveals the previous one.
type ModalStack struct {
	styles theme.Styles
	stack  []Modal
}

// NewModalStack builds an empty modal stack.
func NewModalStack(styles theme.Styles) ModalStack { return ModalStack{styles: styles} }

// Push adds a modal to the top of the stack.
func (s *ModalStack) Push(m Modal) { s.stack = append(s.stack, m) }

// Active reports whether a modal is currently shown.
func (s ModalStack) Active() bool { return len(s.stack) > 0 }

// Top returns the visible modal, or false if none.
func (s ModalStack) Top() (Modal, bool) {
	if len(s.stack) == 0 {
		return Modal{}, false
	}
	return s.stack[len(s.stack)-1], true
}

// Update handles Esc (close top) and y/n for confirm modals. It returns the
// (possibly shrunk) stack and any ModalResult command.
func (s ModalStack) Update(msg tea.Msg) (ModalStack, tea.Cmd) {
	if len(s.stack) == 0 {
		return s, nil
	}
	top := s.stack[len(s.stack)-1]
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			s.stack = s.stack[:len(s.stack)-1]
			if top.Kind == ModalConfirm {
				return s, func() tea.Msg { return ModalResult{Kind: top.Kind, Confirmed: false} }
			}
			return s, nil
		case "y", "enter":
			if top.Kind == ModalConfirm {
				s.stack = s.stack[:len(s.stack)-1]
				return s, func() tea.Msg { return ModalResult{Kind: top.Kind, Confirmed: true} }
			}
			s.stack = s.stack[:len(s.stack)-1]
			return s, nil
		case "n":
			if top.Kind == ModalConfirm {
				s.stack = s.stack[:len(s.stack)-1]
				return s, func() tea.Msg { return ModalResult{Kind: top.Kind, Confirmed: false} }
			}
		}
	}
	return s, nil
}

// View renders the top modal centered within the frame as an overlay box.
func (s ModalStack) View(f Frame) string {
	top, ok := s.Top()
	if !ok || f.Empty() {
		return ""
	}
	role := theme.RoleInfo
	if top.Kind == ModalError {
		role = theme.RoleCritical
	}
	boxW := min(f.W-4, 60)
	if boxW < 10 {
		boxW = f.W
	}
	content := s.styles.Role(theme.RoleTitle).Render(top.Title) + "\n\n" +
		s.styles.Role(role).Render(top.Body)
	if top.Kind == ModalConfirm {
		content += "\n\n" + s.styles.Role(theme.RoleLabel).Render("[y] confirm  [n/esc] cancel")
	} else {
		content += "\n\n" + s.styles.Role(theme.RoleLabel).Render("[esc] close")
	}
	box := s.styles.Panel.Width(boxW).Render(content)
	return lipgloss.Place(f.W, f.H, lipgloss.Center, lipgloss.Center, box)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
