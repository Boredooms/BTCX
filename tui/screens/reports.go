package screens

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Reports is the Reports screen (design §2). It lists the case's persisted
// reports (ListReports) and can build a new deterministic snapshot for the
// selected subject through reporting.Service (BuildSnapshot/Build). Build is
// local and never dials (design §12.3). Export/verify of a bundle to disk is a
// filesystem operation surfaced via the CLI; this screen builds + previews.
type Reports struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	// box is the inline subject input: press i to type a wallet/txid, Enter to
	// build a report for it — so the Reports page is self-contained (choose the
	// subject, build, view, format, export without leaving the page).
	box     components.SearchBox
	boxNote string
	table   components.Table
	detail  components.DetailPanel

	// result is the last-built report's InvestigationResult, the only subject
	// the LLM pane may narrate here. nil until a report is built.
	result *schema.InvestigationResult
	// summary holds the OPTIONAL local-LLM pane state (design §E.4).
	summary summaryPane

	// --- in-TUI report content viewer ------------------------------------
	// selectedID is the id of the currently-selected report row (for render +
	// export). selectedResult is its InvestigationResult (from the persisted
	// row's ResultJSON, or the just-built report), the input the renderer needs.
	selectedID     string
	selectedResult *schema.InvestigationResult
	// viewing toggles the rendered-content pane (v). viewFormat is the format
	// the body is rendered in (md / json / html text), cycled with f. viewLines
	// holds the rendered body split into lines; viewScroll is the top line.
	viewing    bool
	viewFormat schema.ReportFormat
	viewLines  []string
	viewScroll int
	viewErr    string
	exportMsg  string
}

