package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
)

// alertsAcross builds a slice of alerts spanning all four bands for the chart.
func alertsAcross() []schema.Alert {
	scores := []int{10, 20, 30, 45, 55, 60, 70, 80, 95} // low×2, elev×2, high×3, crit×2
	out := make([]schema.Alert, 0, len(scores))
	for i, s := range scores {
		out = append(out, schema.Alert{ID: "a", Subject: "s", Risk: s})
		_ = i
	}
	return out
}

// TestRiskHistogramBuckets asserts alerts are bucketed into the four bands by
// score with the fixed highest-first order and correct per-band counts.
func TestRiskHistogramBuckets(t *testing.T) {
	bands := RiskHistogram(alertsAcross())
	if len(bands) != 4 {
		t.Fatalf("expected 4 bands, got %d", len(bands))
	}
	want := map[string]int{"CRITICAL": 2, "HIGH": 3, "ELEVATED": 2, "LOW": 2}
	order := []string{"CRITICAL", "HIGH", "ELEVATED", "LOW"}
	for i, b := range bands {
		if b.Band != order[i] {
			t.Errorf("band %d order: got %q want %q", i, b.Band, order[i])
		}
		if b.Count != want[b.Band] {
			t.Errorf("%s count: got %d want %d", b.Band, b.Count, want[b.Band])
		}
	}
}

// TestRiskHistogramEmptyRendersAxis asserts an empty alert set still yields four
// zero-count bands (an honest all-zero distribution, not a blank panel).
func TestRiskHistogramEmpty(t *testing.T) {
	bands := RiskHistogram(nil)
	if len(bands) != 4 {
		t.Fatalf("empty should still produce 4 bands, got %d", len(bands))
	}
	for _, b := range bands {
		if b.Count != 0 {
			t.Errorf("empty band %s should be 0, got %d", b.Band, b.Count)
		}
	}
}

// TestRenderRiskChartShowsBands asserts the rendered chart contains every band
// label and the bar glyph, so the operator sees a multi-band breakdown.
func TestRenderRiskChartShowsBands(t *testing.T) {
	styles := theme.Build(theme.Default())
	out := RenderRiskChart(styles, alertsAcross(), components.Frame{W: 60, H: 6})
	for _, band := range []string{"CRITICAL", "HIGH", "ELEVATED", "LOW"} {
		if !strings.Contains(out, band) {
			t.Errorf("risk chart should show band %q; got:\n%s", band, out)
		}
	}
	if !strings.Contains(out, "█") {
		t.Errorf("risk chart should draw bar glyphs; got:\n%s", out)
	}
}

// TestRiskSummaryLine asserts the compact tally line reports the total and the
// per-band counts.
func TestRiskSummaryLine(t *testing.T) {
	line := riskSummaryLine(alertsAcross())
	for _, want := range []string{"9 alerts", "2 critical", "3 high", "2 elevated", "2 low"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary line missing %q; got %q", want, line)
		}
	}
}
