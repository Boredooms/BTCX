package screens

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Data is the Data / Ingestion screen (design §2). It lists the case's datasets
// (ListDatasets) and the persisted graph build statistics (graph.Builder
// GraphStats) in a detail pane. The sync action is network-touching: the screen
// calls the offline gate app.AcquisitionAllowed(cfg) FIRST (design §12 invariant
// 1, acceptance §19.10/§19.13) and blocks honestly when offline; it never
// constructs a provider inside the TUI (that would pull HTTP transport into
// tui/), so an allowed sync is launched via the CLI.
type Data struct {
	ctx    *ScreenCtx
	styles theme.Styles

	table  components.Table
	detail components.DetailPanel
	notice string

	// fileBox is the inline LOCAL-FILE import input: press f, type a dataset
	// path (CSV/JSON/NDJSON/XML), Enter to import it OFFLINE through the
	// existing ingestion pipeline. importing marks an in-flight import so the
	// UI shows progress; importMsg is the honest result/err line.
	fileBox   components.SearchBox
	importing bool
	importMsg string
}

// NewData builds the Data screen.
func NewData(ctx *ScreenCtx) *Data {
	cols := []table.Column{
		{Title: "dataset", Width: 20},
		{Title: "format", Width: 8},
		{Title: "records", Width: 10},
		{Title: "txs", Width: 8},
		{Title: "status", Width: 10},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("no datasets imported in this case")
	return &Data{
		ctx:     ctx,
		styles:  ctx.styles(),
		table:   tbl,
		detail:  components.NewDetailPanel(ctx.styles()),
		fileBox: components.NewSearchBox(ctx.styles(), "local dataset path (CSV/JSON/NDJSON/XML) — press f, Enter to import offline"),
	}
}

// Focused reports whether the local-file input owns the keyboard so the Root
// suppresses single-letter nav while the operator types a path.
func (d *Data) Focused() bool { return d.fileBox.Focused() }

// Init loads datasets and the graph stats.
func (d *Data) Init() tea.Cmd {
	repo := d.ctx.repo()
	if repo == nil {
		d.table.SetError(errNoCase.Error())
		d.detail.SetError(errNoCase.Error())
		return nil
	}
	d.table.SetLoading()
	d.detail.SetLoading()
	return tea.Batch(
		datasetsCmd(d.ctx.bgCtx(), repo),
		graphStatsCmd(d.ctx),
	)
}

// Update folds in datasets and graph stats and handles sync gating.
func (d *Data) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		switch p := m.Payload.(type) {
		case []schema.Dataset:
			d.setDatasets(p)
		case graph.Stats:
			d.setStats(p)
		}
	case localImportResult:
		d.importing = false
		if m.Err != "" {
			d.importMsg = "import failed: " + m.Err + "  (no network used)"
			return d, nil
		}
		d.importMsg = fmt.Sprintf("imported %s [%s]: read=%d accepted=%d txs=%d obs=%d dup=%d (sha %s)",
			shortID(m.Path), m.Format, m.RecordsRead, m.Accepted, m.Transactions, m.NetworkObs, m.Duplicates, shortID(m.SHA256))
		// Refresh the dataset list + graph stats so the import shows immediately.
		if repo := d.ctx.repo(); repo != nil {
			return d, tea.Batch(datasetsCmd(d.ctx.bgCtx(), repo), graphStatsCmd(d.ctx))
		}
		return d, nil
	case components.SearchSubmitted:
		// The local-file input submitted: import the path OFFLINE via the
		// existing ingestion pipeline. No network, ever.
		d.fileBox.Blur()
		path := strings.TrimSpace(m.Query)
		if path == "" {
			d.importMsg = "enter a dataset path (CSV/JSON/NDJSON/XML)"
			return d, nil
		}
		d.importing = true
		d.importMsg = "importing " + path + " … (offline)"
		return d, importLocalFileCmd(d.ctx, path)
	case dataError:
		if m.Request == "graph-stats" {
			d.detail.SetError(m.Err.Error())
		} else {
			d.table.SetError(m.Err.Error())
		}
	case tea.KeyMsg:
		// While the local-file input is focused, editing keys go to it; Enter
		// submits (SearchSubmitted) and Esc blurs it.
		if d.fileBox.Focused() {
			if m.String() == "esc" {
				d.fileBox.Blur()
				return d, nil
			}
			var cmd tea.Cmd
			d.fileBox, cmd = d.fileBox.Update(m)
			return d, cmd
		}
		switch m.String() {
		case "f":
			d.importMsg = ""
			return d, d.fileBox.Focus()
		case "y":
			return d, d.sync()
		}
		var cmd tea.Cmd
		d.table, cmd = d.table.Update(m)
		return d, cmd
	}
	// Non-key messages (textinput blink) reach the file box while focused.
	if d.fileBox.Focused() {
		var cmd tea.Cmd
		d.fileBox, cmd = d.fileBox.Update(msg)
		return d, cmd
	}
	return d, nil
}

