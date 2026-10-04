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

// Graph is the Graph exploration screen (design §2, §9). It renders a bounded
// *schema.Subgraph fetched from graph.Service (MaxNodes=5000) via the
// cancellable subgraphCmd — it never builds or traverses the graph itself. The
// depth keymap ([ / ]) re-queries at a new depth as a fresh cancellable command,
// superseding the previous query. Path mode (p) picks a src then a dst and
// highlights the graph.Service.Path result. The GraphView enforces the data
// honesty rules: only the center node shows a numeric risk (joined from the
// subject's InvestigationResult.Risk), and the engine cap is shown with a
// truncation marker.
type Graph struct {
	ctx    *ScreenCtx
	styles theme.Styles

	center string
	depth  int
	view   components.GraphView

	centerRisk *int

	// pathMode state: 0 = off, 1 = awaiting src, 2 = awaiting dst.
	pathStage int
	pathSrc   string
	pathIDs   []string
	status    string
	loadErr   error
}

// engineMaxNodes mirrors graph.Service.MaxNodes (5000). The GraphView shows it
// as the engine cap in its limit badge (design §9).
const engineMaxNodes = 5000

// defaultDepth is the initial subgraph radius; graph.Service.MaxNodes caps it.
const defaultDepth = 2

// NewGraph builds the Graph screen centered on the current subject.
func NewGraph(ctx *ScreenCtx) *Graph {
	gv := components.NewGraphView(ctx.styles(), engineMaxNodes, 800)
	return &Graph{
		ctx:    ctx,
		styles: ctx.styles(),
		center: ctx.Subject.ID,
		depth:  defaultDepth,
		view:   gv,
	}
}

// Init fetches the initial subgraph plus the subject's analysis (for the
// center-node risk join). Both are cancellable commands.
func (g *Graph) Init() tea.Cmd {
	if g.center == "" {
		g.status = "no subject — select a wallet/tx to center the graph"
		return nil
	}
	if g.ctx.repo() == nil {
		g.loadErr = errNoCase
		return nil
	}
	g.status = fmt.Sprintf("querying subgraph depth %d…", g.depth)
	return tea.Batch(
		subgraphCmd(g.ctx, g.center, g.depth),
		analyzeWalletCmd(g.ctx, g.center, true),
	)
}

// Update handles the graph keymap and folds in query results.
func (g *Graph) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		return g, g.applyLoaded(m)
	case dataError:
		g.loadErr = m.Err
		g.status = "error: " + m.Err.Error()
		return g, nil
	case tea.KeyMsg:
		return g.handleKey(m)
	}
	return g, nil
}

func (g *Graph) applyLoaded(m dataLoaded) tea.Cmd {
	switch p := m.Payload.(type) {
	case *schema.Subgraph:
		if m.Request == g.center {
			g.view.SetSubgraph(p, g.center, g.centerRiskPtr())
			g.status = ""
		}
	case *schema.InvestigationResult:
		if m.Request == g.center {
			score := p.Risk.Score
			g.centerRisk = &score
			g.view.SetSubgraph(g.currentSub(), g.center, g.centerRisk)
		}
	case pathResult:
		g.pathIDs = p.IDs
		if len(p.IDs) == 0 {
			g.status = fmt.Sprintf("no path %s → %s", shortID(p.Src), shortID(p.Dst))
		} else {
			g.status = fmt.Sprintf("path %s → %s: %d hops", shortID(p.Src), shortID(p.Dst), len(p.IDs)-1)
		}
	}
	return nil
}

func (g *Graph) handleKey(m tea.KeyMsg) (Model, tea.Cmd) {
	switch m.String() {
	case "[":
		if g.depth > 0 {
			g.depth--
			g.status = fmt.Sprintf("querying subgraph depth %d…", g.depth)
			return g, subgraphCmd(g.ctx, g.center, g.depth) // supersedes previous
		}
	case "]":
		g.depth++
		g.status = fmt.Sprintf("querying subgraph depth %d…", g.depth)
		return g, subgraphCmd(g.ctx, g.center, g.depth)
	case "p":
		g.pathStage = 1
		g.pathSrc = ""
		g.status = "path mode: press n/N to select src node, enter to pick"
		return g, nil
	case "enter":
		if g.pathStage == 1 {
			g.pathSrc = g.selectedID()
			g.pathStage = 2
			g.status = "path mode: select dst node, enter to run"
			return g, nil
		}
		if g.pathStage == 2 {
			dst := g.selectedID()
			g.pathStage = 0
			g.status = fmt.Sprintf("resolving path %s → %s…", shortID(g.pathSrc), shortID(dst))
			return g, pathCmd(g.ctx, g.pathSrc, dst)
		}
	case "0":
		g.depth = defaultDepth
		g.pathStage = 0
		g.pathIDs = nil
		g.status = fmt.Sprintf("reset: querying depth %d…", g.depth)
		return g, subgraphCmd(g.ctx, g.center, g.depth)
	}
	var cmd tea.Cmd
	g.view, cmd = g.view.Update(m)
	return g, cmd
}

// selectedID returns the currently selected node id from the view, if any.
func (g *Graph) selectedID() string { return g.view.SelectedID() }

// currentSub returns the subgraph currently held by the view.
func (g *Graph) currentSub() *schema.Subgraph { return g.view.Subgraph() }

// centerRiskPtr is the subject's risk score (joined from analysis), or nil.
func (g *Graph) centerRiskPtr() *int { return g.centerRisk }

// View renders the graph with the limit badge, status, and path hint.
func (g *Graph) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if g.center == "" {
		return placeholderView(g.styles, "GRAPH", "no subject selected — center the graph from a wallet/tx", f)
	}
	var b strings.Builder
	header := g.styles.Title.Render("GRAPH") + " " +
		g.styles.Value.Render("center "+shortID(g.center)) + " " +
		g.styles.Label.Render(fmt.Sprintf("depth %d", g.depth))
	b.WriteString(header)
	b.WriteString("\n")
	statusRows := 0
	if g.status != "" {
		statusRows = 1
	}
	bodyH := f.H - 1 - statusRows
	if bodyH < 1 {
		bodyH = 1
	}
	// Shared cockpit chrome: the node graph lives in a titled bordered panel.
	b.WriteString(components.Panel(g.styles, "SUBGRAPH — BOUNDED NODE SET",
		g.view.View(components.Frame{W: f.W - 2, H: paneInner(bodyH)}),
		components.Frame{W: f.W, H: bodyH}))
	if g.status != "" {
		b.WriteString("\n")
		b.WriteString(g.styles.Role(theme.RoleInfo).Render(g.status))
	}
	return clampBlockLocal(b.String(), f)
}

// ShortHelp lists Graph's context keys.
func (g *Graph) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("["), key.WithHelp("[", "depth-")),
		key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "depth+")),
		key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n/N", "select")),
		key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "path mode")),
		key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset")),
	}
}
