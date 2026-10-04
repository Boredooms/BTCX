package tui

import (
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/screens"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// adapter.go bridges the screens package (which implements screens.Model) to the
// tui.Screen interface the registry uses. screens cannot implement tui.Screen
// directly because tui imports screens for its registry factories, and a
// reciprocal import would be a cycle. The adapter wraps a screens.Model, so each
// registry ScreenDef.New just constructs the real screen and wraps it — the
// registry structure (ids/keys/caps/help/Nav) is untouched (FEAT-002 contract).

// screenAdapter wraps a screens.Model as a tui.Screen. Update re-wraps the
// returned Model so the concrete screen type is preserved across turns.
type screenAdapter struct{ m screens.Model }

func adapt(m screens.Model) Screen { return screenAdapter{m: m} }

func (a screenAdapter) Init() tea.Cmd { return a.m.Init() }

func (a screenAdapter) Update(msg tea.Msg) (Screen, tea.Cmd) {
	next, cmd := a.m.Update(msg)
	return screenAdapter{m: next}, cmd
}

func (a screenAdapter) View(f Frame) string { return a.m.View(components.Frame(f)) }

func (a screenAdapter) ShortHelp() []key.Binding { return a.m.ShortHelp() }

// toScreenCtx adapts a tui.AppCtx into the screens.ScreenCtx a factory needs. It
// is nil-safe so a bare AppCtx (tests) still produces a renderable screen.
func toScreenCtx(ctx *AppCtx) *screens.ScreenCtx {
	if ctx == nil {
		return &screens.ScreenCtx{}
	}
	sc := &screens.ScreenCtx{
		App:        ctx.App,
		Styles:     ctx.Styles,
		Ctx:        ctx.Program,
		Compact:    ctx.Breakpoint == BreakpointCompact,
		Subject:    screens.Subject{ID: ctx.Subject.ID, Kind: string(ctx.Subject.Kind)},
		Summarizer: ctx.Summarizer,
		Acquire:    ctx.Acquire,
	}
	if ctx.App != nil && ctx.App.Engine != nil {
		sc.Cfg = ctx.App.Engine.Config()
	}
	return sc
}