// sync enforces the offline gate before any provider construction (design §12).
func (d *Data) sync() tea.Cmd {
	if err := d.ctx.acquisitionAllowed(); err != nil {
		if errors.Is(err, acquisition.ErrOfflineAcquisition) {
			d.notice = "ACQUISITION BLOCKED — offline/airgapped: sync is unavailable. Local analysis still works."
		} else {
			d.notice = "cannot sync: " + err.Error()
		}
		return nil
	}
	d.notice = "Sync requires a connected provider — run it with: bctx sync wallet <addr>. " +
		"This view shows imported datasets and the built graph."
	return nil
}

func (d *Data) setDatasets(ds []schema.Dataset) {
	rows := make([]table.Row, 0, len(ds))
	ids := make([]string, 0, len(ds))
	kinds := make([]string, 0, len(ds))
	for _, s := range ds {
		rows = append(rows, table.Row{
			shortID(s.ID),
			nonEmpty(s.Format, "—"),
			fmt.Sprintf("%d", s.RecordsRead),
			fmt.Sprintf("%d", s.Transactions),
			nonEmpty(s.Status, "—"),
		})
		ids = append(ids, s.ID)
		kinds = append(kinds, "dataset")
	}
	d.table.SetRows(rows, ids, kinds)
}

func (d *Data) setStats(st graph.Stats) {
	rows := []components.Row{
		{Label: "edges", Value: fmt.Sprintf("%d", st.Edges)},
	}
	for et, n := range st.ByType {
		rows = append(rows, components.Row{Label: string(et), Value: fmt.Sprintf("%d", n)})
	}
	d.detail.SetSections([]components.Section{{Title: "graph build", Rows: rows}})
}

// View renders the datasets table beside the graph-stats detail.
func (d *Data) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(d.styles.Title.Render("DATA / INGESTION"))
	b.WriteString("\n")
	// LOCAL-FILE import row: type a dataset path (f) and Enter to import offline.
	fileRow := d.fileBox.View(components.Frame{W: f.W, H: 1})
	hint := d.importMsg
	if hint == "" && !d.fileBox.Focused() {
		hint = "press f to import a LOCAL dataset file (offline) · y sync (gated)"
	}
	if hint != "" {
		role := theme.RoleInfo
		if strings.HasPrefix(d.importMsg, "import failed") {
			role = theme.RoleCritical
		} else if strings.HasPrefix(d.importMsg, "imported ") {
			role = theme.RoleHealthy
		}
		fileRow = clampLineLocal(fileRow, f.W/2) + "  " + d.styles.Role(role).Render(clampLineLocal(hint, f.W/2-2))
	}
	b.WriteString(clampLineLocal(fileRow, f.W))
	b.WriteString("\n")
	if d.notice != "" {
		b.WriteString(d.styles.Role(theme.RoleWarning).Render(d.notice))
		b.WriteString("\n")
	}
	used := strings.Count(b.String(), "\n")
	remaining := f.H - used
	if remaining < 4 {
		remaining = 4
	}
	// Two bordered, height-filling panels + one separator row between them.
	sep := 1
	usable := remaining - sep
	tblH := usable * 2 / 3
	if tblH < 2 {
		tblH = 2
	}
	detH := usable - tblH
	if detH < 2 {
		detH = 2
	}
	b.WriteString(components.Panel(d.styles, "DATASETS",
		d.table.View(components.Frame{W: f.W - 2, H: paneInner(tblH)}),
		components.Frame{W: f.W, H: tblH}))
	b.WriteString("\n")
	b.WriteString(components.Panel(d.styles, "GRAPH BUILD",
		d.detail.View(components.Frame{W: f.W - 2, H: paneInner(detH)}),
		components.Frame{W: f.W, H: detH}))
	return clampBlockLocal(b.String(), f)
}

// ShortHelp lists Data's context keys.
func (d *Data) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "import local file (offline)")),
		key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "sync (gated)")),
	}
}
