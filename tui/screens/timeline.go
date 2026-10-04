package screens

import (
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Timeline is the contextual Timeline screen (design §2, Nav=false). It renders
// the chronological events for the selected subject from reporting.Service
// Timeline, built off a fresh analysis snapshot. The TimelineView shows one
// glyph per event Kind and a severity ONLY on alert-kind rows (design §3) — the
// reporting TimelineEvent carries no per-event severity.
type Timeline struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	view    components.TimelineView
	err     error
	loading bool
}

// NewTimeline builds the Timeline screen for the current subject.
func NewTimeline(ctx *ScreenCtx) *Timeline {
	return &Timeline{
		ctx:     ctx,
		styles:  ctx.styles(),
		subject: ctx.Subject.ID,
		view:    components.NewTimelineView(ctx.styles()),
	}
}

// Init analyzes the subject, then builds its timeline. The two-step command
// (analyze -> build snapshot -> timeline) runs under one cancellable context.
func (t *Timeline) Init() tea.Cmd {
	if t.subject == "" {
		t.err = errNoSubject
		return nil
	}
	if t.ctx.repo() == nil {
		t.err = errNoCase
		return nil
	}
	t.loading = true
	return analyzeThenTimelineCmd(t.ctx, t.subject)
}

// Update folds in the timeline result.
func (t *Timeline) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != t.subject {
			return t, nil
		}
		if tr, ok := m.Payload.(timelineResult); ok {
			t.loading = false
			t.view.SetEvents(tr.Events, tr.Alerts)
		}
	case dataError:
		if m.Request == t.subject {
			t.loading = false
			t.err = m.Err
		}
	case tea.KeyMsg:
		var cmd tea.Cmd
		t.view, cmd = t.view.Update(m)
		return t, cmd
	}
	return t, nil
}

// View renders the timeline.
func (t *Timeline) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if t.subject == "" {
		return placeholderView(t.styles, "TIMELINE", "no subject selected — open a wallet, then press t", f)
	}
	title := t.styles.Title.Render("TIMELINE") + " " + t.styles.Value.Render(shortID(t.subject))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	if t.err != nil {
		return clampBlockLocal(title+"\n"+t.styles.Role(theme.RoleCritical).Render("error: "+t.err.Error()), f)
	}
	if t.loading {
		return clampBlockLocal(title+"\n"+t.styles.Role(theme.RoleInfo).Render("building timeline…"), f)
	}
	return clampBlockLocal(title+"\n"+t.view.View(components.Frame{W: f.W, H: bodyH}), f)
}

// ShortHelp lists Timeline's context keys.
func (t *Timeline) ShortHelp() []key.Binding { return noBinding }
