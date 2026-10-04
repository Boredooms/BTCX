package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// stubScreen is a placeholder Screen used to register all 15 screens in the
// router before their real bodies land (that is FEAT-004). It holds NO service
// logic — it only renders its registry title inside the allocated Frame so the
// shell, navigation, and tests have a reachable, non-panicking target for every
// registered ScreenID. Each stub is replaced by its real screen in a later
// feature without touching the registry wiring.
type stubScreen struct {
	id    ScreenID
	title string
	ctx   *AppCtx
}

func stubFactory(id ScreenID) func(*AppCtx) Screen {
	return func(ctx *AppCtx) Screen {
		title := string(id)
		if def := Lookup(id); def != nil {
			title = def.Title
		}
		return &stubScreen{id: id, title: title, ctx: ctx}
	}
}

func (s *stubScreen) Init() tea.Cmd { return nil }

func (s *stubScreen) Update(tea.Msg) (Screen, tea.Cmd) { return s, nil }

func (s *stubScreen) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	// Presentation only: render the title clamped to the frame width.
	line := s.title
	if len(line) > f.W {
		line = line[:f.W]
	}
	return line
}

func (s *stubScreen) ShortHelp() []key.Binding { return nil }
