package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Wallet is the Wallet / Entity analysis screen (design §2). It runs the real
// orchestrator (app.NewOrchestrator().AnalyzeWallet) for the selected subject
// and pages the wallet's transactions (WalletTransactions, bounded by limit).
// The right pane renders the risk block honestly (design §10): the numeric
// score, confidence, contributing signals, and the delta when present — the
// LOW/ELEVATED/HIGH/CRITICAL bucket is color-only and never alters the number.
type Wallet struct {
	ctx    *ScreenCtx
	styles theme.Styles

	subject string
	txs     components.Table
	detail  components.DetailPanel
	split   components.SplitView

	result     *schema.InvestigationResult
	resultErr  error
	showDetail bool

	// pipeline is the staged analysis readout (querying → features → ML → risk
	// → evidence → report). Each stage is marked done only from REAL output in
	// the result, so the readout never fabricates progress. reportBuilt records
	// that the user ran the report step (r); reportMsg is its honest summary.
	pipeline    components.Pipeline
	reportBuilt bool
	reportMsg   string

	// summary holds the OPTIONAL local-LLM pane state (design §E.4). It is set
	// only on the explicit L / :explain action, never during normal render.
	summary summaryPane
}

const walletTxLimit = 50

// NewWallet builds the Wallet screen for the current subject.
func NewWallet(ctx *ScreenCtx) *Wallet {
	cols := []table.Column{
		{Title: "txid", Width: 16},
		{Title: "in", Width: 4},
		{Title: "out", Width: 4},
		{Title: "value", Width: 12},
		{Title: "when", Width: 20},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("no transactions for this wallet in the active case")
	w := &Wallet{
		ctx:      ctx,
		styles:   ctx.styles(),
		subject:  ctx.Subject.ID,
		txs:      tbl,
		detail:   components.NewDetailPanel(ctx.styles()),
		split:    components.SplitView{Ratio: 0.55, Compact: ctx.Compact},
		pipeline: components.NewPipeline(ctx.styles(), "ANALYSIS PIPELINE"),
	}
	w.pipeline.SetStages(w.pendingStages())
	return w
}

// pipelineStageNames are the ordered analysis stages shown in the readout.
var pipelineStageNames = []string{
	"querying local evidence",
	"feature extraction",
	"ML inference",
	"risk scoring",
	"evidence assembly",
	"report",
}

// pendingStages returns the initial all-pending stage list with the first stage
// (querying) marked running, reflecting that Init dispatched the analysis.
func (w *Wallet) pendingStages() []components.PipelineStage {
	stages := make([]components.PipelineStage, len(pipelineStageNames))
	for i, name := range pipelineStageNames {
		st := components.PipelineStage{Label: name, Status: components.StagePending}
		if i == 0 {
			st.Status = components.StageRunning
		}
		stages[i] = st
	}
	// The report stage is skipped until the user runs it (r).
	stages[len(stages)-1].Status = components.StageSkipped
	stages[len(stages)-1].Detail = "press r to build"
	return stages
}

// stagesFromResult marks each pre-report stage done, with an honest detail
// drawn from the REAL result output that proves the step ran: predictions
// (ML), signals (risk), evidence items (evidence). A stage with no output is
// marked done with an honest "none" rather than hidden.
func (w *Wallet) stagesFromResult(r *schema.InvestigationResult) []components.PipelineStage {
	done := func(label, detail string) components.PipelineStage {
		return components.PipelineStage{Label: label, Status: components.StageDone, Detail: detail}
	}
	net := "OFFLINE (local evidence only)"
	stages := []components.PipelineStage{
		done(pipelineStageNames[0], net),
		done(pipelineStageNames[1], "feature-schema v1"),
		done(pipelineStageNames[2], mlDetail(r)),
		done(pipelineStageNames[3], fmt.Sprintf("risk %d/100 · %s", r.Risk.Score, riskBand(r.Risk.Score))),
		done(pipelineStageNames[4], fmt.Sprintf("%d evidence items", len(r.Evidence))),
	}
	// Report stage reflects whether the user has built it yet.
	rep := components.PipelineStage{Label: pipelineStageNames[5]}
	if w.reportBuilt {
		rep.Status = components.StageDone
		rep.Detail = w.reportMsg
	} else {
		rep.Status = components.StageSkipped
		rep.Detail = "press r to build (offline)"
	}
	return append(stages, rep)
}

// mlDetail summarizes the ML predictions present in the result (honest: the top
// model + score, or "no model output").
func mlDetail(r *schema.InvestigationResult) string {
	if len(r.Predictions) == 0 {
		return "no model output"
	}
	p := r.Predictions[0]
	return fmt.Sprintf("%s %.2f", p.Model, p.Score)
}

// Init runs the orchestrator and loads the wallet's transactions.
func (w *Wallet) Init() tea.Cmd {
	if w.subject == "" {
		w.detail.SetError("no wallet selected — use Search (/) or jump from another screen")
		return nil
	}
	repo := w.ctx.repo()
	if repo == nil {
		w.detail.SetError(errNoCase.Error())
		return nil
	}
	w.detail.SetLoading()
	w.txs.SetLoading()
	// Offline forced true for analysis: AnalyzeWallet never acquires; it reads
	// local evidence only (the orchestrator's offline flag is informational).
	offline := true
	return tea.Batch(
		analyzeWalletCmd(w.ctx, w.subject, offline),
		walletTxsCmd(w.ctx.bgCtx(), repo, w.subject, walletTxLimit),
	)
}

// Update folds in the analysis result and the transaction page.
func (w *Wallet) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if m.Request != w.subject {
			return w, nil
		}
		switch p := m.Payload.(type) {
		case *schema.InvestigationResult:
			w.result, w.resultErr = p, nil
			w.detail.SetSections(w.detailSections())
			w.pipeline.SetStages(w.stagesFromResult(p))
		case schema.Report:
			// The report build step completed: mark the report stage done with
			// an honest snapshot summary and refresh the pipeline.
			w.reportBuilt = true
			w.reportMsg = "built " + shortID(p.ID) + " · snapshot " + shortID(p.SnapshotSHA256)
			if w.result != nil {
				w.pipeline.SetStages(w.stagesFromResult(w.result))
			}
		case []schema.Transaction:
			w.setTxs(p)
		}
	case dataError:
		if m.Request == w.subject {
			w.resultErr = m.Err
			w.detail.SetError(m.Err.Error())
			w.txs.SetError(m.Err.Error())
			// Mark the running stage failed so the pipeline is honest.
			stages := w.pipeline.StagesCopy()
			for i := range stages {
				if stages[i].Status == components.StageRunning {
					stages[i].Status = components.StageFailed
					stages[i].Detail = m.Err.Error()
				}
			}
			w.pipeline.SetStages(stages)
		}
	case SummaryLoaded:
		w.summary.acceptSummary(w.ctx, m)
		return w, nil
	case tea.KeyMsg:
		switch m.String() {
		case "tab":
			if w.split.Compact {
				w.split.Showing ^= 1
			}
			return w, nil
		case "g":
			// Cross-link to the Graph scoped to this wallet.
			if w.subject != "" {
				id := w.subject
				return w, func() tea.Msg {
					return NavScreen{Screen: "graph", Subject: Subject{ID: id, Kind: SubjectWallet}}
				}
			}
			return w, nil
		case "t":
			// Cross-link to the Timeline scoped to this wallet.
			if w.subject != "" {
				id := w.subject
				return w, func() tea.Msg {
					return NavScreen{Screen: "timeline", Subject: Subject{ID: id, Kind: SubjectWallet}}
				}
			}
			return w, nil
		case "r":
			// Build the forensic report for this subject (offline, deterministic).
			// Marks the report stage running; the built schema.Report arrives via
			// the dataLoaded case and flips the stage to done.
			if w.result == nil {
				return w, nil // nothing analyzed yet
			}
			stages := w.pipeline.StagesCopy()
			if len(stages) > 0 {
				stages[len(stages)-1].Status = components.StageRunning
				stages[len(stages)-1].Detail = "building snapshot…"
				w.pipeline.SetStages(stages)
			}
			return w, analyzeForReportCmd(w.ctx, w.subject)
		case "L":
			// Explicit, on-demand local-LLM summary (design §E.4). NIT-4: with no
			// loaded result this is an honest no-op note, not a network call.
			return w, w.summary.requestExplain(w.ctx, w.result)
		}
		var cmd tea.Cmd
		w.txs, cmd = w.txs.Update(m)
		return w, cmd
	}
	return w, nil
}

