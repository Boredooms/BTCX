package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GraphView renders a *schema.Subgraph fetched from the bounded graph.Service
// (design §9). It NEVER builds or traverses the graph itself. Two data-honesty
// rules are enforced here (design §9, acceptance §19.8/§19.16):
//
//  1. Only the center/subject node may show a numeric risk, cross-referenced at
//     render time from the InvestigationResult.Risk.Score the parent supplies.
//     Every non-subject node renders Neutral with NO number, because
//     graph.Service leaves GraphNode.Risk zero.
//  2. The engine's MaxNodes cap and the render cap are both shown in a badge,
//     and the badge turns Warning when the result is truncated at the cap.
type GraphView struct {
	styles theme.Styles

	sub        *schema.Subgraph
	center     string
	maxNodes   int  // engine cap (graph.Service.MaxNodes), shown in badge
	renderCap  int  // visible node budget (design §9)
	centerRisk *int // subject risk from InvestigationResult.Risk; nil if unknown
	selected   int  // index into sub.Nodes
}

// NewGraphView builds a GraphView. engineMaxNodes is graph.Service.MaxNodes
// (5000); renderCap is the visible-node budget (default 800 if <= 0).
func NewGraphView(styles theme.Styles, engineMaxNodes, renderCap int) GraphView {
	if renderCap <= 0 {
		renderCap = 800
	}
	return GraphView{styles: styles, maxNodes: engineMaxNodes, renderCap: renderCap}
}

// SetSubgraph loads a subgraph centered on center. centerRisk, when non-nil, is
// the subject's InvestigationResult.Risk.Score and is the ONLY numeric risk the
// view may display (and only on the center node).
func (g *GraphView) SetSubgraph(sub *schema.Subgraph, center string, centerRisk *int) {
	g.sub = sub
	g.center = center
	g.centerRisk = centerRisk
	g.selected = 0
}

// Subgraph returns the loaded subgraph (nil until SetSubgraph). Used by screens
// to re-apply a subgraph when the center-node risk arrives separately.
func (g GraphView) Subgraph() *schema.Subgraph { return g.sub }

// SelectedID returns the id of the currently selected node, or "" when the view
// holds no nodes. Used by the Graph screen's path mode to pick src/dst.
func (g GraphView) SelectedID() string {
	if g.sub == nil || g.selected < 0 || g.selected >= len(g.sub.Nodes) {
		return ""
	}
	return g.sub.Nodes[g.selected].ID
}

// nodeGlyph returns the per-type glyph (ASCII in degraded mode).
func (g GraphView) nodeGlyph(t schema.NodeType) string {
	if g.styles.Theme().Degraded {
		switch t {
		case schema.NodeWallet:
			return "W"
		case schema.NodeTransaction:
			return "T"
		case schema.NodeIP:
			return "I"
		case schema.NodeEntity:
			return "E"
		default:
			return "?"
		}
	}
	switch t {
	case schema.NodeWallet:
		return "◆"
	case schema.NodeTransaction:
		return "▣"
	case schema.NodeIP:
		return "◉"
	case schema.NodeEntity:
		return "⬢"
	default:
		return "·"
	}
}

// truncated reports whether the subgraph hit the engine cap.
func (g GraphView) truncated() bool {
	return g.sub != nil && g.maxNodes > 0 && len(g.sub.Nodes) >= g.maxNodes
}

// Badge returns the limit badge text. It reports the full subgraph size plus
// how many nodes the ego diagram actually draws (center + first-hop neighbours,
// capped), so a 600-node subgraph honestly reads "641 nodes · showing 1+16".
func (g GraphView) Badge() string {
	n := 0
	if g.sub != nil {
		n = len(g.sub.Nodes)
	}
	nbr := g.directNeighbourCount()
	shown := nbr
	if shown > maxDrawnNeighbours {
		shown = maxDrawnNeighbours
	}
	return fmt.Sprintf("%d nodes / %d cap · showing center + %d of %d neighbours",
		n, g.maxNodes, shown, nbr)
}

// directNeighbourCount counts distinct first-hop neighbours of the center in the
// subgraph (used by the badge to report how many were drawn vs hidden).
func (g GraphView) directNeighbourCount() int {
	if g.sub == nil {
		return 0
	}
	seen := map[string]bool{}
	for _, e := range g.sub.Edges {
		switch g.center {
		case e.From:
			seen[e.To] = true
		case e.To:
			seen[e.From] = true
		}
	}
	return len(seen)
}

