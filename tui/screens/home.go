package screens

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/geoenrich"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Home is the Dashboard screen (design §2) — the "godmode" intelligence
// cockpit. It composes the shared bordered chrome (components.Panel / Tile /
// TileRow) into a dense, titled, multi-panel grid: a top row of metric tiles, a
// middle row with a geo-activity panel (the MapView over NetworkObservation
// bins) beside a transaction/entity graph panel (the persisted graph Stats), a
// lower row with a recent high-risk alert panel beside a risk/anomaly summary,
// and a bottom case/system detail strip.
//
// Every value comes from a live service seam — Repo.Counts (tiles),
// AllNetworkObservations (geo panel), graph.Builder GraphStats (graph panel),
// ListAlerts (high-risk + risk summary), and ModelsStatus/GeoIPStatus (system
// strip). It holds no business logic and fabricates nothing: an empty case
// shows honest zeros and honest empty states (AGENTS §12, §15, §16).
type Home struct {
	ctx    *ScreenCtx
	styles theme.Styles

	counts      sdk.Counts
	countsErr   error
	gotCounts   bool
	alerts      components.AlertList
	alertRows   []schema.Alert
	alertErr    string
	gotAlerts   bool
	geo         components.MapView
	geoActivity geoenrich.Result
	gotObs      bool
	obsCount    int
	stats       graph.Stats
	gotStats    bool
	statsErr    string
	recentTx    []schema.Transaction
	gotRecentTx bool
	recentTxErr string
	modelState  string
	geoNote     string
	tileRegionH int // adaptive tile-row region height (set per render in View)

	// subject is the open-subject input in the detail strip: pressing 'i'
	// focuses it; on submit the trimmed id is classified and a NavSearch is
	// emitted for a valid id, or an inline note is shown (no nav) otherwise
	// (design §B.3). subjectNote carries that inline note.
	subject     components.SearchBox
	subjectNote string
}

// NewHome builds the Dashboard screen bound to the shared app seam.
func NewHome(ctx *ScreenCtx) *Home {
	return &Home{
		ctx:        ctx,
		styles:     ctx.styles(),
		alerts:     components.NewAlertList(ctx.styles()),
		geo:        components.NewMapView(ctx.styles()),
		modelState: modelStatus(ctx),
		geoNote:    geoDBNote(),
		subject:    components.NewSearchBox(ctx.styles(), "open subject — wallet / txid / IP / entity"),
	}
}

// Focused reports whether the open-subject input owns the keyboard so the Root
// suppresses single-letter nav (incl. the 'i' focus key) while typing a subject
// (design §B.3 / §A.7).
func (h *Home) Focused() bool { return h.subject.Focused() }

// Init kicks off the live reads: local corpus counts, recent alerts, network
// observations (for the geo panel) and the persisted graph stats (graph panel).
func (h *Home) Init() tea.Cmd {
	repo := h.ctx.repo()
	if repo == nil {
		h.countsErr = errNoCase
		h.gotCounts = true
		h.alerts.SetAlerts(nil)
		h.gotAlerts = true
		h.gotObs = true
		h.statsErr = errNoCase.Error()
		h.gotStats = true
		return nil
	}
	return tea.Batch(
		countsCmd(h.ctx.bgCtx(), repo),
		alertsCmd(h.ctx.bgCtx(), repo, 10),
		allObservationsCmd(h.ctx.bgCtx(), repo, networkObsLimit),
		graphStatsCmd(h.ctx),
		recentTxCmd(h.ctx.bgCtx(), repo, recentTxLimit),
	)
}

// recentTxLimit bounds the dashboard activity feed (newest transactions).
const recentTxLimit = 12

