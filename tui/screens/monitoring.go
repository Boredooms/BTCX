package screens

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Monitoring is the Live Monitoring screen (design §2). It lists the recorded
// monitor sessions (ListMonitorSessions) and, for a selected session, its
// history (ListMonitorEvents / ListRiskDeltas) in the append-only MonitorStream.
//
// Honest LIVE marker (AGENTS §16): the stream shows ●LIVE only while a
// MonitorWallet loop is actually running. The TUI itself never constructs a
// network provider (that import would pull HTTP transport into tui, breaking the
// offline boundary), so sessions shown here are historical and marked ●ENDED.
// Starting a live session is network-touching: the screen calls the offline
// gate app.AcquisitionAllowed(cfg) FIRST (design §12 invariant 1); when offline
// it blocks with an honest message and never attempts a provider construction.
type Monitoring struct {
	ctx    *ScreenCtx
	styles theme.Styles

	sessions []sdk.MonitorSessionRow
	table    components.Table
	stream   components.MonitorStream
	selected string
	notice   string

	// target is the in-screen selector for choosing a wallet/tx to monitor.
	// Pressing 't' focuses it; on submit the id is classified and the screen
	// shows the exact gated `bctx monitor` command for it (the TUI never
	// constructs a provider itself, keeping tui/ transport-free). targetNote
	// carries the resulting command / honest reject message.
	target     components.SearchBox
	targetNote string

	// chart + chartSamples drive the long-horizon MOVEMENT panel: the selected
	// session's risk-over-time (from its persisted risk deltas), rendered as a
	// bar chart so a monitored wallet's movement reads at a glance. riskSpark is
	// a dense inline trend for the panel header.
	chart        components.BarChart
	chartSamples []components.BarSample
	riskSpark    string
	eventCount   int
}

const monitorHistoryLimit = 500

// NewMonitoring builds the Monitoring screen.
func NewMonitoring(ctx *ScreenCtx) *Monitoring {
	cols := []table.Column{
		{Title: "session", Width: 16},
		{Title: "target", Width: 18},
		{Title: "status", Width: 10},
		{Title: "health", Width: 12},
		{Title: "events", Width: 8},
		{Title: "risk", Width: 6},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("no monitor sessions recorded in this case")
	mo := &Monitoring{
		ctx:    ctx,
		styles: ctx.styles(),
		table:  tbl,
		stream: components.NewMonitorStream(ctx.styles()),
		target: components.NewSearchBox(ctx.styles(), "monitor target — wallet / txid"),
		chart:  components.NewBarChart(ctx.styles(), "", ""),
	}
	// Seed the selector with the active global subject so 't' pre-fills a
	// sensible target (the subject the user was just investigating).
	if ctx != nil && ctx.Subject.ID != "" {
		mo.target.SetValue(ctx.Subject.ID)
	}
	return mo
}

// Focused reports whether the monitor-target input owns the keyboard, so the
// Root suppresses single-letter nav (incl. the 't' focus key) while typing.
func (mo *Monitoring) Focused() bool { return mo.target.Focused() }

// Init loads the recorded sessions.
func (mo *Monitoring) Init() tea.Cmd {
	repo := mo.ctx.repo()
	if repo == nil {
		mo.table.SetError(errNoCase.Error())
		return nil
	}
	mo.table.SetLoading()
	return monitorSessionsCmd(mo.ctx.bgCtx(), repo)
}

// Update folds in sessions/history and handles start-session gating.
func (mo *Monitoring) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		switch p := m.Payload.(type) {
		case []sdk.MonitorSessionRow:
			mo.setSessions(p)
		case monitorHistory:
			mo.applyHistory(p)
		}
	case dataError:
		mo.table.SetError(m.Err.Error())
	case components.RowSelected:
		mo.selected = m.ID
		return mo, monitorHistoryCmd(mo.ctx.bgCtx(), mo.ctx.repo(), m.ID, monitorHistoryLimit)
	case components.SearchSubmitted:
		mo.submitTarget(m.Query)
		return mo, nil
	case tea.KeyMsg:
		// While the target selector is focused, editing keys go to it; Enter
		// submits (SearchSubmitted above) and Esc blurs it.
		if mo.target.Focused() {
			var cmd tea.Cmd
			mo.target, cmd = mo.target.Update(m)
			return mo, cmd
		}
		switch m.String() {
		case "t":
			// Focus the target selector to choose a wallet/tx to monitor.
			mo.targetNote = ""
			return mo, mo.target.Focus()
		case "s":
			return mo, mo.startSession()
		}
		var cmd tea.Cmd
		mo.table, cmd = mo.table.Update(m)
		return mo, cmd
	}
	// Non-key messages (textinput blink) reach the target box when focused so
	// its cursor keeps blinking.
	if mo.target.Focused() {
		var cmd tea.Cmd
		mo.target, cmd = mo.target.Update(msg)
		return mo, cmd
	}
	return mo, nil
}