// Init implements the sub-model contract.
func (g GraphView) Init() tea.Cmd { return nil }

// Update handles node selection (n/N) within the loaded node set.
func (g GraphView) Update(msg tea.Msg) (GraphView, tea.Cmd) {
	if g.sub == nil || len(g.sub.Nodes) == 0 {
		return g, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "n":
			if g.selected < len(g.sub.Nodes)-1 {
				g.selected++
			}
		case "N":
			if g.selected > 0 {
				g.selected--
			}
		}
	}
	return g, nil
}

// isCenter reports whether a node id is the subject/center node.
func (g GraphView) isCenter(id string) bool { return id == g.center }

// View renders the graph into the frame: a badge line, then a DRAWN node-link
// diagram (center node in the middle, neighbours placed on a ring with lines
// connecting them along the real subgraph edges), and a selection readout that
// obeys the center-only risk rule. Below a minimum drawable size it falls back
// to the compact node list so tiny panes still render honestly.
func (g GraphView) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	if g.sub == nil {
		return clampBlock(g.styles.Role(theme.RoleInfo).Render("querying…"), f)
	}
	if len(g.sub.Nodes) == 0 {
		return clampBlock(g.styles.Role(theme.RoleLabel).Render("no nodes in subgraph"), f)
	}

	badgeRole := theme.RoleInfo
	badge := g.Badge()
	if g.truncated() {
		badgeRole = theme.RoleWarning
		badge = fmt.Sprintf("%d nodes (truncated at engine cap %d) · showing center + %d",
			len(g.sub.Nodes), g.maxNodes, min(g.directNeighbourCount(), maxDrawnNeighbours))
	}

	var b strings.Builder
	b.WriteString(g.styles.Badge(badgeRole, badge))
	b.WriteString("\n")

	// Reserve: 1 badge row (already written) + 1 selection readout row.
	bodyH := f.H - 2
	if bodyH < 1 {
		bodyH = 1
	}
	// Draw the node-link diagram when the pane is big enough to place a ring;
	// otherwise fall back to the compact list.
	if f.W >= 32 && bodyH >= 7 {
		b.WriteString(g.renderDiagram(Frame{W: f.W, H: bodyH}))
	} else {
		b.WriteString(g.renderList(Frame{W: f.W, H: bodyH}))
	}
	b.WriteString("\n")
	b.WriteString(g.selectionReadout())
	return clampBlock(b.String(), f)
}