// Update folds in the async service results.
func (h *Home) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		switch p := m.Payload.(type) {
		case sdk.Counts:
			h.counts, h.gotCounts, h.countsErr = p, true, nil
		case []schema.Alert:
			h.alerts.SetAlerts(p)
			h.alertRows, h.gotAlerts = p, true
		case []schema.NetworkObservation:
			h.geo.SetObservations(p)
			// Resolve the observations through the installed GeoIP DB (offline)
			// so the GEO panel can show a compact country/city activity summary
			// from the same pipeline the Geo Map uses.
			h.geoActivity = geoenrich.Enrich(p, 0)
			h.obsCount, h.gotObs = len(p), true
		case graph.Stats:
			h.stats, h.gotStats = p, true
		case recentTxResult:
			h.recentTx, h.gotRecentTx = []schema.Transaction(p), true
		}
	case dataError:
		switch m.Request {
		case "counts":
			h.countsErr, h.gotCounts = m.Err, true
		case "alerts":
			h.alerts.SetError(m.Err.Error())
			h.alertErr, h.gotAlerts = m.Err.Error(), true
		case "all-obs":
			h.gotObs = true
		case "graph-stats":
			h.statsErr, h.gotStats = m.Err.Error(), true
		case "recent-tx":
			h.recentTxErr, h.gotRecentTx = m.Err.Error(), true
		}
	case components.SearchSubmitted:
		// The open-subject box submitted: classify the trimmed id and either
		// emit a NavSearch (valid) or show an honest inline note (no nav).
		return h, h.submitSubject(m.Query)
	case tea.KeyMsg:
		// While the open-subject input is focused, editing keys go to it; Enter
		// submits via the SearchSubmitted case above and Esc blurs it.
		if h.subject.Focused() {
			var cmd tea.Cmd
			h.subject, cmd = h.subject.Update(m)
			return h, cmd
		}
		// 'i' focuses the open-subject input (design §B.3).
		if m.String() == "i" {
			h.subjectNote = ""
			return h, h.subject.Focus()
		}
		// Enter on the selected high-risk alert opens its subject: classify the
		// alert's Subject id and emit a NavSearch so the Dashboard drills into a
		// populated Wallet/Transaction screen (design §B.3 cross-link). Falls
		// through to the list when there is no selectable alert.
		if m.String() == "enter" {
			if al, ok := h.alerts.SelectedAlert(); ok {
				if kind, valid := classifySubject(al.Subject); valid {
					id := al.Subject
					return h, func() tea.Msg { return NavSearch{Kind: kind, ID: id} }
				}
			}
		}
		var cmd tea.Cmd
		h.alerts, cmd = h.alerts.Update(m)
		return h, cmd
	}
	// Non-key messages (textinput blink) still reach the subject box when it is
	// focused so the cursor keeps blinking.
	if h.subject.Focused() {
		var cmd tea.Cmd
		h.subject, cmd = h.subject.Update(msg)
		return h, cmd
	}
	return h, nil
}

// submitSubject classifies the trimmed open-subject id and emits a NavSearch
// for a structurally-valid id, or sets an honest inline note (no nav) for an
// empty or unclassifiable one (design §B.3). The box is blurred on any submit
// so focus returns to the dashboard.
func (h *Home) submitSubject(raw string) tea.Cmd {
	id := strings.TrimSpace(raw)
	h.subject.Blur()
	if id == "" {
		h.subjectNote = "enter a wallet, txid, IP, or entity id"
		return nil
	}
	kind, ok := classifySubject(id)
	if !ok {
		h.subjectNote = "unrecognized id — expected a wallet, 64-hex txid, or IP"
		return nil
	}
	h.subjectNote = ""
	h.subject.SetValue("")
	return func() tea.Msg { return NavSearch{Kind: kind, ID: id} }
}

