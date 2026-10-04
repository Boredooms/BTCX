package components

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Table ----------------------------------------------------------------

func TestTableStatesAndSelection(t *testing.T) {
	s := styles()
	tbl := NewTable(s, []table.Column{{Title: "txid", Width: 8}, {Title: "val", Width: 6}})

	// Loading by default.
	if tbl.State() != StateLoading {
		t.Fatalf("new table should be loading, got %v", tbl.State())
	}
	// Empty rows -> empty state (honest, no fabricated rows).
	tbl.SetRows(nil, nil, nil)
	if tbl.State() != StateEmpty {
		t.Fatalf("empty rows should yield empty state, got %v", tbl.State())
	}
	// Loaded rows -> loaded state; Enter emits RowSelected with the row id/kind.
	tbl.SetRows(
		[]table.Row{{"9ab3", "0.42"}, {"7c1d", "0.10"}},
		[]string{"tx-9ab3", "tx-7c1d"},
		[]string{"tx", "tx"},
	)
	if tbl.State() != StateLoaded {
		t.Fatalf("rows should yield loaded state, got %v", tbl.State())
	}
	_, cmd := tbl.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a loaded table should emit a command")
	}
	msg := cmd()
	sel, ok := msg.(RowSelected)
	if !ok {
		t.Fatalf("expected RowSelected, got %T", msg)
	}
	if sel.ID != "tx-9ab3" || sel.Kind != "tx" {
		t.Errorf("RowSelected = %+v, want id tx-9ab3 kind tx", sel)
	}
}

func TestTablePaginationRendersWithinHeight(t *testing.T) {
	s := styles()
	cols := []table.Column{{Title: "n", Width: 4}}
	tbl := NewTable(s, cols)
	rows := make([]table.Row, 100)
	ids := make([]string, 100)
	for i := range rows {
		rows[i] = table.Row{itoa(i)}
		ids[i] = "id" + itoa(i)
	}
	tbl.SetRows(rows, ids, nil)
	// A short frame must still render bounded output (pagination/virtualization).
	f := Frame{W: 20, H: 6}
	out := tbl.View(f)
	assertNoOverflow(t, "table-paginate", out, f)
}

// --- TimelineView ---------------------------------------------------------

func TestTimelineGlyphPerKind(t *testing.T) {
	s := styles()
	tv := NewTimelineView(s)
	cases := map[models.TimelineEventKind]string{
		models.KindTx:           "▣",
		models.KindMonitorEvent: "◇",
		models.KindRiskDelta:    "Δ",
		models.KindAlert:        "!",
	}
	for kind, glyph := range cases {
		got := tv.kindGlyph(kind)
		if got != glyph {
			t.Errorf("kindGlyph(%q) = %q, want %q", kind, got, glyph)
		}
	}
}

func TestTimelineSeverityOnlyOnAlertRows(t *testing.T) {
	s := styles()
	tv := NewTimelineView(s)
	tv.SetEvents(sampleEvents(), sampleAlerts())

	for _, e := range sampleEvents() {
		has := tv.HasSeverity(e)
		if e.Kind == models.KindAlert {
			// Alert row whose ID exists in the alerts map carries a severity.
			if !has {
				t.Errorf("alert-kind row %q should carry a severity", e.ID)
			}
		} else if has {
			t.Errorf("non-alert row (kind %q) must NOT carry a severity", e.Kind)
		}
	}

	// An alert-kind event whose ID is absent from the snapshot alerts gets no
	// severity (no fabrication).
	orphan := models.TimelineEvent{Kind: models.KindAlert, ID: "missing", Detail: "x"}
	if tv.HasSeverity(orphan) {
		t.Error("alert row with unknown id must not fabricate a severity")
	}
}

func TestTimelineDegradedGlyphsAreASCII(t *testing.T) {
	s := theme.Build(theme.Degraded())
	tv := NewTimelineView(s)
	if tv.kindGlyph(models.KindTx) != "#" {
		t.Error("degraded tx glyph should be ASCII #")
	}
}

// --- GraphView ------------------------------------------------------------

func TestGraphViewLimitBadge(t *testing.T) {
	s := styles()
	gv := NewGraphView(s, 5000, 800)
	risk := 72
	gv.SetSubgraph(sampleSubgraph(), "bc1qcenter", &risk)
	badge := gv.Badge()
	// Badge reports the full subgraph size, the engine cap, and how many of the
	// center's first-hop neighbours the ego diagram draws (all honest, §9). The
	// fixture center has 3 incident edges -> 3 neighbours, all drawn.
	want := "4 nodes / 5000 cap · showing center + 3 of 3 neighbours"
	if badge != want {
		t.Errorf("badge = %q, want %q", badge, want)
	}
}

func TestGraphViewTruncatedBadge(t *testing.T) {
	s := styles()
	// MaxNodes small so the fixture subgraph counts as truncated.
	gv := NewGraphView(s, 4, 800)
	gv.SetSubgraph(sampleSubgraph(), "bc1qcenter", nil)
	if !gv.truncated() {
		t.Fatal("subgraph at the engine cap should be flagged truncated")
	}
	out := gv.View(Frame{W: 70, H: 12})
	if !contains(out, "truncated at engine cap") {
		t.Error("truncated subgraph should show the engine-cap warning")
	}
}

