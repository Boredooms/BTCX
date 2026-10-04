package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
)

// TestMonitoringMovementChart asserts that selecting a session with recorded
// risk deltas renders the long-horizon MOVEMENT bar chart (risk over time) with
// a header sparkline — the long-horizon movement readout for a monitored
// wallet.
func TestMonitoringMovementChart(t *testing.T) {
	mo := NewMonitoring(fixtureCtx(t, Subject{}))
	// Feed a recorded session history (as monitorHistoryCmd would) with a rising
	// then falling risk track.
	h := monitorHistory{
		SessionID: "mon-x",
		Events: []sdk.MonitorEventRow{
			{EventID: "e1", Type: "new_tx", TxID: "TXA", Timestamp: "2024-11-02T14:00:00Z"},
			{EventID: "e2", Type: "new_tx", TxID: "TXB", Timestamp: "2024-11-02T14:05:00Z"},
		},
		Deltas: []sdk.RiskDeltaRow{
			{CurrentScore: 20, PreviousScore: 0, Delta: 20, Timestamp: "2024-11-02T14:00:00Z"},
			{CurrentScore: 48, PreviousScore: 20, Delta: 28, Timestamp: "2024-11-02T14:05:00Z"},
			{CurrentScore: 72, PreviousScore: 48, Delta: 24, Timestamp: "2024-11-02T14:10:00Z"},
			{CurrentScore: 61, PreviousScore: 72, Delta: -11, Timestamp: "2024-11-02T14:15:00Z"},
		},
	}
	m, _ := mo.Update(dataLoaded{Request: "mon-x", Payload: h})
	mo = m.(*Monitoring)

	out := mo.View(components.Frame{W: 156, H: 41})
	for _, want := range []string{
		"MOVEMENT — RISK OVER TIME",
		"█",        // bar glyph proves the chart drew bars
		"2 events", // event count in the panel title
	} {
		if !strings.Contains(out, want) {
			t.Errorf("monitoring movement chart missing %q in:\n%s", want, out)
		}
	}
	// Sparkline ramp char present in the header.
	if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Errorf("expected a header sparkline:\n%s", out)
	}
}

// TestMonitoringChartPreview prints the monitoring screen with movement bars.
func TestMonitoringChartPreview(t *testing.T) {
	mo := NewMonitoring(fixtureCtx(t, Subject{}))
	h := monitorHistory{
		SessionID: "mon-x",
		Deltas: []sdk.RiskDeltaRow{
			{CurrentScore: 18, Timestamp: "2024-11-02T14:00:00Z"},
			{CurrentScore: 40, Timestamp: "2024-11-02T14:05:00Z"},
			{CurrentScore: 63, Timestamp: "2024-11-02T14:10:00Z"},
			{CurrentScore: 88, Timestamp: "2024-11-02T14:15:00Z"},
			{CurrentScore: 74, Timestamp: "2024-11-02T14:20:00Z"},
		},
	}
	m, _ := mo.Update(dataLoaded{Request: "mon-x", Payload: h})
	mo = m.(*Monitoring)
	t.Logf("\n%s", mo.View(components.Frame{W: 120, H: 36}))
}