// View composes the cockpit grid, reflowing by width breakpoint so panels tile
// in wide terminals and stack in compact ones. Nothing overflows the frame: the
// row budget is split explicitly and every region is clamped to its sub-frame.
func (h *Home) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}

	// Title row + bottom detail strip are fixed; the rest is split into the
	// tiles row, the middle (geo+graph) row and the lower (alerts+risk) row.
	title := h.styles.Title.Render("DASHBOARD — INTELLIGENCE COCKPIT")

	// Budget: 1 title row + the panel body + the bordered detail strip. Reserve
	// the strip rows up front so it is never clamped off the bottom; shrink the
	// strip if the frame is too short to hold the full bordered band.
	stripRows := stripH
	if stripRows > f.H-2 {
		stripRows = f.H - 2
	}
	if stripRows < 1 {
		stripRows = 1
	}
	strip := h.detailStrip(f.W, stripRows)

	bodyH := f.H - 1 - stripRows
	if bodyH < 1 {
		bodyH = 1
	}

	// The tile region height adapts to how many stacked tile rows the width
	// needs (narrow terminals wrap 6 tiles onto 2 rows), so tiles never clip.
	h.tileRegionH = components.TileRowsNeeded(f.W, len(h.tileSpecs())) * tileH
	if h.tileRegionH < tileH {
		h.tileRegionH = tileH
	}

	var body string
	switch dashBreakpoint(f.W) {
	case dashCompact:
		body = h.compact(components.Frame{W: f.W, H: bodyH})
	case dashStandard:
		body = h.standard(components.Frame{W: f.W, H: bodyH})
	default:
		body = h.wide(components.Frame{W: f.W, H: bodyH})
	}

	out := title + "\n" + body + "\n" + strip
	return clampBlockLocal(out, f)
}

// --- breakpoint layouts -----------------------------------------------------

// tileH / stripH are the fixed row budgets the bordered tiles and detail strip
// occupy; the remaining rows are divided between the two panel rows.
const (
	tileH  = 4 // bordered tile: border + label + value + border
	stripH = 5 // border + title + subject input + status + border
)

// dashClass is the dashboard's width class. The thresholds mirror the design §6
// breakpoint lower bounds (standard=100, wide=160) rather than ad-hoc literals,
// so the Home reflow and the shell layout agree on where columns tile vs stack.
type dashClass int

const (
	dashCompact dashClass = iota
	dashStandard
	dashWide
)

// Dashboard width thresholds (inclusive lower bounds), per design §6.
const (
	dashWidthStandard = 100
	dashWidthWide     = 160
)

// dashBreakpoint classifies a body width into its dashboard layout class.
func dashBreakpoint(w int) dashClass {
	switch {
	case w >= dashWidthWide:
		return dashWide
	case w >= dashWidthStandard:
		return dashStandard
	default:
		return dashCompact
	}
}

// twoColWidths returns the (left, right) column widths Row2 will produce for a
// body width and ratio, so each panel is sized by EXACTLY the width Row2 clamps
// it to (design §C.2): the sub-widths are driven by the single SplitH budget,
// never by local f.W/2 or f.W*55/100 literals that double-count the gap. When
// the frame cannot fit two floored columns it returns (f.W, 0) so the caller
// renders the left panel full-width (stacks).
func twoColWidths(w int, ratio float64) (left, right int) {
	l, r, ok := components.Row2Widths(w, ratio)
	if !ok {
		return w, 0
	}
	return l, r
}

// wide/ultrawide: full multi-panel grid — tiles row, geo|graph row, alerts|risk
// row. Each panel is sized by the width twoColWidths/Row2 agree on.
func (h *Home) wide(f components.Frame) string {
	th := h.tileRegionH
	rowsLeft := f.H - th
	if rowsLeft < 2 {
		return h.compact(f)
	}
	midH := rowsLeft / 2
	lowH := rowsLeft - midH

	tiles := components.TileRow(h.styles, h.tileSpecs(), components.Frame{W: f.W, H: th})

	// Middle row: GEO ACTIVITY | RECENT TRANSACTIONS (the live activity feed the
	// home page shows at a glance). Both are reliably populated for any case
	// with data, so neither column reads as an empty box.
	gL, gR := twoColWidths(f.W, 0.5)
	mid := components.Row2(components.Frame{W: f.W, H: midH}, 0.5,
		h.geoPanel(components.Frame{W: gL, H: midH}),
		h.recentTxPanel(components.Frame{W: gR, H: midH}),
	)
	// Lower row: RECENT HIGH-RISK ALERTS | GRAPH + RISK summary stacked.
	aL, aR := twoColWidths(f.W, 0.55)
	rightLowH := lowH
	grH := rightLowH / 2
	rkH := rightLowH - grH
	rightCol := components.StackV(components.Frame{W: aR, H: rightLowH},
		[]string{
			h.graphPanel(components.Frame{W: aR, H: grH}),
			h.riskPanel(components.Frame{W: aR, H: rkH}),
		}, []int{grH, rkH})
	low := components.Row2(components.Frame{W: f.W, H: lowH}, 0.55,
		h.alertsPanel(components.Frame{W: aL, H: lowH}),
		rightCol,
	)
	return components.StackV(f, []string{tiles, mid, low}, []int{th, midH, lowH})
}