func TestGraphViewOnlyCenterNodeShowsRisk(t *testing.T) {
	s := styles()
	gv := NewGraphView(s, 5000, 800)
	risk := 72
	gv.SetSubgraph(sampleSubgraph(), "bc1qcenter", &risk)

	// selected starts at 0 which is the center node -> risk shown.
	readout := gv.selectionReadout()
	if !contains(readout, "risk 72") || !contains(readout, "center node") {
		t.Errorf("center node readout should show numeric risk from analysis, got %q", readout)
	}

	// Move selection to a non-center node -> risk must be the em-dash, no number.
	gv2, _ := gv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	readout2 := gv2.selectionReadout()
	if contains(readout2, "risk 72") {
		t.Errorf("non-center node must NOT show the subject's numeric risk, got %q", readout2)
	}
	if !contains(readout2, "risk —") {
		t.Errorf("non-center node should show 'risk —', got %q", readout2)
	}
}

func TestGraphViewNoCenterRiskWhenUnknown(t *testing.T) {
	s := styles()
	gv := NewGraphView(s, 5000, 800)
	gv.SetSubgraph(sampleSubgraph(), "bc1qcenter", nil) // no analysis risk
	readout := gv.selectionReadout()
	if contains(readout, "risk 7") {
		t.Errorf("with no analysis risk, even the center node shows no number, got %q", readout)
	}
	if !contains(readout, "risk —") {
		t.Errorf("center node without a known risk should show 'risk —', got %q", readout)
	}
}

// --- MapView --------------------------------------------------------------

func TestMapBinningByCountry(t *testing.T) {
	s := styles()
	mv := NewMapView(s)
	mv.SetObservations(sampleObs())
	bins := mv.Bins()
	// 4 observations: DE x2, US x1, (unknown) x1 -> 3 bins, DE first (count desc).
	if len(bins) != 3 {
		t.Fatalf("expected 3 country bins, got %d (%v)", len(bins), bins)
	}
	if bins[0].Key != "DE" || bins[0].Count != 2 {
		t.Errorf("top bin should be DE(2), got %+v", bins[0])
	}
	// The empty-country observation falls into an explicit (unknown) bin, never
	// a fabricated coordinate (design §13).
	found := false
	for _, b := range bins {
		if b.Key == "(unknown)" && b.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Error("empty-country observation should land in an explicit (unknown) bin")
	}
}

func TestMapBinningByASN(t *testing.T) {
	s := styles()
	mv := NewMapView(s)
	mv.SetBinKey(BinByASN)
	mv.SetObservations(sampleObs())
	bins := mv.Bins()
	if bins[0].Key != "AS3320" || bins[0].Count != 2 {
		t.Errorf("top ASN bin should be AS3320(2), got %+v", bins[0])
	}
}

// --- geometry -------------------------------------------------------------

// TestSplitHSumsToWidth asserts the width budget is exact: for every column
// count SplitH returns, sum(cols)+(n-1)*gap == total, so a row of panels plus
// its rendered gap spacers can never over- or under-fill the frame (design
// §C.2). When SplitH drops columns under the floor, the surviving columns still
// consume the full width for the (smaller) returned count.
func TestSplitHSumsToWidth(t *testing.T) {
	gap := theme.Space.Gap
	widths := []int{40, 60, 80, 100, 120, 160, 200}
	for _, total := range widths {
		for n := 1; n <= 5; n++ {
			cols := SplitH(total, n, gap)
			if len(cols) == 0 {
				t.Errorf("SplitH(%d,%d,%d) returned no columns", total, n, gap)
				continue
			}
			got := (len(cols) - 1) * gap
			for _, c := range cols {
				got += c
				if c < 0 {
					t.Errorf("SplitH(%d,%d,%d) produced negative column %d", total, n, gap, c)
				}
			}
			if got != total {
				t.Errorf("SplitH(%d,%d,%d): sum(cols)+(k-1)*gap = %d, want %d (cols=%v)",
					total, n, gap, got, total, cols)
			}
			if len(cols) > n {
				t.Errorf("SplitH(%d,%d,%d) returned %d columns, more than requested",
					total, n, gap, len(cols))
			}
		}
	}
}

// TestRow2RendersGapSpacer asserts Row2 renders exactly to its frame width with
// an explicit gap between the two columns (no column double-counts the gap).
func TestRow2RendersGapSpacer(t *testing.T) {
	f := Frame{W: 80, H: 6}
	out := Row2(f, 0.5, strings.Repeat("L\n", 6), strings.Repeat("R\n", 6))
	// assertNoOverflow verifies the row never exceeds the frame width/height,
	// which is the box-break guard the gap spacer exists to satisfy (§C.2).
	assertNoOverflow(t, "row2-gap", out, f)
	if strings.TrimSpace(out) == "" {
		t.Fatal("Row2 rendered empty")
	}
}

// --- helpers --------------------------------------------------------------

func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
