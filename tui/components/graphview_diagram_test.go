package components

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/charmbracelet/lipgloss"
)

// starSubgraph builds a center wallet linked to three neighbours by real edges,
// mirroring the tx-e2e fan-out (1 input -> tx -> 2 outputs).
func starSubgraph() *schema.Subgraph {
	return &schema.Subgraph{
		Center: "CENTER",
		Depth:  2,
		Nodes: []schema.GraphNode{
			{ID: "CENTER", Type: schema.NodeWallet},
			{ID: "TXAAA", Type: schema.NodeTransaction},
			{ID: "OUT111", Type: schema.NodeWallet},
			{ID: "OUT222", Type: schema.NodeWallet},
		},
		Edges: []schema.GraphEdge{
			{ID: "e1", Type: schema.EdgeInputTo, From: "CENTER", To: "TXAAA"},
			{ID: "e2", Type: schema.EdgeOutputTo, From: "TXAAA", To: "OUT111"},
			{ID: "e3", Type: schema.EdgeOutputTo, From: "TXAAA", To: "OUT222"},
			{ID: "e4", Type: schema.EdgeSentTo, From: "CENTER", To: "OUT111"},
		},
	}
}

// TestGraphViewDrawsLines asserts the diagram render actually draws connecting
// lines (box-drawing glyphs) between nodes, not just a node list, at a size big
// enough for the ring layout, and never overflows the frame.
func TestGraphViewDrawsLines(t *testing.T) {
	gv := NewGraphView(styles(), 5000, 800)
	r := 72
	gv.SetSubgraph(starSubgraph(), "CENTER", &r)

	f := Frame{W: 80, H: 24}
	out := gv.View(f)

	// Line glyphs prove edges were drawn.
	if !strings.ContainsAny(out, "─│╲╱") {
		t.Errorf("diagram should draw connecting lines; got:\n%s", out)
	}
	// Node glyphs for wallet + transaction present.
	if !strings.ContainsAny(out, "◆▣") {
		t.Errorf("diagram should draw node glyphs; got:\n%s", out)
	}
	// No row exceeds the frame width (lipgloss width accounting).
	for i, ln := range strings.Split(out, "\n") {
		if lipgloss.Width(ln) > f.W {
			t.Errorf("row %d width %d exceeds %d: %q", i, lipgloss.Width(ln), f.W, ln)
		}
	}
}

// TestGraphViewListFallbackWhenTiny asserts a pane too small for the ring falls
// back to the compact node list (still renders every selectable node, no lines
// required) without panicking or overflowing.
func TestGraphViewListFallbackWhenTiny(t *testing.T) {
	gv := NewGraphView(styles(), 5000, 800)
	r := 72
	gv.SetSubgraph(starSubgraph(), "CENTER", &r)
	out := gv.View(Frame{W: 24, H: 6})
	if strings.TrimSpace(out) == "" {
		t.Fatal("tiny pane should still render the fallback list")
	}
	if !strings.Contains(out, "CENTER") {
		t.Errorf("fallback list should show node ids; got:\n%s", out)
	}
}

// bigSubgraph builds a center with `n` neighbours (via edges) to mimic a large
// real subgraph where only the first-hop neighbourhood should be drawn.
func bigSubgraph(n int) *schema.Subgraph {
	nodes := []schema.GraphNode{{ID: "CENTER", Type: schema.NodeWallet}}
	var edges []schema.GraphEdge
	for i := 0; i < n; i++ {
		id := "N" + itoaTest(i)
		nodes = append(nodes, schema.GraphNode{ID: id, Type: schema.NodeWallet})
		edges = append(edges, schema.GraphEdge{ID: "e" + itoaTest(i),
			Type: schema.EdgeSentTo, From: "CENTER", To: id})
	}
	return &schema.Subgraph{Center: "CENTER", Depth: 2, Nodes: nodes, Edges: edges}
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// TestGraphViewLargeIsReadable asserts a big subgraph (640 neighbours) draws
// only the capped first-hop neighbourhood (no overlap, no overflow) and the
// badge honestly reports how many of the neighbours are shown.
func TestGraphViewLargeIsReadable(t *testing.T) {
	gv := NewGraphView(styles(), 5000, 800)
	r := 85
	gv.SetSubgraph(bigSubgraph(640), "CENTER", &r)
	f := Frame{W: 120, H: 40}
	out := gv.View(f)

	// Badge reports the drawn cap honestly.
	if !strings.Contains(out, "showing center + 16 of 640 neighbours") {
		t.Errorf("badge should report 16 of 640 drawn; got first line:\n%s",
			strings.SplitN(out, "\n", 2)[0])
	}
	// No row overflows the frame despite 640 nodes in the data.
	for i, ln := range strings.Split(out, "\n") {
		if lipgloss.Width(ln) > f.W {
			t.Errorf("row %d width %d exceeds %d", i, lipgloss.Width(ln), f.W)
		}
	}
	// The diagram still draws lines + the center.
	if !strings.ContainsAny(out, "─│╲╱") {
		t.Error("large graph should still draw connecting lines")
	}
	if !strings.Contains(out, "CENTER") {
		t.Error("large graph should label the center node")
	}
}

// TestGraphViewLargePreview prints the capped ego diagram for a big subgraph.
func TestGraphViewLargePreview(t *testing.T) {
	gv := NewGraphView(styles(), 5000, 800)
	r := 85
	gv.SetSubgraph(bigSubgraph(640), "CENTER", &r)
	t.Logf("\n%s", gv.View(Frame{W: 110, H: 30}))
}

// TestGraphViewPreview prints the drawn diagram so a human can eyeball it:
//
//	go test ./tui/components -run TestGraphViewPreview -v
func TestGraphViewPreview(t *testing.T) {
	gv := NewGraphView(styles(), 5000, 800)
	r := 72
	gv.SetSubgraph(starSubgraph(), "CENTER", &r)
	t.Logf("\n%s", gv.View(Frame{W: 90, H: 26}))
}