// standard: tiles row, then two 2-up rows (geo|graph, alerts|risk) but tighter.
func (h *Home) standard(f components.Frame) string {
	th := h.tileRegionH
	rowsLeft := f.H - th
	if rowsLeft < 2 {
		return h.compact(f)
	}
	midH := rowsLeft / 2
	lowH := rowsLeft - midH

	tiles := components.TileRow(h.styles, h.tileSpecs(), components.Frame{W: f.W, H: th})

	gL, gR := twoColWidths(f.W, 0.5)
	mid := components.Row2(components.Frame{W: f.W, H: midH}, 0.5,
		h.geoPanel(components.Frame{W: gL, H: midH}),
		h.recentTxPanel(components.Frame{W: gR, H: midH}),
	)
	aL, aR := twoColWidths(f.W, 0.55)
	low := components.Row2(components.Frame{W: f.W, H: lowH}, 0.55,
		h.alertsPanel(components.Frame{W: aL, H: lowH}),
		h.graphPanel(components.Frame{W: aR, H: lowH}),
	)
	return components.StackV(f, []string{tiles, mid, low}, []int{th, midH, lowH})
}

// compact: stack the tiles (2-wide) and every panel full-width vertically.
func (h *Home) compact(f components.Frame) string {
	th := h.tileRegionH
	rowsLeft := f.H - th
	if rowsLeft < 3 {
		// Not enough height for panels: just the tiles.
		return components.TileRow(h.styles, h.tileSpecs(), components.Frame{W: f.W, H: f.H})
	}
	each := rowsLeft / 3
	gH := each
	grH := each
	aH := rowsLeft - gH - grH

	tiles := components.TileRow(h.styles, h.tileSpecs(), components.Frame{W: f.W, H: th})
	rtx := h.recentTxPanel(components.Frame{W: f.W, H: gH})
	geo := h.geoPanel(components.Frame{W: f.W, H: grH})
	al := h.alertsPanel(components.Frame{W: f.W, H: aH})
	return components.StackV(f, []string{tiles, rtx, geo, al},
		[]int{th, gH, grH, aH})
}

// --- panels (every value from a live seam) ----------------------------------

// tileSpecs builds the top metric row from Repo.Counts and the loaded alert
// rows. Zeros are honest for an empty case; nothing is fabricated.
func (h *Home) tileSpecs() []components.TileSpec {
	tx, addr, net, edges, alerts, high := "—", "—", "—", "—", "—", "—"
	if h.gotCounts && h.countsErr == nil {
		c := h.counts
		tx = fmt.Sprintf("%d", c.Transactions)
		addr = fmt.Sprintf("%d", c.Wallets)
		net = fmt.Sprintf("%d", c.NetworkRecs)
		edges = fmt.Sprintf("%d", c.Edges)
		alerts = fmt.Sprintf("%d", c.Alerts)
	}
	if h.gotAlerts {
		high = fmt.Sprintf("%d", h.highRiskCount())
	}
	alertRole := theme.RoleValue
	if h.gotCounts && h.counts.Alerts > 0 {
		alertRole = theme.RoleWarning
	}
	highRole := theme.RoleValue
	if h.highRiskCount() > 0 {
		highRole = theme.RoleCritical
	}
	// Short labels so six tiles fit a standard-width row without wrapping; the
	// Tile truncates anything that still would not fit (narrow terminals).
	return []components.TileSpec{
		{Label: "TXNS", Value: tx, Role: theme.RoleInfo},
		{Label: "ADDRS", Value: addr, Role: theme.RoleInfo},
		{Label: "NET REC", Value: net, Role: theme.RoleNeutral},
		{Label: "EDGES", Value: edges, Role: theme.RoleNeutral},
		{Label: "ALERTS", Value: alerts, Role: alertRole},
		{Label: "HI-RISK", Value: high, Role: highRole},
	}
}

