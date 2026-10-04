package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Entity is the Entity Cluster screen (design §2). It presents the inferred
// clusters for the subject taken from the orchestrator's InvestigationResult
// (.Clusters, produced by the deterministic common-input heuristic). It is
// explicit that a cluster is an INFERRED relationship, never proof of real
// ownership (AGENTS §18).
type Entity struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	detail  components.DetailPanel
	result  *schema.InvestigationResult
}

// NewEntity builds the Entity screen for the current subject.
func NewEntity(ctx *ScreenCtx) *Entity {
	return &Entity{
		ctx:     ctx,
		styles:  ctx.styles(),
		subject: ctx.Subject.ID,
		detail:  components.NewDetailPanel(ctx.styles()),
	}
}

// Init runs the orchestrator; clusters come from the InvestigationResult.
func (e *Entity) Init() tea.Cmd {
	if e.subject == "" {
		e.detail.SetError("no subject selected")
		return nil
	}
	if e.ctx.repo() == nil {
		e.detail.SetError(errNoCase.Error())
		return nil
	}
	e.detail.SetLoading()
	return analyzeWalletCmd(e.ctx, e.subject, true)
}

// Update folds in the analysis result and extracts its clusters.
func (e *Entity) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != e.subject {
			return e, nil
		}
		if r, ok := m.Payload.(*schema.InvestigationResult); ok {
			e.result = r
			e.detail.SetSections(e.clusterSections())
		}
	case dataError:
		if m.Request == e.subject {
			e.detail.SetError(m.Err.Error())
		}
	case tea.KeyMsg:
		if m.String() == "g" && e.subject != "" {
			id := e.subject
			return e, func() tea.Msg {
				return NavScreen{Screen: "graph", Subject: Subject{ID: id, Kind: SubjectWallet}}
			}
		}
		var cmd tea.Cmd
		e.detail, cmd = e.detail.Update(m)
		return e, cmd
	}
	return e, nil
}

func (e *Entity) clusterSections() []components.Section {
	if e.result == nil {
		return nil
	}
	note := components.Section{Title: "note", Body: "A cluster is an INFERRED " +
		"relationship from shared inputs — it is not proof of common ownership."}
	if len(e.result.Clusters) == 0 {
		return []components.Section{note, {Title: "clusters", Rows: []components.Row{
			{Label: "result", Value: "no multi-member cluster for this subject"}}}}
	}
	secs := []components.Section{note}
	for _, c := range e.result.Clusters {
		rows := []components.Row{
			{Label: "cluster id", Value: c.ID},
			{Label: "confidence", Value: fmt.Sprintf("%.2f", c.Confidence)},
			{Label: "members", Value: fmt.Sprintf("%d", len(c.Members))},
			{Label: "basis", Value: strings.Join(c.Basis, ", ")},
		}
		for i, mbr := range c.Members {
			rows = append(rows, components.Row{Label: fmt.Sprintf("member %d", i+1), Value: shortID(mbr)})
		}
		secs = append(secs, components.Section{Title: "cluster " + shortID(c.ID), Rows: rows})
	}
	return secs
}

// View renders the entity-cluster detail.
func (e *Entity) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if e.subject == "" {
		return placeholderView(e.styles, "ENTITY CLUSTER", "no subject selected — analyze a wallet first", f)
	}
	title := e.styles.Title.Render("ENTITY CLUSTER") + " " + e.styles.Value.Render(shortID(e.subject))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	cw, ch := components.PanelInner(components.Frame{W: f.W, H: bodyH})
	panel := components.Panel(e.styles, "INFERRED CLUSTERS",
		e.detail.View(components.Frame{W: cw, H: ch - 1}),
		components.Frame{W: f.W, H: bodyH})
	return clampBlockLocal(title+"\n"+panel, f)
}

// ShortHelp lists Entity's context keys.
func (e *Entity) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	}
}
