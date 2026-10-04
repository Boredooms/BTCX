package components

import (
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// DataState is the four explicit states every data region cycles through
// (design §11.1). Nothing silently fails: empty says empty, error surfaces.
type DataState int

const (
	StateLoading DataState = iota
	StateEmpty
	StateError
	StateLoaded
)

// RowSelected is emitted when the user activates a row. Kind lets the parent
// screen route the selection (e.g. open a wallet vs a transaction).
type RowSelected struct {
	ID   string
	Kind string
}

// Table wraps bubbles/table with the four-state UX and BCTX styling. It is
// paginated by the underlying model and column widths reflow by frame width.
// Rows are supplied by the parent (from a bounded repository read); the Table
// never fetches data itself (design §11.4).
type Table struct {
	styles theme.Styles
	model  table.Model
	state  DataState
	errMsg string
	empty  string
	// ids parallels the visible rows so RowSelected can carry a stable id.
	ids   []string
	kinds []string
}

// NewTable builds a Table with the given columns.
func NewTable(styles theme.Styles, cols []table.Column) Table {
	m := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
	)
	return Table{styles: styles, model: m, state: StateLoading, empty: "no rows"}
}

// SetLoading / SetError / SetEmpty move the component into an explicit state.
func (t *Table) SetLoading()             { t.state = StateLoading }
func (t *Table) SetError(msg string)     { t.state, t.errMsg = StateError, msg }
func (t *Table) SetEmptyText(msg string) { t.empty = msg }

// SetRows loads rows with their parallel ids/kinds. An empty set moves the
// component to the empty state (honest, never fabricated rows).
func (t *Table) SetRows(rows []table.Row, ids, kinds []string) {
	t.model.SetRows(rows)
	t.ids = ids
	t.kinds = kinds
	if len(rows) == 0 {
		t.state = StateEmpty
		return
	}
	t.state = StateLoaded
}

// State returns the current data state.
func (t Table) State() DataState { return t.state }

// Cursor returns the selected row index.
func (t Table) Cursor() int { return t.model.Cursor() }

// SelectedID returns the stable id of the currently-highlighted row (the cursor
// position), or "" when the table has no loaded rows. It lets a screen act on
// the highlighted row (e.g. view/export the report under the cursor) WITHOUT
// requiring a separate Enter press.
func (t Table) SelectedID() string {
	if t.state != StateLoaded {
		return ""
	}
	idx := t.model.Cursor()
	if idx < 0 || idx >= len(t.ids) {
		return ""
	}
	return t.ids[idx]
}

// SelectedKind returns the kind of the currently-highlighted row, or "".
func (t Table) SelectedKind() string {
	if t.state != StateLoaded {
		return ""
	}
	idx := t.model.Cursor()
	if idx < 0 || idx >= len(t.kinds) {
		return ""
	}
	return t.kinds[idx]
}

// Init implements the sub-model contract.
func (t Table) Init() tea.Cmd { return nil }

// Update forwards messages to the underlying table and emits RowSelected on
// Enter when loaded.
func (t Table) Update(msg tea.Msg) (Table, tea.Cmd) {
	if t.state != StateLoaded {
		return t, nil
	}
	switch m := msg.(type) {
	case tea.KeyMsg:
		if m.String() == "enter" {
			idx := t.model.Cursor()
			if idx >= 0 && idx < len(t.ids) {
				id := t.ids[idx]
				kind := ""
				if idx < len(t.kinds) {
					kind = t.kinds[idx]
				}
				return t, func() tea.Msg { return RowSelected{ID: id, Kind: kind} }
			}
		}
	}
	var cmd tea.Cmd
	t.model, cmd = t.model.Update(msg)
	return t, cmd
}

// View renders the current state within the frame.
func (t Table) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	switch t.state {
	case StateLoading:
		return clampBlock(t.styles.Role(theme.RoleInfo).Render("querying…"), f)
	case StateEmpty:
		return clampBlock(t.styles.Role(theme.RoleLabel).Render(t.empty), f)
	case StateError:
		return clampBlock(t.styles.Role(theme.RoleCritical).Render("error: "+t.errMsg), f)
	default:
		// Size the underlying table to the frame before rendering.
		m := t.model
		m.SetWidth(f.W)
		m.SetHeight(f.H)
		return clampBlock(m.View(), f)
	}
}
