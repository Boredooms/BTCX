package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Command is one entry in the palette: a screen jump or a verb. The parent
// builds the list from the registry plus a fixed verb list and routes the
// chosen ID (design §3). The palette knows nothing about the registry itself.
type Command struct {
	ID    string
	Label string
}

// CommandChosen is emitted when the user selects a palette entry.
type CommandChosen struct {
	ID string
}

// CommandPalette is a fuzzy launcher over a command list, opened with `:` or
// Ctrl+P. It filters by substring on the input text and highlights the current
// candidate.
type CommandPalette struct {
	styles  theme.Styles
	input   textinput.Model
	all     []Command
	matches []Command
	cursor  int
	open    bool
}

// NewCommandPalette builds a palette over the given command set.
func NewCommandPalette(styles theme.Styles, cmds []Command) CommandPalette {
	in := textinput.New()
	in.Placeholder = "type a command…"
	p := CommandPalette{styles: styles, input: in, all: cmds}
	p.filter()
	return p
}

// Open shows the palette and focuses the input.
func (p *CommandPalette) Open() tea.Cmd {
	p.open = true
	p.input.SetValue("")
	p.filter()
	return p.input.Focus()
}

// Close hides the palette.
func (p *CommandPalette) Close() {
	p.open = false
	p.input.Blur()
}

// IsOpen reports whether the palette is shown.
func (p CommandPalette) IsOpen() bool { return p.open }

// Focused reports whether the palette input owns the keyboard.
func (p CommandPalette) Focused() bool { return p.input.Focused() }

func (p *CommandPalette) filter() {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		p.matches = p.all
	} else {
		var m []Command
		for _, c := range p.all {
			if strings.Contains(strings.ToLower(c.Label), q) || strings.Contains(strings.ToLower(c.ID), q) {
				m = append(m, c)
			}
		}
		p.matches = m
	}
	if p.cursor >= len(p.matches) {
		p.cursor = 0
	}
}

// Init implements the sub-model contract.
func (p CommandPalette) Init() tea.Cmd { return nil }

// Update handles navigation and selection within the palette.
func (p CommandPalette) Update(msg tea.Msg) (CommandPalette, tea.Cmd) {
	if !p.open {
		return p, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			p.Close()
			return p, nil
		case "up", "ctrl+k":
			if p.cursor > 0 {
				p.cursor--
			}
			return p, nil
		case "down", "ctrl+j":
			if p.cursor < len(p.matches)-1 {
				p.cursor++
			}
			return p, nil
		case "enter":
			if p.cursor >= 0 && p.cursor < len(p.matches) {
				id := p.matches[p.cursor].ID
				p.Close()
				return p, func() tea.Msg { return CommandChosen{ID: id} }
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.filter()
	return p, cmd
}

// View renders the palette centered as an overlay within the frame.
func (p CommandPalette) View(f Frame) string {
	if !p.open || f.Empty() {
		return ""
	}
	boxW := min(f.W-4, 60)
	if boxW < 10 {
		boxW = f.W
	}
	var b strings.Builder
	b.WriteString(p.styles.Role(theme.RoleTitle).Render("Command Palette"))
	b.WriteString("\n")
	b.WriteString(p.input.View())
	b.WriteString("\n")
	maxRows := min(len(p.matches), max(f.H-6, 1))
	for i := 0; i < maxRows; i++ {
		c := p.matches[i]
		role := theme.RoleValue
		prefix := "  "
		if i == p.cursor {
			role = theme.RoleSelected
			prefix = "▸ "
		}
		b.WriteString(prefix + p.styles.Role(role).Render(c.Label) + "\n")
	}
	box := p.styles.Panel.Width(boxW).Render(b.String())
	return lipgloss.Place(f.W, f.H, lipgloss.Center, lipgloss.Center, box)
}