// submitTarget classifies the chosen monitor target and, when valid, records
// the exact gated `bctx monitor` command for it. The TUI never constructs a
// provider itself (that import would pull HTTP transport into tui/), so the
// screen surfaces the command the user runs while connected; the offline gate
// still applies when they run it. An empty/garbage id yields an honest note.
func (mo *Monitoring) submitTarget(raw string) {
	id := strings.TrimSpace(raw)
	mo.target.Blur()
	if id == "" {
		mo.targetNote = "enter a wallet address or 64-hex txid to monitor"
		return
	}
	kind, ok := classifySubject(id)
	if !ok {
		mo.targetNote = "unrecognized id — expected a wallet address or 64-hex txid"
		return
	}
	// Only wallet/tx targets are monitorable; map the classifier kind to the
	// matching subcommand.
	sub := ""
	switch kind {
	case "wallet":
		sub = "wallet"
	case "tx":
		sub = "tx"
	default:
		mo.targetNote = "only wallet or transaction targets can be monitored (got " + kind + ")"
		return
	}
	// Honour the offline gate in the hint itself so the user knows up front
	// whether a live session can start right now.
	if err := mo.ctx.acquisitionAllowed(); err != nil {
		mo.targetNote = "selected " + sub + " " + shortID(id) +
			" — but acquisition is offline/airgapped; reconnect, then run: bctx monitor " + sub + " " + id
		return
	}
	mo.targetNote = "run while connected:  bctx monitor " + sub + " " + id +
		"   (events stream here as ●LIVE; this view also replays recorded sessions)"
}

// startSession enforces the offline gate BEFORE any provider construction
// (design §12 invariant 1, acceptance §19.10/§19.13). When acquisition is not
// allowed it blocks with the honest acquisition.ErrOfflineAcquisition message.
// When allowed, it reports that the live loop is launched via the CLI, because
// the TUI deliberately does not import a network provider (keeping tui/
// transport-free); the LIVE marker is never faked.
func (mo *Monitoring) startSession() tea.Cmd {
	if err := mo.ctx.acquisitionAllowed(); err != nil {
		if errors.Is(err, acquisition.ErrOfflineAcquisition) {
			mo.notice = "ACQUISITION BLOCKED — offline/airgapped: start a live session only while connected."
		} else {
			mo.notice = "cannot start session: " + err.Error()
		}
		return nil
	}
	mo.notice = "Live monitoring requires a connected provider — start it with: bctx monitor wallet <addr>. " +
		"This view shows recorded sessions and their history."
	return nil
}

func (mo *Monitoring) setSessions(s []sdk.MonitorSessionRow) {
	mo.sessions = s
	rows := make([]table.Row, 0, len(s))
	ids := make([]string, 0, len(s))
	kinds := make([]string, 0, len(s))
	for _, se := range s {
		rows = append(rows, table.Row{
			shortID(se.SessionID),
			shortID(se.Target),
			se.Status,
			se.Health,
			fmt.Sprintf("%d", se.EventsSeen),
			fmt.Sprintf("%d", se.LastRiskScore),
		})
		ids = append(ids, se.SessionID)
		kinds = append(kinds, "session")
	}
	mo.table.SetRows(rows, ids, kinds)
}

