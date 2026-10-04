package components

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

// acceptanceSizes is the planner's required render set (design §6, §19.4).
var acceptanceSizes = [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}}

// assertNoOverflow verifies a rendered block fits within the frame: no line is
// wider than f.W cells and the block has at most f.H lines (design §6).
func assertNoOverflow(t *testing.T, name string, out string, f Frame) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > f.H {
		t.Errorf("%s: %d lines exceeds frame height %d", name, len(lines), f.H)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > f.W {
			t.Errorf("%s: line %d width %d exceeds frame width %d", name, i, w, f.W)
		}
	}
}

func styles() theme.Styles { return theme.Build(theme.Default()) }

// renderers wraps each component's View in a uniform closure for the matrix.
func renderers(s theme.Styles) map[string]func(Frame) string {
	tbl := NewTable(s, []table.Column{{Title: "a", Width: 6}, {Title: "b", Width: 6}})
	tbl.SetRows(
		[]table.Row{{"r1a", "r1b"}, {"r2a", "r2b"}},
		[]string{"id1", "id2"}, []string{"wallet", "tx"},
	)

	dp := NewDetailPanel(s)
	dp.SetSections([]Section{{Title: "risk", Rows: []Row{{Label: "score", Value: "72/100"}}, Body: "signals follow"}})

	gv := NewGraphView(s, 5000, 800)
	risk := 72
	gv.SetSubgraph(sampleSubgraph(), "bc1qcenter", &risk)

	mv := NewMapView(s)
	mv.SetObservations(sampleObs())

	tv := NewTimelineView(s)
	tv.SetEvents(sampleEvents(), sampleAlerts())

	al := NewAlertList(s)
	al.SetAlerts(sampleAlerts())

	ms := NewMonitorStream(s)
	ms.SetLive(true)
	ms.Append(StreamLine{Timestamp: "14:03:10", Kind: "event", Text: "tx 9ab3 confirmed"})
	ms.Append(StreamLine{Timestamp: "14:03:10", Kind: "alert", Text: "structuring_like", Severity: "CRITICAL"})

	modal := NewModalStack(s)
	modal.Push(Modal{Kind: ModalConfirm, Title: "Start monitor?", Body: "This opens a network session."})

	pal := NewCommandPalette(s, []Command{{ID: "home", Label: "Dashboard"}, {ID: "analyze", Label: "Analyze wallet"}})
	_ = pal.Open()

	sb := NewSearchBox(s, "address / txid / ip")

	nl := NewNotificationLayer(s)
	nl.SetToasts([]Toast{{Level: "success", Text: "sync ok"}, {Level: "error", Text: "model missing"}})

	ho := NewHelpOverlay(s, [][]key.Binding{{
		key.NewBinding(key.WithHelp("↑/k", "up")),
		key.NewBinding(key.WithHelp("q", "quit")),
	}})

	nav := NewSideNav(s)
	nav.SetItems([]NavItem{{Key: "1", Title: "Dashboard", Active: true}, {Key: "2", Title: "Search"}})

	top := NewTopBar(s)
	top.Case = "investigation-01"
	top.Network = "DISCONNECTED"
	top.Acquisition = "PAUSED"
	top.Models = "LOADED"
	top.DB = "LOCAL"
	top.Provider = "fake"
	top.GeoIP = "v2024.11"
	top.Clock = "14:03:22"
	top.Subject = "bc1qsubjectlong"

	ctx := NewContextBar(s)
	ctx.Hints = "↑↓ move · enter open · / search · q quit"
	ctx.Event = "last: sync ok · 0 err"
	ctx.Status = "READY"

	sv := SplitView{Ratio: 0.5}

	return map[string]func(Frame) string{
		"table":         tbl.View,
		"detailpanel":   dp.View,
		"graphview":     gv.View,
		"mapview":       mv.View,
		"timelineview":  tv.View,
		"alertlist":     al.View,
		"monitorstream": ms.View,
		"modal":         modal.View,
		"palette":       pal.View,
		"searchbox":     sb.View,
		"notification":  nl.View,
		"helpoverlay":   ho.View,
		"sidenav":       nav.View,
		"topbar":        func(f Frame) string { return top.View(f, "Dashboard") },
		"contextbar":    ctx.View,
		"splitview":     func(f Frame) string { return sv.Join(f, "left pane content", "right pane content") },
	}
}

