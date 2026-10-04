package screens

import (
	"fmt"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Detection is the Detection screen (design §2). It shows the pattern detectors
// and ML model outputs from the orchestrator's InvestigationResult
// (.Patterns / .Predictions). Pattern types are "-like" (never a crime claim,
// AGENTS §18); the screen renders them as detected signals with scores.
type Detection struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	detail  components.DetailPanel
	result  *schema.InvestigationResult
}

// NewDetection builds the Detection screen for the current subject.
func NewDetection(ctx *ScreenCtx) *Detection {
	return &Detection{
		ctx:     ctx,
		styles:  ctx.styles(),
		subject: ctx.Subject.ID,
		detail:  components.NewDetailPanel(ctx.styles()),
	}
}

// Init runs the orchestrator; patterns/predictions come from the result.
func (d *Detection) Init() tea.Cmd {
	if d.subject == "" {
		d.detail.SetError("no subject selected")
		return nil
	}
	if d.ctx.repo() == nil {
		d.detail.SetError(errNoCase.Error())
		return nil
	}
	d.detail.SetLoading()
	return analyzeWalletCmd(d.ctx, d.subject, true)
}

// Update folds in the analysis result.
func (d *Detection) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != d.subject {
			return d, nil
		}
		if r, ok := m.Payload.(*schema.InvestigationResult); ok {
			d.result = r
			d.detail.SetSections(d.sections())
		}
	case dataError:
		if m.Request == d.subject {
			d.detail.SetError(m.Err.Error())
		}
	case tea.KeyMsg:
		var cmd tea.Cmd
		d.detail, cmd = d.detail.Update(m)
		return d, cmd
	}
	return d, nil
}

func (d *Detection) sections() []components.Section {
	if d.result == nil {
		return nil
	}
	r := d.result
	note := components.Section{Title: "note", Body: "Pattern labels are \"-like\" " +
		"heuristics/model signals — a detected pattern is not proven criminal activity."}

	patRows := make([]components.Row, 0, len(r.Patterns))
	for _, p := range r.Patterns {
		patRows = append(patRows, components.Row{
			Label: string(p.Type),
			Value: fmt.Sprintf("score=%.2f conf=%.2f", p.Score, p.Confidence)})
	}
	if len(patRows) == 0 {
		patRows = []components.Row{{Label: "patterns", Value: "none detected"}}
	}

	predRows := make([]components.Row, 0, len(r.Predictions))
	for _, p := range r.Predictions {
		predRows = append(predRows, components.Row{
			Label: p.Model + " " + p.ModelVersion,
			Value: fmt.Sprintf("score=%.3f conf=%.3f", p.Score, p.Confidence)})
	}
	if len(predRows) == 0 {
		predRows = []components.Row{{Label: "predictions", Value: "no model output"}}
	}

	return []components.Section{
		note,
		{Title: "patterns", Rows: patRows},
		{Title: "model predictions", Rows: predRows},
	}
}

// View renders the detection detail.
func (d *Detection) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if d.subject == "" {
		return placeholderView(d.styles, "DETECTION", "no subject selected — analyze a wallet first", f)
	}
	title := d.styles.Title.Render("DETECTION") + " " + d.styles.Value.Render(shortID(d.subject))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	cw, ch := components.PanelInner(components.Frame{W: f.W, H: bodyH})
	panel := components.Panel(d.styles, "PATTERNS & MODEL SIGNALS",
		d.detail.View(components.Frame{W: cw, H: ch - 1}),
		components.Frame{W: f.W, H: bodyH})
	return clampBlockLocal(title+"\n"+panel, f)
}

// ShortHelp lists Detection's context keys.
func (d *Detection) ShortHelp() []key.Binding { return noBinding }