// applyHistory replays a recorded session into the append-only stream. The
// stream is marked ENDED: these are historical rows, not a running loop
// (AGENTS §16 — LIVE is never shown for replayed history).
func (mo *Monitoring) applyHistory(h monitorHistory) {
	mo.stream = components.NewMonitorStream(mo.styles)
	mo.stream.SetLive(false)
	for _, e := range h.Events {
		mo.stream.Append(components.StreamLine{
			Timestamp: e.Timestamp, Kind: "event",
			Text: fmt.Sprintf("%s tx %s block %d", e.Type, shortID(e.TxID), e.BlockHeight)})
	}
	for _, d := range h.Deltas {
		mo.stream.Append(components.StreamLine{
			Timestamp: d.Timestamp, Kind: "risk_delta",
			Text: fmt.Sprintf("%d → %d (%+d)", d.PreviousScore, d.CurrentScore, d.Delta)})
	}

	// Build the long-horizon MOVEMENT chart from the risk-delta history: one bar
	// per recorded risk point (current score at that step), colored by its band,
	// plus a dense sparkline for the header. All from real persisted deltas.
	mo.chartSamples = mo.chartSamples[:0]
	var trend []float64
	for i, d := range h.Deltas {
		label := shortMonitorTime(d.Timestamp)
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
		}
		mo.chartSamples = append(mo.chartSamples, components.BarSample{
			Label: label,
			Value: float64(d.CurrentScore),
			Role:  theme.SeverityRole(riskBand(d.CurrentScore)),
		})
		trend = append(trend, float64(d.CurrentScore))
	}
	mo.riskSpark = components.Sparkline(trend)
	mo.eventCount = len(h.Events)
}

// shortMonitorTime extracts HH:MM from a stored RFC3339-ish timestamp, or "".
func shortMonitorTime(ts string) string {
	// Stored timestamps look like 2024-11-02T14:03:00Z; take the HH:MM.
	if i := strings.IndexByte(ts, 'T'); i >= 0 && len(ts) >= i+6 {
		return ts[i+1 : i+6]
	}
	return ""
}

// View renders the session table over the historical stream.
func (mo *Monitoring) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(mo.styles.Title.Render("LIVE MONITORING"))
	b.WriteString("\n")
	// Target selector row: the in-screen chooser for what wallet/tx to monitor.
	targetRow := mo.target.View(components.Frame{W: f.W, H: 1})
	switch {
	case mo.targetNote != "":
		targetRow = clampLineLocal(targetRow, f.W/3) + "  " +
			mo.styles.Role(theme.RoleInfo).Render(mo.targetNote)
	case !mo.target.Focused():
		targetRow = clampLineLocal(targetRow, f.W/3) + "  " +
			mo.styles.Muted.Render("press t to choose a monitor target (wallet / txid)")
	}
	b.WriteString(clampLineLocal(targetRow, f.W))
	b.WriteString("\n")
	if mo.notice != "" {
		b.WriteString(mo.styles.Role(theme.RoleWarning).Render(mo.notice))
		b.WriteString("\n")
	}
	used := strings.Count(b.String(), "\n")
	remaining := f.H - used
	if remaining < 3 {
		remaining = 3
	}

	// Three stacked bordered panels (two separators reserved): the session
	// table, the long-horizon MOVEMENT chart (risk over time for the selected
	// session), and the event stream. The chart only takes space once a session
	// is selected; otherwise its rows go to the stream.
	hasChart := len(mo.chartSamples) > 0
	sep := 1 // table -> stream separator
	if hasChart {
		sep = 2 // table -> chart -> stream
	}
	usable := remaining - sep
	if usable < 6 {
		usable = 6
	}
	tblH := usable / 3
	if tblH < 2 {
		tblH = 2
	}
	chartH := 0
	if hasChart {
		chartH = usable / 3
		if chartH < 5 {
			chartH = 5
		}
	}
	streamH := usable - tblH - chartH
	if streamH < 2 {
		streamH = 2
	}

	b.WriteString(components.Panel(mo.styles, "MONITOR SESSIONS",
		mo.table.View(components.Frame{W: f.W - 2, H: paneInner(tblH)}),
		components.Frame{W: f.W, H: tblH}))
	b.WriteString("\n")
	if hasChart {
		title := "MOVEMENT — RISK OVER TIME"
		if mo.riskSpark != "" {
			title += "  " + mo.riskSpark
		}
		if mo.eventCount > 0 {
			title += fmt.Sprintf("  · %d events", mo.eventCount)
		}
		cw, _ := components.PanelInner(components.Frame{W: f.W, H: chartH})
		b.WriteString(components.Panel(mo.styles, title,
			mo.chart.View(mo.chartSamples, components.Frame{W: cw, H: paneInner(chartH)}),
			components.Frame{W: f.W, H: chartH}))
		b.WriteString("\n")
	}
	b.WriteString(components.Panel(mo.styles, "EVENT STREAM (HISTORY)",
		mo.stream.View(components.Frame{W: f.W - 2, H: paneInner(streamH)}),
		components.Frame{W: f.W, H: streamH}))
	return clampBlockLocal(b.String(), f)
}

// ShortHelp lists Monitoring's context keys.
func (mo *Monitoring) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "choose target")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open session")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start (gated)")),
	}
}
