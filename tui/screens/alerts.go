package screens

import (
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Alerts is the Alerts screen (design §2). It lists the active case's alerts
// (ListAlerts) through the AlertList component, which derives a presentation
// severity band from each alert's risk score (design §10) and never asserts a
// crime (AGENTS §18). Monitor alerts are session-scoped and surfaced from the
// Monitoring screen's history; this screen shows the case alert list.
type Alerts struct {
	ctx    *ScreenCtx
	styles theme.Styles

	list components.AlertList
	// alerts is the loaded alert set, kept so the risk-distribution chart can
	// bucket them by band (how many risky are there, per severity).
	alerts []schema.Alert

	// summary holds the OPTIONAL local-LLM pane state (design §E.4). The Alerts
	// screen loads no InvestigationResult, so L is always the NIT-4 honest no-op
	// "open a subject first" here — it never dispatches a summarizer call.
	summary summaryPane
}

const alertsLimit = 200

// NewAlerts builds the Alerts screen.
func NewAlerts(ctx *ScreenCtx) *Alerts {
	return &Alerts{
		ctx:    ctx,
		styles: ctx.styles(),
		list:   components.NewAlertList(ctx.styles()),
	}
}

// Init loads the case alerts.
func (a *Alerts) Init() tea.Cmd {
	repo := a.ctx.repo()
	if repo == nil {
		a.list.SetError(errNoCase.Error())
		return nil
	}
	a.list.SetLoading()
	return alertsCmd(a.ctx.bgCtx(), repo, alertsLimit)
}

// Update folds in the alerts.
func (a *Alerts) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if alerts, ok := m.Payload.([]schema.Alert); ok {
			a.alerts = alerts
			a.list.SetAlerts(alerts)
		}
	case dataError:
		a.list.SetError(m.Err.Error())
	case SummaryLoaded:
		a.summary.acceptSummary(a.ctx, m)
		return a, nil
	case tea.KeyMsg:
		if m.String() == "L" {
			// NIT-4: no InvestigationResult is loaded on the Alerts screen, so
			// this is an honest no-op note, never a summarizer/network call.
			return a, a.summary.requestExplain(a.ctx, nil)
		}
		var cmd tea.Cmd
		a.list, cmd = a.list.Update(m)
		return a, cmd
	}
	return a, nil
}

// View renders the alert list.
func (a *Alerts) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	total := len(a.alerts)
	title := a.styles.Title.Render("ALERTS") + " " +
		a.styles.Label.Render(riskSummaryLine(a.alerts))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	// Reserve the lower portion for the optional LLM pane when it is active.
	paneH := 0
	if a.summary.active {
		paneH = bodyH / 3
		if paneH < 4 {
			paneH = 4
		}
		if paneH > bodyH-2 {
			paneH = bodyH - 2
		}
	}
	mainH := bodyH - paneH
	if mainH < 6 {
		mainH = 6
	}
	// RISK DISTRIBUTION chart panel on top (multi-colored bars, one per band),
	// then the risk-ranked alert list below. One separator row between them.
	sep := 1
	// The chart needs a compact fixed height: 4 band rows + border(2) + title(1)
	// = 7, clamped so a short terminal still leaves room for the list.
	chartH := 7
	if chartH > mainH-5 {
		chartH = mainH - 5
	}
	if chartH < 4 {
		chartH = 4
	}
	listH := mainH - chartH - sep
	if listH < 3 {
		listH = 3
	}
	chartTitle := "RISK DISTRIBUTION — ALERTS BY BAND"
	if total == 0 {
		chartTitle = "RISK DISTRIBUTION — no alerts in this case yet"
	}
	chartPanel := components.Panel(a.styles, chartTitle,
		RenderRiskChart(a.styles, a.alerts, components.Frame{W: f.W - 2, H: paneInner(chartH)}),
		components.Frame{W: f.W, H: chartH})
	listPanel := components.Panel(a.styles, "CASE ALERTS — RISK-RANKED",
		a.list.View(components.Frame{W: f.W - 2, H: paneInner(listH)}),
		components.Frame{W: f.W, H: listH})
	out := title + "\n" + chartPanel + "\n" + listPanel
	if paneH > 0 {
		out += "\n" + renderSummaryPane(a.styles, a.summary, components.Frame{W: f.W, H: paneH})
	}
	return clampBlockLocal(out, f)
}

// riskSummaryLine is a compact header tally: total alerts + the per-band counts
// (critical/high/elevated/low), so the number is visible even before the chart.
func riskSummaryLine(alerts []schema.Alert) string {
	if len(alerts) == 0 {
		return "0 alerts"
	}
	bands := RiskHistogram(alerts)
	// bands is highest-first: CRITICAL, HIGH, ELEVATED, LOW.
	return fmtRiskSummary(len(alerts), bands)
}

// ShortHelp lists Alerts' context keys.
func (a *Alerts) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open evidence")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "explain (local)")),
	}
}