// highRiskCount counts loaded alerts in the HIGH/CRITICAL presentation band.
// Derived from the live alert rows, never fabricated.
func (h *Home) highRiskCount() int {
	n := 0
	for _, a := range h.alertRows {
		// HIGH/CRITICAL presentation band mirrors the component's risk bands
		// (design §10): a cue only, never an assertion of wrongdoing.
		if a.Risk >= 50 {
			n++
		}
	}
	return n
}

// geoPanel shows a compact country/city activity summary (top countries by
// observation count) resolved from the installed GeoIP DB via the shared
// geoenrich pipeline, with an honest empty / NOT-INSTALLED state. The wording
// stays metadata-only: it reports IP geolocation estimates, never ownership.
func (h *Home) geoPanel(f components.Frame) string {
	var body string
	switch {
	case !h.gotObs:
		body = h.styles.Role(theme.RoleInfo).Render("querying…")
	case h.obsCount == 0:
		msg := "no network observations in this case"
		if h.geoNote != "" {
			msg += "\nGEOIP NOT INSTALLED — country/ASN labels only"
		}
		body = h.styles.Role(theme.RoleLabel).Render(msg)
	default:
		body = h.geoActivityBody(f.H - 3)
	}
	return components.Panel(h.styles, "GEO ACTIVITY — IP GEOLOCATION (ESTIMATE)", body, f)
}

