package components

import (
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SearchBox wraps bubbles/textinput for the global `/` search and in-screen
// filters (design §3). When focused it captures the keyboard so single-letter
// nav is suppressed upstream (the shell checks Focused()).
type SearchBox struct {
	styles theme.Styles
	model  textinput.Model
}

// SearchSubmitted is emitted when the user presses Enter in a focused SearchBox.
type SearchSubmitted struct {
	Query string
}

// NewSearchBox builds a SearchBox with a placeholder.
func NewSearchBox(styles theme.Styles, placeholder string) SearchBox {
	m := textinput.New()
	m.Placeholder = placeholder
	return SearchBox{styles: styles, model: m}
}

// Focus / Blur manage keyboard capture.
func (s *SearchBox) Focus() tea.Cmd { return s.model.Focus() }
func (s *SearchBox) Blur()          { s.model.Blur() }

// Focused reports whether the input owns the keyboard.
func (s SearchBox) Focused() bool { return s.model.Focused() }

// Value returns the current query text.
func (s SearchBox) Value() string { return s.model.Value() }

// SetValue sets the query text.
func (s *SearchBox) SetValue(v string) { s.model.SetValue(v) }

// Init implements the sub-model contract.
func (s SearchBox) Init() tea.Cmd { return nil }

// Update forwards editing keys and emits SearchSubmitted on Enter.
func (s SearchBox) Update(msg tea.Msg) (SearchBox, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && s.model.Focused() {
		switch km.String() {
		case "enter":
			q := s.model.Value()
			return s, func() tea.Msg { return SearchSubmitted{Query: q} }
		case "esc":
			s.model.Blur()
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.model, cmd = s.model.Update(msg)
	return s, cmd
}

// View renders the input within the frame.
func (s SearchBox) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	s.model.Width = f.W - 2
	label := s.styles.Role(theme.RoleInfo).Render("/ ")
	return clampBlock(label+s.model.View(), f)
}