// renderList is the compact fallback: a selectable vertical node list (the
// original rendering), used when the pane is too small for the diagram.
func (g GraphView) renderList(f Frame) string {
	limit := len(g.sub.Nodes)
	if limit > g.renderCap {
		limit = g.renderCap
	}
	if limit > f.H {
		limit = f.H
	}
	var b strings.Builder
	for i := 0; i < limit; i++ {
		node := g.sub.Nodes[i]
		marker := "  "
		role := theme.RoleNeutral
		if i == g.selected {
			marker = "▸ "
			role = theme.RoleSelected
		}
		line := marker + g.styles.Role(role).Render(g.nodeGlyph(node.Type)+" "+shortID(node.ID))
		b.WriteString(clampLine(line, f.W))
		if i < limit-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// maxDrawnNeighbours bounds how many neighbours the ego diagram draws so a huge
// subgraph (hundreds of nodes) stays readable. Beyond this the badge reports the
// hidden count honestly ("showing N of M").
const maxDrawnNeighbours = 16

// renderDiagram draws a readable EGO diagram: the subject in the middle, its
// direct neighbours (nodes sharing an edge with the center) on a ring around it,
// and one clean radial line per center–neighbour edge. For a large subgraph it
// draws only the center + up to maxDrawnNeighbours neighbours (the meaningful
// first hop) instead of hundreds of overlapping nodes — the full counts stay in
// the badge. The selected node is highlighted; labels are shown only when the
// ring is uncrowded so they never collide into noise.
func (g GraphView) renderDiagram(f Frame) string {
	grid := newGraphGrid(f.W, f.H)

	drawn, order := g.egoLayout(f.W, f.H)
	// Labels are only legible when few nodes are drawn; above this, glyph-only.
	showLabels := len(order) <= 10

	// Edges: only those whose BOTH endpoints are drawn (clean radial links).
	for _, e := range g.sub.Edges {
		fromP, okF := drawn[e.From]
		toP, okT := drawn[e.To]
		if !okF || !okT {
			continue
		}
		grid.line(fromP.x, fromP.y, toP.x, toP.y)
	}

	// Node glyphs over the lines.
	for _, node := range order {
		p, ok := drawn[node.ID]
		if !ok {
			continue
		}
		gl := []rune(g.nodeGlyph(node.Type))
		role := theme.RoleNeutral
		switch {
		case g.isCenter(node.ID):
			role = theme.RoleWarning // the subject stands out
		case g.SelectedID() == node.ID:
			role = theme.RoleSelected
		}
		grid.setStyled(p.x, p.y, gl[0], g.styles.Role(role))

		// Label only when uncrowded, or always for the center/selected node.
		isFocus := g.isCenter(node.ID) || g.SelectedID() == node.ID
		if showLabels || isFocus {
			lbl := shortID(node.ID)
			for j, r := range []rune(lbl) {
				lx := p.x + 2 + j
				if lx >= f.W {
					break
				}
				lblRole := theme.RoleLabel
				if isFocus {
					lblRole = role
				}
				grid.setStyled(lx, p.y, r, g.styles.Role(lblRole))
			}
		}
	}
	return grid.render()
}

// ringPoint is a placed node coordinate in the grid.
type ringPoint struct{ x, y int }

// egoLayout chooses which nodes to draw (center + its direct neighbours, capped)
// and assigns each a non-overlapping grid coordinate: the center at the middle,
// the neighbours spaced evenly on an ellipse. It returns the placement map and
// the drawn-node slice in draw order (center first). Choosing the FIRST-HOP
// neighbourhood — not an arbitrary prefix of all nodes — is what keeps a
// 600-node subgraph legible: the diagram shows the subject's immediate
// relationships, and the badge reports how many were hidden.
func (g GraphView) egoLayout(w, h int) (map[string]ringPoint, []schema.GraphNode) {
	placed := make(map[string]ringPoint)
	if len(g.sub.Nodes) == 0 {
		return placed, nil
	}
	byID := make(map[string]schema.GraphNode, len(g.sub.Nodes))
	for _, n := range g.sub.Nodes {
		byID[n.ID] = n
	}
	center, ok := byID[g.center]
	if !ok {
		center = g.sub.Nodes[0]
	}

	// Direct neighbours of the center, de-duplicated, in stable edge order.
	seen := map[string]bool{center.ID: true}
	var neighbours []schema.GraphNode
	addNbr := func(id string) {
		if seen[id] {
			return
		}
		if n, ok := byID[id]; ok {
			seen[id] = true
			neighbours = append(neighbours, n)
		}
	}
	for _, e := range g.sub.Edges {
		if e.From == center.ID {
			addNbr(e.To)
		} else if e.To == center.ID {
			addNbr(e.From)
		}
	}
	// If the center had no incident edges in the subgraph (shouldn't happen for
	// a real center), fall back to the first nodes so something still draws.
	if len(neighbours) == 0 {
		for _, n := range g.sub.Nodes {
			if n.ID != center.ID {
				addNbr(n.ID)
			}
			if len(neighbours) >= maxDrawnNeighbours {
				break
			}
		}
	}
	cap := maxDrawnNeighbours
	if len(neighbours) > cap {
		neighbours = neighbours[:cap]
	}

	cx, cy := w/2, h/2
	placed[center.ID] = ringPoint{cx, cy}
	order := append([]schema.GraphNode{center}, neighbours...)

	n := len(neighbours)
	if n == 0 {
		return placed, order
	}
	rx := w/2 - 12
	if rx < 6 {
		rx = 6
	}
	ry := h/2 - 1
	if ry < 2 {
		ry = 2
	}
	occupied := map[[2]int]bool{{cx, cy}: true}
	for i, node := range neighbours {
		theta := 2*math.Pi*float64(i)/float64(n) - math.Pi/2 // start at top
		x := cx + int(float64(rx)*math.Cos(theta))
		y := cy + int(float64(ry)*math.Sin(theta))
		x = clampInt(x, 1, w-2)
		y = clampInt(y, 0, h-1)
		// Nudge off any already-occupied cell so glyphs never stack.
		for occupied[[2]int{x, y}] {
			y++
			if y > h-1 {
				y = 0
				x = clampInt(x+1, 1, w-2)
			}
		}
		occupied[[2]int{x, y}] = true
		placed[node.ID] = ringPoint{x, y}
	}
	return placed, order
}

// clampInt clamps v into [lo,hi].
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// selectionReadout renders the selected node line. Only the center node shows a
// numeric risk (joined from InvestigationResult); all others show "risk —".
func (g GraphView) selectionReadout() string {
	if g.selected < 0 || g.selected >= len(g.sub.Nodes) {
		return ""
	}
	node := g.sub.Nodes[g.selected]
	base := fmt.Sprintf("selected: %s %s  type %s  ",
		g.nodeGlyph(node.Type), shortID(node.ID), node.Type)
	if g.isCenter(node.ID) && g.centerRisk != nil {
		riskLine := fmt.Sprintf("risk %d (center node — from analysis)", *g.centerRisk)
		return g.styles.Role(theme.RoleValue).Render(base) +
			g.styles.Role(theme.RoleWarning).Render(riskLine)
	}
	return g.styles.Role(theme.RoleValue).Render(base) +
		g.styles.Role(theme.RoleLabel).Render("risk —")
}

// shortID truncates a long id for display.
func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:6] + "…" + id[len(id)-3:]
}