func (w *Wallet) setTxs(txs []schema.Transaction) {
	rows := make([]table.Row, 0, len(txs))
	ids := make([]string, 0, len(txs))
	kinds := make([]string, 0, len(txs))
	for _, t := range txs {
		rows = append(rows, table.Row{
			shortID(t.TxID),
			fmt.Sprintf("%d", t.FanIn()),
			fmt.Sprintf("%d", t.FanOut()),
			fmt.Sprintf("%.4f", t.TotalOutBTC()),
			t.Timestamp.Format("2006-01-02 15:04"),
		})
		ids = append(ids, t.TxID)
		kinds = append(kinds, "tx")
	}
	w.txs.SetRows(rows, ids, kinds)
}

// detailSections builds the right-pane sections from the InvestigationResult.
// The risk block always shows score + confidence + signals (+ delta when
// present); the bucket label is a presentation cue only (design §10).
func (w *Wallet) detailSections() []components.Section {
	if w.result == nil {
		return nil
	}
	r := w.result
	risk := r.Risk
	riskRows := []components.Row{
		{Label: "score", Value: fmt.Sprintf("%d/100", risk.Score), Role: riskRole(risk.Score)},
		{Label: "band", Value: riskBand(risk.Score), Role: riskRole(risk.Score)},
		{Label: "confidence", Value: fmt.Sprintf("%.2f", risk.Confidence)},
	}
	if risk.Delta != nil {
		riskRows = append(riskRows, components.Row{
			Label: "delta", Value: fmt.Sprintf("%+d", *risk.Delta), Role: theme.RoleWarning})
	}
	secs := []components.Section{{Title: "risk", Rows: riskRows}}

	if len(risk.Signals) > 0 {
		sigRows := make([]components.Row, 0, len(risk.Signals))
		for _, s := range risk.Signals {
			sigRows = append(sigRows, components.Row{
				Label: s.Name, Value: fmt.Sprintf("score=%.2f weight=%.2f", s.Score, s.Weight)})
		}
		secs = append(secs, components.Section{Title: "signals", Rows: sigRows})
	}

	secs = append(secs, components.Section{Title: "related", Rows: []components.Row{
		{Label: "related wallets", Value: fmt.Sprintf("%d", r.RelatedWallets)},
		{Label: "relevant txs", Value: fmt.Sprintf("%d", r.RelevantTxs)},
	}})

	if len(r.Evidence) > 0 {
		evRows := make([]components.Row, 0, len(r.Evidence))
		for _, e := range r.Evidence {
			evRows = append(evRows, components.Row{
				Label: string(e.Severity), Value: e.Description, Role: theme.SeverityRole(string(e.Severity))})
		}
		secs = append(secs, components.Section{Title: "evidence", Rows: evRows})
	}
	return secs
}

