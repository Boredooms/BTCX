package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/extensions"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Extensions is the Extension / Knowledge Manager screen (ministry-extensible,
// offline). It lists the extensions installed under ~/.bctx/extensions and lets
// an operator enable/disable them and see how to load a demo example. An
// extension is a self-describing local package a deployment drops in to bring
// its OWN knowledge bases, model packs, or demo-example datasets into the
// air-gapped workstation.
//
// The screen is PRESENTATION + INTERACTION only (AGENTS §12): discovery and
// enable/disable go through the leaf extensions.Store (filesystem only, no
// network, no SQL). Loading a demo-example's dataset into a case is a dataset
// import, which the TUI must NOT run itself (that would pull the ingestion
// package into the transport-confined tui closure); instead the screen surfaces
// the exact, copy-pasteable `bctx extension load <id>` command — mirroring how
// the Data screen surfaces `bctx sync` rather than dialing a provider inline.
type Extensions struct {
	ctx    *ScreenCtx
	styles theme.Styles

	store  *extensions.Store
	table  components.Table
	detail components.DetailPanel

	exts    []extensions.Extension
	byID    map[string]extensions.Extension
	counts  extensions.Counts
	loadErr string
	notice  string
}

// NewExtensions builds the Extension / Knowledge Manager screen.
func NewExtensions(ctx *ScreenCtx) *Extensions {
	cols := []table.Column{
		{Title: "kind", Width: 10},
		{Title: "name", Width: 26},
		{Title: "version", Width: 9},
		{Title: "status", Width: 9},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("no extensions installed — drop a package under " + extensionsDirHint(ctx))
	e := &Extensions{
		ctx:    ctx,
		styles: ctx.styles(),
		store:  extensions.NewStore(extensionsDir(ctx)),
		table:  tbl,
		detail: components.NewDetailPanel(ctx.styles()),
		byID:   map[string]extensions.Extension{},
	}
	return e
}

// Init loads the installed extensions from disk (offline).
func (e *Extensions) Init() tea.Cmd {
	e.reload()
	return nil
}

// reload re-scans the extensions directory and refreshes the table + detail.
func (e *Extensions) reload() {
	exts, err := e.store.List()
	if err != nil {
		e.table.SetError(err.Error())
		e.detail.SetError(err.Error())
		return
	}
	e.exts = exts
	e.counts = extensions.Summarize(exts)
	e.byID = map[string]extensions.Extension{}
	rows := make([]table.Row, 0, len(exts))
	ids := make([]string, 0, len(exts))
	kinds := make([]string, 0, len(exts))
	for _, x := range exts {
		e.byID[x.ID] = x
		rows = append(rows, table.Row{
			x.Manifest.Kind.Label(),
			clampLineLocal(nonEmpty(x.Manifest.Name, x.ID), 26),
			nonEmpty(x.Manifest.Version, "—"),
			string(x.Status),
		})
		ids = append(ids, x.ID)
		kinds = append(kinds, "extension")
	}
	e.table.SetRows(rows, ids, kinds)
	e.refreshDetail()
}

// selected returns the currently highlighted extension, if any.
func (e *Extensions) selected() (extensions.Extension, bool) {
	if len(e.exts) == 0 {
		return extensions.Extension{}, false
	}
	i := e.table.Cursor()
	if i < 0 || i >= len(e.exts) {
		return extensions.Extension{}, false
	}
	return e.exts[i], true
}

// refreshDetail rebuilds the detail pane from the selected extension.
func (e *Extensions) refreshDetail() {
	x, ok := e.selected()
	if !ok {
		e.detail.SetSections([]components.Section{{
			Title: "no extension selected",
			Rows: []components.Row{
				{Label: "install", Value: "put a package dir under " + e.store.Root()},
				{Label: "manifest", Value: "each needs a manifest.json (name, version, kind)"},
				{Label: "kinds", Value: "knowledge-base · model-pack · demo-example"},
			},
		}})
		return
	}
	m := x.Manifest
	info := []components.Row{
		{Label: "id", Value: x.ID},
		{Label: "name", Value: nonEmpty(m.Name, "—")},
		{Label: "kind", Value: string(m.Kind)},
		{Label: "version", Value: nonEmpty(m.Version, "—")},
		{Label: "publisher", Value: nonEmpty(m.Publisher, "—")},
		{Label: "status", Value: string(x.Status), Role: statusRole(x.Status)},
	}
	if m.Description != "" {
		info = append(info, components.Row{Label: "description", Value: m.Description})
	}
	if x.Err != "" {
		info = append(info, components.Row{Label: "error", Value: x.Err, Role: theme.RoleCritical})
	}
	secs := []components.Section{{Title: "extension", Rows: info}}

	// Payload section per kind.
	switch m.Kind {
	case extensions.KindDemoExample:
		payload := []components.Row{
			{Label: "dataset", Value: nonEmpty(m.Dataset, "—")},
			{Label: "subject", Value: nonEmpty(m.Subject, "—")},
		}
		if m.Geo != "" {
			payload = append(payload, components.Row{Label: "geo", Value: m.Geo})
		}
		payload = append(payload, components.Row{
			Label: "load", Value: "bctx extension load " + x.ID, Role: theme.RoleInfo})
		secs = append(secs, components.Section{Title: "demo example", Rows: payload})
	case extensions.KindModelPack:
		secs = append(secs, components.Section{Title: "model pack", Rows: []components.Row{
			{Label: "model_dir", Value: nonEmpty(m.ModelDir, "—")},
			{Label: "note", Value: "enable, then load with: bctx extension load " + x.ID},
		}})
	case extensions.KindKnowledgeBase:
		secs = append(secs, components.Section{Title: "knowledge base", Rows: []components.Row{
			{Label: "docs", Value: nonEmpty(m.Docs, "—")},
		}})
	}
	e.detail.SetSections(secs)
}

// Update handles selection movement and enable/disable/load actions.
func (e *Extensions) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case components.RowSelected:
		// Enter is used to load (demo-example) or toggle; handled via keys below.
		return e, nil
	case tea.KeyMsg:
		switch m.String() {
		case "e":
			return e, e.setEnabled(true)
		case "x":
			return e, e.setEnabled(false)
		case "l":
			return e, e.loadSelected()
		case "r":
			e.notice = "re-scanned " + e.store.Root()
			e.reload()
			return e, nil
		}
		var cmd tea.Cmd
		e.table, cmd = e.table.Update(m)
		e.refreshDetail()
		return e, cmd
	}
	return e, nil
}

