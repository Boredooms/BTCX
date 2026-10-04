package components

import (
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// TimelineView renders a chronology of reporting TimelineEvents (design §3).
// reporting.models.TimelineEvent is {Kind, Timestamp, ID, Detail} with NO
// per-event severity. The view therefore renders ONE glyph per Kind and shows a
// severity ONLY on alert-kind rows, looked up by the event ID in a supplied
// alerts map (design §3, acceptance §19.17). It never implies a severity exists
// on every row.
type TimelineView struct {
	styles theme.Styles
	events []models.TimelineEvent
	// alertSeverity maps an alert id -> severity, used ONLY for alert-kind rows.
	alertSeverity map[string]string
}

// NewTimelineView builds an empty timeline.
func NewTimelineView(styles theme.Styles) TimelineView {
	return TimelineView{styles: styles, alertSeverity: map[string]string{}}
}

// SetEvents loads events and the alert-id -> severity lookup. The lookup is
// built from the snapshot's alerts by the parent; the view does not fabricate
// severities for non-alert kinds.
func (t *TimelineView) SetEvents(events []models.TimelineEvent, alerts []schema.Alert) {
	t.events = events
	t.alertSeverity = map[string]string{}
	for _, a := range alerts {
		t.alertSeverity[a.ID] = severityFromRisk(a)
	}
}

// severityFromRisk derives a display severity for an alert from its risk score,
// using the presentation bands (design §10). This is a presentation cue only.
func severityFromRisk(a schema.Alert) string {
	switch {
	case a.Risk >= 75:
		return "CRITICAL"
	case a.Risk >= 50:
		return "HIGH"
	case a.Risk >= 25:
		return "ELEVATED"
	default:
		return "LOW"
	}
}

// kindGlyph returns the per-Kind glyph (design §3). This is keyed on Kind, not
// severity — the single rule that makes the timeline data-honest.
func (t TimelineView) kindGlyph(k models.TimelineEventKind) string {
	if t.styles.Theme().Degraded {
		switch k {
		case models.KindTx:
			return "#"
		case models.KindMonitorEvent:
			return "o"
		case models.KindRiskDelta:
			return "d"
		case models.KindAlert:
			return "!"
		default:
			return "."
		}
	}
	switch k {
	case models.KindTx:
		return "▣"
	case models.KindMonitorEvent:
		return "◇"
	case models.KindRiskDelta:
		return "Δ"
	case models.KindAlert:
		return "!"
	default:
		return "·"
	}
}

// Init implements the sub-model contract.
func (t TimelineView) Init() tea.Cmd { return nil }

// Update is a no-op at this level (scroll lands with the viewport wiring).
func (t TimelineView) Update(tea.Msg) (TimelineView, tea.Cmd) { return t, nil }

// RenderRow renders a single event line. Exported for tests that assert the
// glyph-per-kind and severity-only-on-alert rules directly.
func (t TimelineView) RenderRow(e models.TimelineEvent) string {
	glyph := t.kindGlyph(e.Kind)
	ts := e.Timestamp
	line := t.styles.Role(theme.RoleInfo).Render(glyph) + " " +
		t.styles.Role(theme.RoleLabel).Render(ts) + " " +
		t.styles.Role(theme.RoleValue).Render(string(e.Kind)) + " " +
		t.styles.Role(theme.RoleValue).Render(e.Detail)

	// Severity is shown ONLY for alert-kind rows, and only when the alert id is
	// known in the lookup. Every other kind carries no severity.
	if e.Kind == models.KindAlert {
		if sev, ok := t.alertSeverity[e.ID]; ok {
			line += " " + t.styles.SeverityChip(sev, "["+sev+"]")
		}
	}
	return line
}

// HasSeverity reports whether a rendered row would carry a severity chip. Used
// by tests to assert the alert-only rule without string scraping.
func (t TimelineView) HasSeverity(e models.TimelineEvent) bool {
	if e.Kind != models.KindAlert {
		return false
	}
	_, ok := t.alertSeverity[e.ID]
	return ok
}

// View renders the timeline into the frame.
func (t TimelineView) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	if len(t.events) == 0 {
		return clampBlock(t.styles.Role(theme.RoleLabel).Render("no timeline events"), f)
	}
	var lines []string
	for _, e := range t.events {
		lines = append(lines, clampLine(t.RenderRow(e), f.W))
	}
	if len(lines) > f.H {
		lines = lines[:f.H]
	}
	return clampBlock(strings.Join(lines, "\n"), f)
}