// NewReports builds the Reports screen.
func NewReports(ctx *ScreenCtx) *Reports {
	cols := []table.Column{
		{Title: "report", Width: 24},
		{Title: "subject", Width: 18},
		{Title: "schema", Width: 12},
		{Title: "generated", Width: 20},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("no reports built in this case yet")
	return &Reports{
		ctx:        ctx,
		styles:     ctx.styles(),
		subject:    ctx.Subject.ID,
		box:        components.NewSearchBox(ctx.styles(), "wallet / txid to build a report for (press i)"),
		table:      tbl,
		detail:     components.NewDetailPanel(ctx.styles()),
		viewFormat: schema.FormatMarkdown,
	}
}

// Focused reports whether the inline subject input owns the keyboard, so the
// Root suppresses single-letter nav while the operator types a subject.
func (r *Reports) Focused() bool { return r.box.Focused() }

// Init lists existing reports.
func (r *Reports) Init() tea.Cmd {
	repo := r.ctx.repo()
	if repo == nil {
		r.table.SetError(errNoCase.Error())
		return nil
	}
	r.table.SetLoading()
	r.detail.SetSections([]components.Section{{Title: "preview", Rows: []components.Row{
		{Label: "subject", Value: nonEmpty(r.subject, "(none — set a subject to build)")},
		{Label: "action", Value: "press b to build a snapshot for the subject"},
	}}})
	return reportsCmd(r.ctx.bgCtx(), repo)
}

// Update folds in the report list and build results.
func (r *Reports) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		switch p := m.Payload.(type) {
		case []sdk.ReportRow:
			r.setReports(p)
		case reportRowResult:
			r.showPersisted(p.Row)
			// Capture the selected report's id + result so it can be rendered or
			// exported. The InvestigationResult is persisted as canonical JSON.
			r.selectedID = p.Row.ID
			r.selectedResult = parseReportResult(p.Row.ResultJSON)
			r.viewErr = ""
		case schema.Report:
			r.showBuilt(p)
			// Keep the built result so the optional LLM pane can narrate it, and
			// make it the current selection for the content viewer / export.
			res := p.Result
			r.result = &res
			r.selectedID = p.ID
			r.selectedResult = &res
			r.exportMsg = "built report " + shortID(p.ID) + " — rendering content…"
			// Refresh the list AND immediately render the built report into the
			// viewer so the operator SEES the report content right after pressing
			// b (not just a metadata line).
			var cmds []tea.Cmd
			if repo := r.ctx.repo(); repo != nil {
				cmds = append(cmds, reportsCmd(r.ctx.bgCtx(), repo))
			}
			cmds = append(cmds, reportRenderCmd(r.ctx, p.ID, res, r.viewFormat))
			return r, tea.Batch(cmds...)
		case reportContent:
			// Rendered report body arrived: load it into the scrollable viewer.
			r.viewLines = strings.Split(strings.ReplaceAll(p.Text, "\r\n", "\n"), "\n")
			r.viewScroll = 0
			r.viewing = true
			r.viewErr = ""
		}
	case components.RowSelected:
		// Selecting a listed report loads + previews its persisted metadata
		// (a bounded local read — never a rebuild). Enter on the table emits
		// RowSelected via the Table component.
		if repo := r.ctx.repo(); repo != nil && m.ID != "" {
			r.detail.SetLoading()
			return r, reportByIDCmd(r.ctx.bgCtx(), repo, m.ID)
		}
	case reportExported:
		r.exportMsg = "exported " + string(m.Format) + " -> " + m.Path
		r.viewErr = ""
	case components.SearchSubmitted:
		// The inline subject box submitted: set the subject and build a report
		// for it immediately (the common demo flow: type an address -> report).
		id := strings.TrimSpace(m.Query)
		r.box.Blur()
		if id == "" {
			r.boxNote = "enter a wallet or txid"
			return r, nil
		}
		if _, ok := classifySubject(id); !ok {
			r.boxNote = "unrecognized id — expected a wallet or 64-hex txid"
			return r, nil
		}
		r.boxNote = ""
		r.subject = id
		r.box.SetValue("")
		return r, r.build()
	case dataError:
		r.detail.SetError(m.Err.Error())
		r.viewErr = m.Err.Error()
	case SummaryLoaded:
		r.summary.acceptSummary(r.ctx, m)
		return r, nil
	case tea.KeyMsg:
		// While the inline subject box is focused, editing keys go to it; Enter
		// submits (SearchSubmitted case) and Esc blurs it.
		if r.box.Focused() {
			if m.String() == "esc" {
				r.box.Blur()
				return r, nil
			}
			var cmd tea.Cmd
			r.box, cmd = r.box.Update(m)
			return r, cmd
		}
		switch m.String() {
		case "i":
			// Focus the inline subject input so the operator can type a subject.
			r.boxNote = ""
			return r, r.box.Focus()
		case "b":
			return r, r.build()
		case "v":
			// Toggle the in-TUI rendered-content viewer for the selected report.
			return r, r.toggleView()
		case "f":
			// Cycle the view format (md -> json -> html text -> md) and re-render.
			if r.viewing {
				return r, r.cycleFormat()
			}
			return r, nil
		case "e":
			// Export the selected report to a file under the case dir (offline).
			return r, r.export()
		case "up", "k":
			if r.viewing {
				r.scrollBy(-1)
				return r, nil
			}
		case "down", "j":
			if r.viewing {
				r.scrollBy(1)
				return r, nil
			}
		case "pgup":
			if r.viewing {
				r.scrollBy(-10)
				return r, nil
			}
		case "pgdown":
			if r.viewing {
				r.scrollBy(10)
				return r, nil
			}
		case "L":
			// Explicit, on-demand local-LLM summary (design §E.4). NIT-4: with no
			// built report this is an honest no-op note, not a network call.
			return r, r.summary.requestExplain(r.ctx, r.result)
		}
		// When viewing, do NOT forward nav keys to the table (the viewer owns
		// up/down); otherwise drive the report list.
		if r.viewing {
			return r, nil
		}
		var cmd tea.Cmd
		r.table, cmd = r.table.Update(m)
		return r, cmd
	}
	// Non-key messages (textinput blink) reach the subject box while focused so
	// the cursor keeps blinking.
	if r.box.Focused() {
		var cmd tea.Cmd
		r.box, cmd = r.box.Update(msg)
		return r, cmd
	}
	return r, nil
}

// toggleView opens or closes the rendered-content viewer. Opening renders the
// report under the cursor (or the just-built one) in the current format via the
// offline reporting seam — no separate "select" step is needed: v acts on the
// highlighted row directly.
func (r *Reports) toggleView() tea.Cmd {
	if r.viewing {
		r.viewing = false
		return nil
	}
	// Prefer an already-loaded result (e.g. a just-built report); otherwise view
	// the report highlighted in the list by id.
	if r.selectedResult != nil {
		r.viewErr = ""
		r.exportMsg = "rendering report…"
		return reportRenderCmd(r.ctx, r.selectedID, *r.selectedResult, r.viewFormat)
	}
	id := r.table.SelectedID()
	if id == "" {
		r.viewErr = "no report to view — build one with b (or wait for the list to load)"
		return nil
	}
	r.selectedID = id
	r.viewErr = ""
	r.exportMsg = "rendering report " + shortID(id) + "…"
	return reportViewByIDCmd(r.ctx, id, r.viewFormat)
}

// cycleFormat advances the view format and re-renders the current report.
func (r *Reports) cycleFormat() tea.Cmd {
	switch r.viewFormat {
	case schema.FormatMarkdown:
		r.viewFormat = schema.FormatJSON
	case schema.FormatJSON:
		r.viewFormat = schema.FormatHTML
	default:
		r.viewFormat = schema.FormatMarkdown
	}
	if r.selectedResult != nil {
		return reportRenderCmd(r.ctx, r.selectedID, *r.selectedResult, r.viewFormat)
	}
	if r.selectedID != "" {
		return reportViewByIDCmd(r.ctx, r.selectedID, r.viewFormat)
	}
	return nil
}