// --- graphGrid: a tiny character canvas for the node-link diagram ------------

// graphCell is one canvas cell: a rune plus an optional foreground style. A
// zero style means the dim edge style is applied at render time.
type graphCell struct {
	r      rune
	styled bool
	style  lipgloss.Style
}

// graphGrid is a fixed w×h character canvas. Edges are drawn as dim line glyphs;
// node glyphs are written with an explicit style and never overwritten by a
// later line (setStyled wins over line()).
type graphGrid struct {
	w, h  int
	cells []graphCell
	edge  lipgloss.Style
}

func newGraphGrid(w, h int) *graphGrid {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &graphGrid{w: w, h: h, cells: make([]graphCell, w*h),
		edge: lipgloss.NewStyle().Foreground(theme.Palette.Border)}
}

func (g *graphGrid) idx(x, y int) int { return y*g.w + x }

func (g *graphGrid) inBounds(x, y int) bool { return x >= 0 && x < g.w && y >= 0 && y < g.h }

// setStyled places a node glyph with an explicit style; it always wins over an
// edge glyph already in the cell.
func (g *graphGrid) setStyled(x, y int, r rune, style lipgloss.Style) {
	if !g.inBounds(x, y) {
		return
	}
	g.cells[g.idx(x, y)] = graphCell{r: r, styled: true, style: style}
}

// setEdge places a dim line glyph only if the cell is empty or already an edge
// (never over a node glyph).
func (g *graphGrid) setEdge(x, y int, r rune) {
	if !g.inBounds(x, y) {
		return
	}
	c := g.cells[g.idx(x, y)]
	if c.styled {
		return // don't paint a line over a node
	}
	g.cells[g.idx(x, y)] = graphCell{r: r}
}

// line draws a straight line between two points using Bresenham, choosing a
// box-drawing glyph by the dominant direction so the link reads as a line.
func (g *graphGrid) line(x0, y0, x1, y1 int) {
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	glyph := lineGlyph(x1-x0, y1-y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy
	x, y := x0, y0
	for {
		// Skip the two endpoints so node glyphs are not boxed in by line ends.
		if !(x == x0 && y == y0) && !(x == x1 && y == y1) {
			g.setEdge(x, y, glyph)
		}
		if x == x1 && y == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// lineGlyph picks a line character by the dominant slope direction.
func lineGlyph(dx, dy int) rune {
	adx, ady := abs(dx), abs(dy)
	if ady == 0 {
		return '─'
	}
	if adx == 0 {
		return '│'
	}
	if adx > 2*ady {
		return '─'
	}
	if ady > 2*adx {
		return '│'
	}
	// Diagonal: pick the slash matching the sign relationship.
	if (dx > 0) == (dy > 0) {
		return '╲'
	}
	return '╱'
}

// render materializes the grid into a newline-joined styled string. Empty cells
// are spaces; edge cells get the dim edge style; node cells carry their own.
func (g *graphGrid) render() string {
	var b strings.Builder
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			c := g.cells[g.idx(x, y)]
			switch {
			case c.r == 0:
				b.WriteByte(' ')
			case c.styled:
				b.WriteString(c.style.Render(string(c.r)))
			default:
				b.WriteString(g.edge.Render(string(c.r)))
			}
		}
		if y < g.h-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
