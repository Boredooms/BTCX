package screens

import (
	"fmt"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
)

// riskchart.go builds the risk-distribution breakdown: how many alerts fall in
// each presentation band (LOW / ELEVATED / HIGH / CRITICAL, design §10), as a
// multi-colored horizontal bar chart. It is PRESENTATION ONLY — the counts come
// from the real persisted alert rows (ListAlerts); nothing is fabricated. The
// band thresholds and colors mirror the Wallet screen's riskBand/riskRole so
// the whole app reads one risk taxonomy.

// RiskBandCount is one band's label, count, and semantic color role.
type RiskBandCount struct {
	Band  string
	Count int
	Role  theme.Role
}

// riskBandOrder is the fixed, highest-first display order so the chart always
// reads CRITICAL → HIGH → ELEVATED → LOW regardless of the data.
var riskBandBuckets = []struct {
	label string
	min   int
	role  theme.Role
}{
	{"CRITICAL", 75, theme.RoleCritical},
	{"HIGH", 50, theme.RoleWarning},
	{"ELEVATED", 25, theme.RoleNeutral},
	{"LOW", 0, theme.RoleHealthy},
}

// bandFor maps a 0..100 risk score to its presentation band label + role,
// mirroring the Wallet screen's riskBand/riskRole (one taxonomy app-wide).
func bandFor(score int) (string, theme.Role) {
	for _, b := range riskBandBuckets {
		if score >= b.min {
			return b.label, b.role
		}
	}
	return "LOW", theme.RoleHealthy
}

// RiskHistogram buckets alerts into the four presentation bands (highest-first)
// and returns the per-band counts. Every alert is counted exactly once by its
// risk score; an empty input yields four zero-count bands so the chart still
// renders its axis honestly (an all-zero distribution, not a blank panel).
func RiskHistogram(alerts []schema.Alert) []RiskBandCount {
	counts := map[string]int{}
	for _, a := range alerts {
		label, _ := bandFor(a.Risk)
		counts[label]++
	}
	out := make([]RiskBandCount, 0, len(riskBandBuckets))
	for _, b := range riskBandBuckets {
		out = append(out, RiskBandCount{Band: b.label, Count: counts[b.label], Role: b.role})
	}
	return out
}

// riskHistogramSamples projects the band counts into BarChart samples (label =
// "BAND (n)", value = count, color = the band role) so the bars are
// multi-colored by severity.
func riskHistogramSamples(bands []RiskBandCount) []components.BarSample {
	samples := make([]components.BarSample, 0, len(bands))
	for _, b := range bands {
		samples = append(samples, components.BarSample{
			Label: b.Band,
			Value: float64(b.Count),
			Role:  b.Role,
		})
	}
	return samples
}

// RenderRiskChart renders the risk-distribution bar chart for a set of alerts
// into the frame: a multi-colored horizontal bar per band with its count. Total
// is shown so the reader sees the population size at a glance. Honest empty
// state when there are no alerts.
func RenderRiskChart(styles theme.Styles, alerts []schema.Alert, f components.Frame) string {
	if f.Empty() {
		return ""
	}
	bands := RiskHistogram(alerts)
	chart := components.NewBarChart(styles, "", "")
	// Cap the bar region width so the "LABEL ███ count" row always fits on ONE
	// line (a very wide frame would otherwise push the count onto its own row).
	cw := f.W
	if cw > 72 {
		cw = 72
	}
	return chart.View(riskHistogramSamples(bands), components.Frame{W: cw, H: f.H})
}

// fmtRiskSummary renders a one-line tally "N alerts · C critical · H high · E
// elevated · L low" from the highest-first band counts.
func fmtRiskSummary(total int, bands []RiskBandCount) string {
	m := map[string]int{}
	for _, b := range bands {
		m[b.Band] = b.Count
	}
	return fmt.Sprintf("%d alerts · %d critical · %d high · %d elevated · %d low",
		total, m["CRITICAL"], m["HIGH"], m["ELEVATED"], m["LOW"])
}
