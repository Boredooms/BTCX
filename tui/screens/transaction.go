package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Transaction is the Transaction inspection screen (design §2). It loads the
// canonical transaction (GetTransaction) and its incident graph edges
// (EdgesFrom / EdgesTo) and renders inputs, outputs, and the edge summary. All
// data is from the repository; nothing is computed here.
type Transaction struct {
	ctx    *ScreenCtx
	styles theme.Styles

	txid   string
	detail components.DetailPanel

	tx    *schema.Transaction
	edges edgesResult
}

// NewTransaction builds the Transaction screen for the current subject.
func NewTransaction(ctx *ScreenCtx) *Transaction {
	return &Transaction{
		ctx:    ctx,
		styles: ctx.styles(),
		txid:   ctx.Subject.ID,
		detail: components.NewDetailPanel(ctx.styles()),
	}
}

// Init loads the transaction and its edges.
func (t *Transaction) Init() tea.Cmd {
	if t.txid == "" {
		t.detail.SetError("no transaction selected")
		return nil
	}
	repo := t.ctx.repo()
	if repo == nil {
		t.detail.SetError(errNoCase.Error())
		return nil
	}
	t.detail.SetLoading()
	return tea.Batch(
		transactionCmd(t.ctx.bgCtx(), repo, t.txid),
		edgesCmd(t.ctx.bgCtx(), repo, t.txid),
	)
}

// Update folds in the transaction and edges.
func (t *Transaction) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != t.txid {
			return t, nil
		}
		switch p := m.Payload.(type) {
		case *schema.Transaction:
			t.tx = p
		case edgesResult:
			t.edges = p
		}
		t.rebuild()
	case dataError:
		if m.Request == t.txid {
			t.detail.SetError(m.Err.Error())
		}
	case tea.KeyMsg:
		if m.String() == "g" && t.txid != "" {
			// Cross-link to the Graph scoped to this transaction.
			id := t.txid
			return t, func() tea.Msg {
				return NavScreen{Screen: "graph", Subject: Subject{ID: id, Kind: SubjectTx}}
			}
		}
		var cmd tea.Cmd
		t.detail, cmd = t.detail.Update(m)
		return t, cmd
	}
	return t, nil
}

func (t *Transaction) rebuild() {
	if t.tx == nil {
		// GetTransaction returned nil (not found) and no error -> honest empty.
		t.detail.SetSections([]components.Section{{
			Title: "transaction",
			Rows:  []components.Row{{Label: "status", Value: "not found in active case"}},
		}})
		return
	}
	tx := t.tx
	meta := components.Section{Title: "transaction", Rows: []components.Row{
		{Label: "txid", Value: tx.TxID},
		{Label: "value out", Value: fmt.Sprintf("%.8f BTC", tx.TotalOutBTC())},
		{Label: "fee", Value: fmt.Sprintf("%.8f BTC", tx.FeeBTC)},
		{Label: "inputs", Value: fmt.Sprintf("%d", tx.FanIn())},
		{Label: "outputs", Value: fmt.Sprintf("%d", tx.FanOut())},
		{Label: "when", Value: tx.Timestamp.Format("2006-01-02 15:04:05")},
		{Label: "completeness", Value: string(tx.Completeness)},
	}}

	inRows := make([]components.Row, 0, len(tx.Inputs))
	for _, in := range tx.Inputs {
		inRows = append(inRows, components.Row{
			Label: shortID(in.Address), Value: fmt.Sprintf("%.8f BTC", in.AmountBTC)})
	}
	outRows := make([]components.Row, 0, len(tx.Outputs))
	for _, o := range tx.Outputs {
		outRows = append(outRows, components.Row{
			Label: shortID(o.Address), Value: fmt.Sprintf("%.8f BTC (sent_to)", o.AmountBTC)})
	}

	edgeRows := []components.Row{
		{Label: "edges from", Value: fmt.Sprintf("%d", len(t.edges.From))},
		{Label: "edges to", Value: fmt.Sprintf("%d", len(t.edges.To))},
	}

	t.detail.SetSections([]components.Section{
		meta,
		{Title: fmt.Sprintf("inputs (%d)", len(inRows)), Rows: inRows},
		{Title: fmt.Sprintf("outputs (%d)", len(outRows)), Rows: outRows},
		{Title: "edges", Rows: edgeRows},
	})
}

// View renders the transaction detail panel.
func (t *Transaction) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	if t.txid == "" {
		return placeholderView(t.styles, "TRANSACTION", "no transaction selected — open one from Search or the Wallet view", f)
	}
	title := t.styles.Title.Render("TRANSACTION") + " " + t.styles.Value.Render(shortID(t.txid))
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	// Wrap the detail in a bordered panel that FILLS the body height, so the
	// screen never shows an empty, un-bordered region below short content
	// (the "bottom cut-off" look). The panel draws its box to exactly bodyH.
	cw, ch := components.PanelInner(components.Frame{W: f.W, H: bodyH})
	panel := components.Panel(t.styles, "TRANSACTION DETAIL",
		t.detail.View(components.Frame{W: cw, H: ch - 1}),
		components.Frame{W: f.W, H: bodyH})
	return clampBlockLocal(title+"\n"+panel, f)
}

// ShortHelp lists Transaction's context keys.
func (t *Transaction) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	}
}

var _ = strings.TrimSpace