// geoActivityBody renders the top-country observation summary from the enriched
// pipeline. When the City DB is installed it also shows the busiest city for
// the top country; wording is honest ("estimate", "observed from IP").
func (h *Home) geoActivityBody(rows int) string {
	if rows < 1 {
		rows = 1
	}
	r := h.geoActivity
	var b strings.Builder
	src := "country/ASN labels"
	if r.HasCity {
		src = "GeoIP City DB"
	} else if r.Installed {
		src = "GeoIP country DB"
	}
	b.WriteString(h.styles.Label.Render(fmt.Sprintf("%d IPs · %d obs · %s", r.DistinctIPs, r.TotalObservations, src)))
	b.WriteString("\n")
	if len(r.Countries) == 0 {
		b.WriteString(h.styles.Role(theme.RoleLabel).Render("no geolocatable IPs"))
		return b.String()
	}
	for i, c := range r.Countries {
		if i >= rows-1 {
			break
		}
		label := c.ISOCode
		if c.Country != "" {
			label = fmt.Sprintf("%s (%s)", c.Country, c.ISOCode)
		}
		line := fmt.Sprintf("%-22s %d obs · %d IP", clampLineLocal(label, 22), c.Observations, c.IPs)
		b.WriteString(h.styles.Role(theme.RoleValue).Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

// graphPanel summarizes the persisted transaction/entity graph (graph.Builder
// GraphStats) as an edge-type breakdown. Honest zeros/empty when no graph is
// built; the edge-type labels name real schema edges.
func (h *Home) graphPanel(f components.Frame) string {
	var body string
	switch {
	case !h.gotStats:
		body = h.styles.Role(theme.RoleInfo).Render("querying…")
	case h.statsErr != "":
		body = h.styles.Role(theme.RoleLabel).Render("no graph — build with :data / graph build")
	case h.stats.Edges == 0:
		body = h.styles.Role(theme.RoleLabel).Render("no transactions — acquire with :sync, then build the graph")
	default:
		var b strings.Builder
		b.WriteString(h.styles.Label.Render(fmt.Sprintf("%-16s", "total edges")))
		b.WriteString(" ")
		b.WriteString(h.styles.Value.Render(fmt.Sprintf("%d", h.stats.Edges)))
		b.WriteString("\n")
		// Deterministic edge-type order for a stable, dense readout.
		types := make([]string, 0, len(h.stats.ByType))
		for et := range h.stats.ByType {
			types = append(types, string(et))
		}
		sort.Strings(types)
		for _, et := range types {
			n := h.stats.ByType[schema.EdgeType(et)]
			b.WriteString(h.styles.Label.Render(fmt.Sprintf("%-16s", et)))
			b.WriteString(" ")
			b.WriteString(h.styles.Role(theme.RoleValue).Render(fmt.Sprintf("%d", n)))
			b.WriteString("\n")
		}
		body = b.String()
	}
	return components.Panel(h.styles, "TRANSACTION / ENTITY GRAPH", body, f)
}

// recentTxPanel renders the live activity feed: the newest transactions by
// timestamp (RecentTransactions), each as a compact one-line row — time, short
// txid, in/out counts, net BTC, fee. Honest empty/querying states; every value
// from the live repo, nothing fabricated. This is the "last N transactions"
// monitor the home page shows at a glance.
func (h *Home) recentTxPanel(f components.Frame) string {
	innerW, innerH := components.PanelInner(f)
	rows := innerH - 1 // title row
	if rows < 1 {
		rows = 1
	}
	var body string
	switch {
	case !h.gotRecentTx:
		body = h.styles.Role(theme.RoleInfo).Render("querying…")
	case h.recentTxErr != "":
		body = h.styles.Role(theme.RoleCritical).Render("error: " + h.recentTxErr)
	case len(h.recentTx) == 0:
		body = h.styles.Role(theme.RoleLabel).Render("no transactions yet — import a dataset or acquire with :sync")
	default:
		// Adapt the columns to the panel's inner width so a row never exceeds it
		// (which would wrap the value onto a second line). Wide: full columns;
		// narrow: drop the IN/OUT counts, then shorten.
		wide := innerW >= 46
		var b strings.Builder
		var hdr string
		if wide {
			hdr = fmt.Sprintf("%-14s %-13s %3s %3s %11s", "TIME", "TXID", "IN", "OUT", "VALUE")
		} else {
			hdr = fmt.Sprintf("%-14s %-13s %9s", "TIME", "TXID", "VALUE")
		}
		b.WriteString(h.styles.Label.Render(clampLineLocal(hdr, innerW)))
		b.WriteString("\n")
		shown := 0
		for _, tx := range h.recentTx {
			if shown >= rows-1 {
				break
			}
			ts := "—"
			if !tx.Timestamp.IsZero() {
				ts = tx.Timestamp.UTC().Format("01-02 15:04")
			}
			var out float64
			for _, o := range tx.Outputs {
				out += o.AmountBTC
			}
			var row string
			if wide {
				row = fmt.Sprintf("%-14s %-13s %3d %3d %11.4f",
					ts, shortTxID(tx.TxID), len(tx.Inputs), len(tx.Outputs), out)
			} else {
				row = fmt.Sprintf("%-14s %-13s %9.4f", ts, shortTxID(tx.TxID), out)
			}
			b.WriteString(h.styles.Role(theme.RoleValue).Render(clampLineLocal(row, innerW)))
			b.WriteString("\n")
			shown++
		}
		body = b.String()
	}
	return components.Panel(h.styles, "RECENT TRANSACTIONS — LIVE ACTIVITY", body, f)
}

// shortTxID abbreviates a txid for a dense one-line feed row.
func shortTxID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…" + id[len(id)-3:]
}

// alertsPanel wraps the recent high-risk alert list (ListAlerts) in a titled
// bordered box.
func (h *Home) alertsPanel(f components.Frame) string {
	// Size the list to the panel's TRUE inner area (same budget Panel uses), so
	// a row never exceeds the inner width and gets wrapped onto a second line by
	// the panel's clamp. One row is reserved for the title.
	innerW, innerH := components.PanelInner(f)
	bodyH := innerH - 1
	if bodyH < 1 {
		bodyH = 1
	}
	body := h.alerts.View(components.Frame{W: innerW, H: bodyH})
	return components.Panel(h.styles, "RECENT HIGH-RISK ACTIVITY", body, f)
}

// riskPanel renders a risk/anomaly summary + top-pattern breakdown from the
// loaded alert rows. Pattern "categories" are the alert Type values the backend
// produced — a real grouping, never an invented taxonomy.
func (h *Home) riskPanel(f components.Frame) string {
	var body string
	switch {
	case !h.gotAlerts:
		body = h.styles.Role(theme.RoleInfo).Render("querying…")
	case h.alertErr != "":
		body = h.styles.Role(theme.RoleCritical).Render("error: " + h.alertErr)
	case len(h.alertRows) == 0:
		body = h.styles.Role(theme.RoleLabel).Render("no alerts in this case")
	default:
		// Risk DISTRIBUTION: a compact tally line + a multi-colored bar per band
		// (CRITICAL/HIGH/ELEVATED/LOW), bucketed from the real alert rows. The
		// inner area reserves one row for the tally; the chart fills the rest.
		innerW, innerH := components.PanelInner(f)
		var b strings.Builder
		b.WriteString(h.styles.Label.Render(riskSummaryLine(h.alertRows)))
		b.WriteString("\n")
		chartH := innerH - 2 // title row + tally row
		if chartH < 4 {
			chartH = 4
		}
		b.WriteString(RenderRiskChart(h.styles, h.alertRows, components.Frame{W: innerW, H: chartH}))
		body = b.String()
	}
	return components.Panel(h.styles, "RISK DISTRIBUTION — ALERTS BY BAND", body, f)
}

// patternCount is a pattern category and its alert count.
type patternCount struct {
	k string
	n int
}

// topPatterns groups the loaded alerts by their Type (the backend's pattern
// label) and returns the top-N in deterministic order (count desc, label asc).
func (h *Home) topPatterns(limit int) []patternCount {
	if limit < 1 {
		limit = 1
	}
	counts := map[string]int{}
	for _, a := range h.alertRows {
		k := strings.TrimSpace(a.Type)
		if k == "" {
			k = "(unclassified)"
		}
		counts[k]++
	}
	out := make([]patternCount, 0, len(counts))
	for k, n := range counts {
		out = append(out, patternCount{k, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].k < out[j].k
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// detailStrip is the bottom case/system band: the open-subject input, the
// active case id and the live MODELS / GEOIP / NETWORK status, bordered to
// match the cockpit chrome. The subject input is the first inner row so 'i' has
// a visible target; the status line follows.
func (h *Home) detailStrip(w, rows int) string {
	caseID := nonEmpty(h.ctx.caseID(), "(no case)")
	net := "LOCAL / OFFLINE"
	parts := []string{
		h.styles.Label.Render("CASE ") + h.styles.Value.Render(caseID),
		h.styles.Label.Render("MODELS ") + h.styles.Role(theme.RoleInfo).Render(h.modelState),
		h.styles.Label.Render("GEOIP ") + h.styles.Role(theme.RoleInfo).Render(geoStatus(h.ctx)),
		h.styles.Label.Render("DATA ") + h.styles.Role(theme.RoleHealthy).Render(net),
	}
	status := strings.Join(parts, h.styles.Muted.Render("   ·   "))

	// Inner width available inside the bordered panel for the subject row.
	innerW, _ := components.PanelInner(components.Frame{W: w, H: rows})
	if innerW < 1 {
		innerW = 1
	}
	subjectRow := h.subject.View(components.Frame{W: innerW, H: 1})
	if h.subjectNote != "" {
		subjectRow = clampLineLocal(subjectRow, innerW) + "  " +
			h.styles.Role(theme.RoleWarning).Render(h.subjectNote)
	} else if !h.subject.Focused() {
		subjectRow = clampLineLocal(subjectRow, innerW) + "  " +
			h.styles.Muted.Render("press i to open a subject")
	}

	body := subjectRow + "\n" + status
	return components.Panel(h.styles, "CASE / SYSTEM", body,
		components.Frame{W: w, H: rows})
}

// ShortHelp lists the Dashboard's context keys.
func (h *Home) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "open subject")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open alert")),
	}
}

// --- small presentation helpers (shared across screens) --------------------

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// modelStatus resolves the live ML-manifest status for the Dashboard via the
// registry built with app.ModelsDir(cfg) (design §8). "PENDING" is gone.
func modelStatus(c *ScreenCtx) string {
	if c == nil {
		return modelsMissing
	}
	return ModelsStatus(c.Cfg)
}

// geoStatus reports the installed/missing GeoIP state for display (design §8).
func geoStatus(c *ScreenCtx) string {
	return GeoIPStatus()
}

var _ = app.NewRegistry