func TestComponentsNoOverflowAtAcceptanceSizes(t *testing.T) {
	s := styles()
	for _, sz := range acceptanceSizes {
		// Give each component a realistic interior frame (minus chrome rows).
		f := Frame{W: sz[0], H: sz[1] - 3}
		for name, render := range renderers(s) {
			out := render(f)
			assertNoOverflow(t, name+"@"+sizeName(sz), out, f)
		}
	}
}

func TestComponentsNoPanicBelowFloor(t *testing.T) {
	s := styles()
	// Empty and tiny frames must render (empty) without panic.
	for _, f := range []Frame{{W: 0, H: 0}, {W: 1, H: 1}, {W: 10, H: 2}} {
		for _, render := range renderers(s) {
			_ = render(f)
		}
	}
}

func sizeName(sz [2]int) string {
	return itoa(sz[0]) + "x" + itoa(sz[1])
}

func itoa(n int) string {
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

// --- fixtures -------------------------------------------------------------

func sampleSubgraph() *schema.Subgraph {
	return &schema.Subgraph{
		Center: "bc1qcenter",
		Depth:  2,
		Nodes: []schema.GraphNode{
			{ID: "bc1qcenter", Type: schema.NodeWallet},
			{ID: "tx9ab3cafef00d", Type: schema.NodeTransaction},
			{ID: "203.0.113.9", Type: schema.NodeIP},
			{ID: "cluster-04", Type: schema.NodeEntity},
		},
		// Edges from the center to each other node so the ego diagram has a
		// real first-hop neighbourhood to draw.
		Edges: []schema.GraphEdge{
			{ID: "e1", Type: schema.EdgeInputTo, From: "bc1qcenter", To: "tx9ab3cafef00d"},
			{ID: "e2", Type: schema.EdgeObservedWith, From: "203.0.113.9", To: "bc1qcenter"},
			{ID: "e3", Type: schema.EdgeMemberOf, From: "bc1qcenter", To: "cluster-04"},
		},
	}
}

func sampleObs() []schema.NetworkObservation {
	return []schema.NetworkObservation{
		{SrcIP: "1.1.1.1", Country: "DE", ASN: "AS3320"},
		{SrcIP: "1.1.1.2", Country: "DE", ASN: "AS3320"},
		{SrcIP: "1.1.1.3", Country: "US", ASN: "AS7922"},
		{SrcIP: "1.1.1.4", Country: "", ASN: ""},
	}
}

func sampleEvents() []models.TimelineEvent {
	return []models.TimelineEvent{
		{Kind: models.KindTx, Timestamp: "2024-11-02T14:01:00Z", ID: "tx1", Detail: "0.42 BTC in"},
		{Kind: models.KindMonitorEvent, Timestamp: "2024-11-02T14:02:00Z", ID: "ev1", Detail: "seen"},
		{Kind: models.KindRiskDelta, Timestamp: "2024-11-02T14:03:00Z", ID: "rd1", Detail: "63->72"},
		{Kind: models.KindAlert, Timestamp: "2024-11-02T14:03:10Z", ID: "al-crit", Detail: "structuring_like"},
	}
}

func sampleAlerts() []schema.Alert {
	return []schema.Alert{
		{ID: "al-crit", Subject: "bc1qcenter", Type: "structuring_like", Risk: 88, Status: schema.AlertNew, Reason: "many small outputs"},
		{ID: "al-high", Subject: "3F2a9x", Type: "peel_chain_like", Risk: 60, Status: schema.AlertReviewing, Reason: "peel chain"},
	}
}