// scrollBy moves the viewer window, clamped to the content bounds.
func (r *Reports) scrollBy(delta int) {
	r.viewScroll += delta
	if r.viewScroll < 0 {
		r.viewScroll = 0
	}
	max := len(r.viewLines) - 1
	if max < 0 {
		max = 0
	}
	if r.viewScroll > max {
		r.viewScroll = max
	}
}

// export writes the selected report to a file under the active case dir in the
// current view format (defaults to markdown) and reports the exact path. Fully
// offline: reporting renders locally and writes atomically.
func (r *Reports) export() tea.Cmd {
	format := r.viewFormat
	if format == "" {
		format = schema.FormatMarkdown
	}
	// Export the loaded result if present (just-built), else the highlighted row.
	id := r.selectedID
	if id == "" {
		id = r.table.SelectedID()
	}
	if r.selectedResult == nil && id == "" {
		r.exportMsg = ""
		r.viewErr = "no report to export — build one with b or highlight a row"
		return nil
	}
	r.viewErr = ""
	r.exportMsg = "exporting " + string(format) + "…"
	if r.selectedResult != nil {
		return reportExportCmd(r.ctx, id, *r.selectedResult, format)
	}
	return reportExportByIDCmd(r.ctx, id, format)
}

// reportExported is the result of an in-TUI export, carrying the written path.
type reportExported struct {
	Path   string
	Format schema.ReportFormat
}

// parseReportResult unmarshals a persisted report's canonical ResultJSON into an
// InvestigationResult, or returns nil on any error (honest: no fabricated data).
func parseReportResult(resultJSON string) *schema.InvestigationResult {
	if strings.TrimSpace(resultJSON) == "" {
		return nil
	}
	var res schema.InvestigationResult
	if err := json.Unmarshal([]byte(resultJSON), &res); err != nil {
		return nil
	}
	return &res
}

// build constructs a snapshot for the subject via the orchestrator (to produce
// an InvestigationResult) then reporting.Build. Local + deterministic.
func (r *Reports) build() tea.Cmd {
	if r.subject == "" {
		r.detail.SetError("no subject — select a wallet first, then build")
		return nil
	}
	if r.ctx.repo() == nil {
		r.detail.SetError(errNoCase.Error())
		return nil
	}
	r.detail.SetLoading()
	r.viewErr = ""
	r.exportMsg = "building report for " + shortID(r.subject) + " (features → ML → risk → evidence)…"
	// Analyze first; the report build command runs off the analysis result.
	return analyzeForReportCmd(r.ctx, r.subject)
}

func (r *Reports) setReports(rows []sdk.ReportRow) {
	trows := make([]table.Row, 0, len(rows))
	ids := make([]string, 0, len(rows))
	kinds := make([]string, 0, len(rows))
	for _, rr := range rows {
		trows = append(trows, table.Row{
			shortID(rr.ID),
			shortID(rr.Subject),
			nonEmpty(rr.ReportSchemaVersion, "report-v1"),
			rr.GeneratedAt,
		})
		ids = append(ids, rr.ID)
		kinds = append(kinds, "report")
	}
	r.table.SetRows(trows, ids, kinds)
}

// showPersisted previews a persisted report row selected from the list. All
// values come from the stored ReportRow (a local read), never recomputed.
func (r *Reports) showPersisted(row sdk.ReportRow) {
	r.detail.SetSections([]components.Section{{Title: "selected report", Rows: []components.Row{
		{Label: "id", Value: row.ID},
		{Label: "subject", Value: nonEmpty(row.Subject, "(none)")},
		{Label: "subject type", Value: nonEmpty(row.SubjectType, "—")},
		{Label: "schema", Value: nonEmpty(row.ReportSchemaVersion, "report-v1")},
		{Label: "generated at", Value: nonEmpty(row.GeneratedAt, "—")},
		{Label: "generated by", Value: nonEmpty(row.GeneratedBy, "—")},
		{Label: "snapshot sha256", Value: nonEmpty(shortID(row.SnapshotSHA256), "—")},
		{Label: "export", Value: "bctx report export " + row.ID + " (bundle to disk, offline)"},
	}}})
}

func (r *Reports) showBuilt(rep schema.Report) {
	r.detail.SetSections([]components.Section{{Title: "built report", Rows: []components.Row{
		{Label: "id", Value: rep.ID},
		{Label: "subject", Value: shortID(rep.Result.Subject)},
		{Label: "schema", Value: nonEmpty(rep.SchemaVersion, "report-v1")},
		{Label: "snapshot sha256", Value: shortID(rep.SnapshotSHA256) + " (deterministic)"},
		{Label: "generated by", Value: rep.GeneratedBy},
	}}})
}