// setEnabled toggles the selected extension's enabled marker (offline).
func (e *Extensions) setEnabled(enabled bool) tea.Cmd {
	x, ok := e.selected()
	if !ok {
		e.notice = "no extension selected"
		return nil
	}
	if _, err := e.store.SetEnabled(x.ID, enabled); err != nil {
		e.loadErr = err.Error()
		e.notice = ""
		return nil
	}
	e.loadErr = ""
	if enabled {
		e.notice = "enabled " + x.ID
	} else {
		e.notice = "disabled " + x.ID
	}
	e.reload()
	return nil
}

// loadSelected surfaces how to load the selected extension into a case. The TUI
// does not run the import itself (transport/ingestion confinement); it shows the
// exact offline command so the operator loads it deterministically.
func (e *Extensions) loadSelected() tea.Cmd {
	x, ok := e.selected()
	if !ok {
		e.notice = "no extension selected"
		return nil
	}
	if x.Status == extensions.StatusError {
		e.loadErr = "cannot load " + x.ID + ": " + x.Err
		e.notice = ""
		return nil
	}
	e.loadErr = ""
	e.notice = "load this extension offline with:  bctx extension load " + x.ID
	return nil
}

// View renders the installed-extension table beside the detail pane.
func (e *Extensions) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(e.styles.Title.Render("EXTENSIONS / KNOWLEDGE MANAGER"))
	b.WriteString(" ")
	b.WriteString(e.styles.Label.Render(fmt.Sprintf(
		"%d installed · %d enabled · %d knowledge · %d models · %d demos",
		e.counts.Total, e.counts.Enabled, e.counts.Knowledge, e.counts.Models, e.counts.Demos)))
	b.WriteString("\n")
	if e.loadErr != "" {
		b.WriteString(e.styles.Role(theme.RoleCritical).Render(clampLineLocal(e.loadErr, f.W)))
		b.WriteString("\n")
	} else if e.notice != "" {
		b.WriteString(e.styles.Role(theme.RoleInfo).Render(clampLineLocal(e.notice, f.W)))
		b.WriteString("\n")
	}
	used := strings.Count(b.String(), "\n")
	bodyH := f.H - used
	if bodyH < 4 {
		bodyH = 4
	}

	// Split: installed list (left) | selected detail (right).
	split := components.SplitView{Ratio: 0.5, Compact: f.W < 100}
	lf, rf := split.Frames(components.Frame{W: f.W, H: bodyH})
	lcw, lch := components.PanelInner(lf)
	rcw, rch := components.PanelInner(rf)
	left := components.Panel(e.styles, "INSTALLED EXTENSIONS",
		e.table.View(components.Frame{W: lcw, H: lch - 1}), lf)
	right := components.Panel(e.styles, "DETAIL",
		e.detail.View(components.Frame{W: rcw, H: rch - 1}), rf)
	joined := split.Join(components.Frame{W: f.W, H: bodyH}, left, right)
	b.WriteString(joined)
	return clampBlockLocal(b.String(), f)
}

// ShortHelp lists the Extension manager's context keys.
func (e *Extensions) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "enable")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "disable")),
		key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "load (offline)")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rescan")),
	}
}

// statusRole maps an extension status to a semantic color role.
func statusRole(s extensions.Status) theme.Role {
	switch s {
	case extensions.StatusEnabled:
		return theme.RoleHealthy
	case extensions.StatusError:
		return theme.RoleCritical
	default:
		return theme.RoleNeutral
	}
}

// extensionsDir resolves the extensions root (~/.bctx/extensions) from the
// active config, defaulting to the standard layout when cfg is nil (tests).
func extensionsDir(ctx *ScreenCtx) string {
	cfg := configs.Default()
	if ctx != nil && ctx.Cfg != nil {
		cfg = ctx.Cfg
	}
	return extensionsRoot(cfg)
}

func extensionsDirHint(ctx *ScreenCtx) string { return extensionsDir(ctx) }

// extensionsRoot resolves the extensions directory from the runtime layout, so
// the TUI and the `bctx extension` CLI agree on one location.
func extensionsRoot(cfg *configs.Config) string {
	return configs.NewLayout(cfg).Extensions
}