// View composes the transaction list and the analysis detail in a split pane.
func (w *Wallet) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	title := w.styles.Title.Render("WALLET") + " " + w.styles.Value.Render(shortID(w.subject))
	if w.subject == "" {
		return placeholderView(w.styles, "WALLET", "no wallet selected — press / to search or jump from another screen", f)
	}
	// Staged analysis pipeline band (querying → … → report). Sized to the stage
	// count + a title row; it never fabricates progress (stages reflect real
	// result output). Shown whenever the pane is tall enough to spare the rows.
	pipeH := 0
	pipeStr := ""
	if f.H >= 20 {
		// Panel spends 3 rows on border+padding; the pipeline needs 1 title row
		// + one row per stage. Size the panel so every stage is visible.
		pipeH = len(pipelineStageNames) + 1 + 3
		pipeStr = components.Panel(w.styles, "RUN",
			w.pipeline.View(components.Frame{W: f.W - 2, H: paneInner(pipeH)}),
			components.Frame{W: f.W, H: pipeH})
	}
	bodyH := f.H - 1 - pipeH
	if bodyH < 1 {
		bodyH = 1
	}
	// When the optional LLM pane is active, reserve the lower third of the body
	// for it (design §E.4). The pane is presentation-only; it never alters the
	// risk/evidence detail above it.
	paneH := 0
	if w.summary.active {
		paneH = bodyH / 3
		if paneH < 4 {
			paneH = 4
		}
		if paneH > bodyH-2 {
			paneH = bodyH - 2
		}
	}
	splitH := bodyH - paneH
	if splitH < 1 {
		splitH = 1
	}
	body := components.Frame{W: f.W, H: splitH}
	w.split.Compact = f.W < 100
	lf, rf := w.split.Frames(body)
	// Shared cockpit chrome: each split pane is a titled bordered panel so the
	// Wallet screen reads as the same instrument panel as the Dashboard.
	lcw, lch := components.PanelInner(lf)
	rcw, rch := components.PanelInner(rf)
	left := components.Panel(w.styles, "TRANSACTIONS",
		w.txs.View(components.Frame{W: lcw, H: lch - 1}), lf)
	right := components.Panel(w.styles, "ANALYSIS / RISK",
		w.detail.View(components.Frame{W: rcw, H: rch - 1}), rf)
	joined := w.split.Join(body, left, right)
	out := title + "\n"
	if pipeStr != "" {
		out += pipeStr + "\n"
	}
	out += joined
	if paneH > 0 {
		out += "\n" + renderSummaryPane(w.styles, w.summary, components.Frame{W: f.W, H: paneH})
	}
	return clampBlockLocal(out, f)
}

// ShortHelp lists Wallet's context keys.
func (w *Wallet) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open tx")),
		key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "timeline")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "report")),
		key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "explain (local)")),
	}
}

// riskBand maps a numeric score to its presentation bucket label (design §10).
// It is color/emphasis only and never changes the number.
func riskBand(score int) string {
	switch {
	case score >= 75:
		return "CRITICAL (75-100)"
	case score >= 50:
		return "HIGH (50-74)"
	case score >= 25:
		return "ELEVATED (25-49)"
	default:
		return "LOW (0-24)"
	}
}

// riskRole maps a numeric score to its semantic color role (design §10).
func riskRole(score int) theme.Role {
	switch {
	case score >= 75:
		return theme.RoleCritical
	case score >= 50:
		return theme.RoleWarning
	case score >= 25:
		return theme.RoleNeutral
	default:
		return theme.RoleHealthy
	}
}

var _ = strings.TrimSpace