// View renders the report list beside the preview/build detail.
func (r *Reports) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	title := r.styles.Title.Render("REPORTS")
	bodyH := f.H - 1
	if bodyH < 2 {
		bodyH = 2
	}
	// Reserve the lower portion for the optional LLM pane when it is active.
	paneH := 0
	if r.summary.active {
		paneH = bodyH / 3
		if paneH < 4 {
			paneH = 4
		}
		if paneH > bodyH-2 {
			paneH = bodyH - 2
		}
	}
	// mainH is split between the report-list panel and the preview panel. One
	// separator row sits between them; reserve it so the two bordered panels +
	// the separator sum to exactly mainH (no bottom clip).
	mainH := bodyH - paneH
	if mainH < 4 {
		mainH = 4
	}
	sep := 1
	usable := mainH - sep
	tblH := usable * 2 / 3
	if tblH < 2 {
		tblH = 2
	}
	detH := usable - tblH
	if detH < 2 {
		detH = 2
	}
	var b strings.Builder
	b.WriteString(title)
	// A status line for export/view errors so the operator gets honest feedback.
	if r.viewErr != "" {
		b.WriteString(" " + r.styles.Role(theme.RoleCritical).Render(clampLineLocal(r.viewErr, f.W-10)))
	} else if r.exportMsg != "" {
		b.WriteString(" " + r.styles.Role(theme.RoleHealthy).Render(clampLineLocal(r.exportMsg, f.W-10)))
	}
	b.WriteString("\n")
	// SUBJECT input row: lets the operator choose which wallet/txid to report on
	// from within the page (press i to focus, Enter to build). It is one row so
	// the panels below keep their height budget (no chop).
	subjRow := r.box.View(components.Frame{W: f.W, H: 1})
	hint := r.boxNote
	if hint == "" && !r.box.Focused() {
		hint = "press i to set a subject · b build · v view · f format · e export"
	}
	if hint != "" {
		subjRow = clampLineLocal(subjRow, f.W/2) + "  " + r.styles.Muted.Render(hint)
	}
	b.WriteString(clampLineLocal(subjRow, f.W))
	b.WriteString("\n")
	tblH--
	if tblH < 2 {
		tblH = 2
	}
	// Both the list and the lower pane are bordered, height-filling panels so the
	// screen reads as a solid instrument panel with no empty cut-off region.
	b.WriteString(components.Panel(r.styles, "REPORTS — i SUBJECT · b BUILD · v VIEW · e EXPORT",
		r.table.View(components.Frame{W: f.W - 2, H: paneInner(tblH)}),
		components.Frame{W: f.W, H: tblH}))
	b.WriteString("\n")
	if r.viewing {
		b.WriteString(components.Panel(r.styles, r.viewTitle(),
			r.viewBody(components.Frame{W: f.W - 2, H: paneInner(detH)}),
			components.Frame{W: f.W, H: detH}))
	} else {
		b.WriteString(components.Panel(r.styles, "PREVIEW",
			r.detail.View(components.Frame{W: f.W - 2, H: paneInner(detH)}),
			components.Frame{W: f.W, H: detH}))
	}
	if paneH > 0 {
		b.WriteString("\n")
		b.WriteString(renderSummaryPane(r.styles, r.summary, components.Frame{W: f.W, H: paneH}))
	}
	return clampBlockLocal(b.String(), f)
}

// viewTitle is the content-viewer panel title, showing the format and a scroll
// hint so the operator knows what they are looking at.
func (r *Reports) viewTitle() string {
	fmtName := strings.ToUpper(string(r.viewFormat))
	pos := ""
	if len(r.viewLines) > 0 {
		pos = fmt.Sprintf(" · line %d/%d", r.viewScroll+1, len(r.viewLines))
	}
	return "REPORT CONTENT [" + fmtName + "]" + pos + " · f FORMAT · ↑↓ SCROLL · v CLOSE"
}

// viewBody renders the scrollable slice of the rendered report body into the
// frame. Lines are clamped to the width; the window starts at viewScroll.
func (r *Reports) viewBody(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if len(r.viewLines) == 0 {
		return r.styles.Muted.Render("rendering…")
	}
	rows := f.H
	if rows < 1 {
		rows = 1
	}
	start := r.viewScroll
	if start > len(r.viewLines)-1 {
		start = len(r.viewLines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + rows
	if end > len(r.viewLines) {
		end = len(r.viewLines)
	}
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(clampLineLocal(r.viewLines[i], f.W))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ShortHelp lists Reports' context keys.
func (r *Reports) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "set subject")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "build report")),
		key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view content")),
		key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "format json/md/html")),
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export to file")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "explain (local)")),
	}
}

var _ = fmt.Sprintf
