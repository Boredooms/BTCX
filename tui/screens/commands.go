package screens

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
	tea "github.com/charmbracelet/bubbletea"
)

// commands.go wraps every service call a screen issues as a cancellable tea.Cmd
// (design §11.2-§11.3). The UI thread NEVER blocks on I/O or analysis: a command
// runs on Bubble Tea's goroutine pool and returns a dataLoaded/dataError msg.
// Each takes the screen context so Esc / screen change / a superseding query /
// quit cancels in-flight work (no goroutine leak).
//
// These factories are the ONLY place a screen touches a service. They run no
// SQL, build no graph, and compute no risk — they call the existing service and
// ship back the structured result (AGENTS §12).

// dataLoaded carries a successful service result to the screen that requested
// it. Request is an optional correlation key so a screen can drop a stale result
// from a superseded query.
type dataLoaded struct {
	Request string
	Payload any
}

// dataError carries a failed service result. It is surfaced honestly in the
// screen body / context bar, never silently swallowed (AGENTS §19).
type dataError struct {
	Request string
	Err     error
}

// --- Repository reads (bounded; §11.4) --------------------------------------

func countsCmd(ctx context.Context, repo sdk.Repository) tea.Cmd {
	return func() tea.Msg {
		c, err := repo.Counts(ctx)
		if err != nil {
			return dataError{Request: "counts", Err: err}
		}
		return dataLoaded{Request: "counts", Payload: c}
	}
}

// recentTxCmd reads the newest transactions for the dashboard activity feed.
func recentTxCmd(ctx context.Context, repo sdk.Repository, limit int) tea.Cmd {
	return func() tea.Msg {
		txs, err := repo.RecentTransactions(ctx, limit)
		if err != nil {
			return dataError{Request: "recent-tx", Err: err}
		}
		return dataLoaded{Request: "recent-tx", Payload: recentTxResult(txs)}
	}
}

// recentTxResult tags a transaction slice as the recent-tx feed payload so the
// dashboard's Update can distinguish it from other []schema.Transaction loads.
type recentTxResult []schema.Transaction

func alertsCmd(ctx context.Context, repo sdk.Repository, limit int) tea.Cmd {
	return func() tea.Msg {
		a, err := repo.ListAlerts(ctx, limit)
		if err != nil {
			return dataError{Request: "alerts", Err: err}
		}
		return dataLoaded{Request: "alerts", Payload: a}
	}
}

func walletCmd(ctx context.Context, repo sdk.Repository, address string) tea.Cmd {
	return func() tea.Msg {
		w, err := repo.GetWallet(ctx, address)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		return dataLoaded{Request: address, Payload: w}
	}
}

func walletTxsCmd(ctx context.Context, repo sdk.Repository, address string, limit int) tea.Cmd {
	return func() tea.Msg {
		txs, err := repo.WalletTransactions(ctx, address, limit)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		return dataLoaded{Request: address, Payload: txs}
	}
}

func transactionCmd(ctx context.Context, repo sdk.Repository, txid string) tea.Cmd {
	return func() tea.Msg {
		t, err := repo.GetTransaction(ctx, txid)
		if err != nil {
			return dataError{Request: txid, Err: err}
		}
		return dataLoaded{Request: txid, Payload: t}
	}
}

// edgesResult bundles a transaction's incident edges for the Transaction screen.
type edgesResult struct {
	From []schema.GraphEdge
	To   []schema.GraphEdge
}

func edgesCmd(ctx context.Context, repo sdk.Repository, nodeID string) tea.Cmd {
	return func() tea.Msg {
		from, err := repo.EdgesFrom(ctx, nodeID)
		if err != nil {
			return dataError{Request: nodeID, Err: err}
		}
		to, err := repo.EdgesTo(ctx, nodeID)
		if err != nil {
			return dataError{Request: nodeID, Err: err}
		}
		return dataLoaded{Request: nodeID, Payload: edgesResult{From: from, To: to}}
	}
}

func observationsByIPCmd(ctx context.Context, repo sdk.Repository, ip string) tea.Cmd {
	return func() tea.Msg {
		obs, err := repo.NetworkObservationsByIP(ctx, ip)
		if err != nil {
			return dataError{Request: ip, Err: err}
		}
		return dataLoaded{Request: ip, Payload: obs}
	}
}

func allObservationsCmd(ctx context.Context, repo sdk.Repository, limit int) tea.Cmd {
	return func() tea.Msg {
		obs, err := repo.AllNetworkObservations(ctx, limit)
		if err != nil {
			return dataError{Request: "all-obs", Err: err}
		}
		return dataLoaded{Request: "all-obs", Payload: obs}
	}
}

func datasetsCmd(ctx context.Context, repo sdk.Repository) tea.Cmd {
	return func() tea.Msg {
		ds, err := repo.ListDatasets(ctx)
		if err != nil {
			return dataError{Request: "datasets", Err: err}
		}
		return dataLoaded{Request: "datasets", Payload: ds}
	}
}

func reportsCmd(ctx context.Context, repo sdk.Repository) tea.Cmd {
	return func() tea.Msg {
		rs, err := repo.ListReports(ctx)
		if err != nil {
			return dataError{Request: "reports", Err: err}
		}
		return dataLoaded{Request: "reports", Payload: rs}
	}
}

// reportRowResult tags a single persisted report row loaded for preview so the
// Reports screen's Update can distinguish it from the report LIST payload.
type reportRowResult struct{ Row sdk.ReportRow }

// reportByIDCmd loads one persisted report's metadata for preview when a row is
// selected. It is a bounded local read — never a rebuild or a network call.
func reportByIDCmd(ctx context.Context, repo sdk.Repository, id string) tea.Cmd {
	return func() tea.Msg {
		row, err := repo.GetReport(ctx, id)
		if err != nil {
			return dataError{Request: id, Err: err}
		}
		if row == nil {
			return dataError{Request: id, Err: errReportNotFound}
		}
		return dataLoaded{Request: id, Payload: reportRowResult{Row: *row}}
	}
}

func monitorSessionsCmd(ctx context.Context, repo sdk.Repository) tea.Cmd {
	return func() tea.Msg {
		s, err := repo.ListMonitorSessions(ctx)
		if err != nil {
			return dataError{Request: "sessions", Err: err}
		}
		return dataLoaded{Request: "sessions", Payload: s}
	}
}

// monitorHistory bundles one session's recorded events + risk deltas for the
// historical (non-live) monitor view.
type monitorHistory struct {
	SessionID string
	Events    []sdk.MonitorEventRow
	Deltas    []sdk.RiskDeltaRow
}

func monitorHistoryCmd(ctx context.Context, repo sdk.Repository, sessionID string, limit int) tea.Cmd {
	return func() tea.Msg {
		events, err := repo.ListMonitorEvents(ctx, sessionID, limit)
		if err != nil {
			return dataError{Request: sessionID, Err: err}
		}
		deltas, err := repo.ListRiskDeltas(ctx, sessionID)
		if err != nil {
			return dataError{Request: sessionID, Err: err}
		}
		return dataLoaded{Request: sessionID, Payload: monitorHistory{SessionID: sessionID, Events: events, Deltas: deltas}}
	}
}

// --- Analysis services (built via app/services.go) --------------------------

// analyzeWalletCmd runs the orchestrator for a wallet subject. The orchestrator
// is built with app.ModelsDir(cfg) so model loads succeed regardless of CWD,
// and Close()d when the command finishes so no model handle leaks.
func analyzeWalletCmd(c *ScreenCtx, address string, offline bool) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	caseID := c.caseID()
	modelsDir := modelsDir(c.Cfg)
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: address, Err: errNoCase}
		}
		orch := app.NewOrchestrator(repo, caseID, modelsDir)
		defer orch.Close()
		res, err := orch.AnalyzeWallet(ctx, address, offline)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		return dataLoaded{Request: address, Payload: res}
	}
}

// subgraphCmd fetches a bounded subgraph centered on a node at a given depth.
// Depth changes dispatch a fresh command so the previous query is superseded
// rather than piling up (design §9, §11.3).
func subgraphCmd(c *ScreenCtx, center string, depth int) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: center, Err: errNoCase}
		}
		gsvc := app.NewGraph(repo)
		sg, err := gsvc.Subgraph(ctx, center, depth)
		if err != nil {
			return dataError{Request: center, Err: err}
		}
		return dataLoaded{Request: center, Payload: sg}
	}
}

// pathResult carries a resolved path for the Graph path mode.
type pathResult struct {
	Src string
	Dst string
	IDs []string
}

func pathCmd(c *ScreenCtx, src, dst string) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: src + "->" + dst, Err: errNoCase}
		}
		gsvc := app.NewGraph(repo)
		ids, err := gsvc.Path(ctx, src, dst)
		if err != nil {
			return dataError{Request: src + "->" + dst, Err: err}
		}
		return dataLoaded{Request: src + "->" + dst, Payload: pathResult{Src: src, Dst: dst, IDs: ids}}
	}
}

// timelineResult carries both the timeline events and the snapshot's alerts so
// the TimelineView can look up a severity for alert-kind rows only (§3).
type timelineResult struct {
	Subject string
	Events  []models.TimelineEvent
	Alerts  []schema.Alert
}

// analyzeThenTimelineCmd analyzes a subject, builds its report snapshot, and
// returns the snapshot's timeline + alerts. All three steps are local,
// deterministic, and never dial (design §12.3); they share one cancellable
// context so Esc/quit stops the whole chain. The alerts are threaded through so
// the TimelineView can show a severity only on alert-kind rows (design §3).
func analyzeThenTimelineCmd(c *ScreenCtx, address string) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	caseID := c.caseID()
	md := modelsDir(c.Cfg)
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: address, Err: errNoCase}
		}
		orch := app.NewOrchestrator(repo, caseID, md)
		defer orch.Close()
		res, err := orch.AnalyzeWallet(ctx, address, true)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		rsvc := app.NewReporting(repo)
		snap, err := rsvc.BuildSnapshot(ctx, *res)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		events, err := rsvc.Timeline(ctx, snap)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		return dataLoaded{Request: address, Payload: timelineResult{
			Subject: address, Events: events, Alerts: snap.Alerts}}
	}
}

// analyzeForReportCmd runs the orchestrator for a subject and, on success,
// builds (persists) a deterministic report snapshot from the result via
// reporting.Service. Both steps are local and never dial (design §12.3). The
// returned payload is the built schema.Report for the Reports preview.
func analyzeForReportCmd(c *ScreenCtx, address string) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	caseID := c.caseID()
	md := modelsDir(c.Cfg)
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: address, Err: errNoCase}
		}
		orch := app.NewOrchestrator(repo, caseID, md)
		defer orch.Close()
		res, err := orch.AnalyzeWallet(ctx, address, true)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		rsvc := app.NewReporting(repo)
		rep, err := rsvc.Build(ctx, *res)
		if err != nil {
			return dataError{Request: address, Err: err}
		}
		return dataLoaded{Request: address, Payload: rep}
	}
}

// reportExportDir resolves a writable directory for TUI report exports (the
// runtime tmp dir), defaulting to "." when no config is wired.
func reportExportDir(c *ScreenCtx) string {
	if c != nil && c.Cfg != nil {
		return configs.NewLayout(c.Cfg).Tmp
	}
	return "."
}

// reportOutPath builds the export file path for a report id + format.
func reportOutPath(dir, reportID string, format schema.ReportFormat) string {
	return filepath.Join(dir, "report-"+shortID(reportID)+"."+string(format))
}

// reportContent carries a rendered report body back to the Reports screen for
// in-TUI viewing. Format is the ReportFormat it was rendered in; Text is the
// rendered bytes as a string. It is a local, deterministic render — no network
// (reporting is offline-confined, design §12.3).
type reportContent struct {
	ReportID string
	Format   schema.ReportFormat
	Text     string
}

// reportRenderCmd rebuilds the snapshot from an investigation result and renders
// it to the requested format (json/md/html) for in-TUI viewing. Local only; the
// reporting package has no network in its closure (offline-boundary test).
func reportRenderCmd(c *ScreenCtx, reportID string, result schema.InvestigationResult, format schema.ReportFormat) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: reportID, Err: errNoCase}
		}
		rsvc := app.NewReporting(repo)
		snap, err := rsvc.BuildSnapshot(ctx, result)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		data, err := rsvc.Render(ctx, snap, format)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		return dataLoaded{Request: reportID, Payload: reportContent{
			ReportID: reportID, Format: format, Text: string(data)}}
	}
}

// reportExportCmd builds + renders a report from a result and writes it to a
// file under the runtime tmp dir, returning the path. Offline, atomic write.
func reportExportCmd(c *ScreenCtx, reportID string, result schema.InvestigationResult, format schema.ReportFormat) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	dir := reportExportDir(c)
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: reportID, Err: errNoCase}
		}
		rsvc := app.NewReporting(repo)
		rep, err := rsvc.Build(ctx, result)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		out := reportOutPath(dir, reportID, format)
		if err := rsvc.Export(ctx, rep, format, out); err != nil {
			return dataError{Request: reportID, Err: err}
		}
		return reportExported{Path: out, Format: format}
	}
}

// reportExportByIDCmd loads a persisted report by id and exports it to a file.
func reportExportByIDCmd(c *ScreenCtx, reportID string, format schema.ReportFormat) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	dir := reportExportDir(c)
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: reportID, Err: errNoCase}
		}
		row, err := repo.GetReport(ctx, reportID)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		if row == nil {
			return dataError{Request: reportID, Err: errReportNotFound}
		}
		var result schema.InvestigationResult
		if uerr := json.Unmarshal([]byte(row.ResultJSON), &result); uerr != nil {
			return dataError{Request: reportID, Err: uerr}
		}
		rsvc := app.NewReporting(repo)
		rep, err := rsvc.Build(ctx, result)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		out := reportOutPath(dir, reportID, format)
		if err := rsvc.Export(ctx, rep, format, out); err != nil {
			return dataError{Request: reportID, Err: err}
		}
		return reportExported{Path: out, Format: format}
	}
}

// reportViewByIDCmd loads a persisted report by id, parses its stored result,
// then renders it in one shot — so a screen can VIEW the highlighted report row
// directly (no separate select step). Local + offline.
func reportViewByIDCmd(c *ScreenCtx, reportID string, format schema.ReportFormat) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: reportID, Err: errNoCase}
		}
		row, err := repo.GetReport(ctx, reportID)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		if row == nil {
			return dataError{Request: reportID, Err: errReportNotFound}
		}
		var result schema.InvestigationResult
		if uerr := json.Unmarshal([]byte(row.ResultJSON), &result); uerr != nil {
			return dataError{Request: reportID, Err: uerr}
		}
		rsvc := app.NewReporting(repo)
		snap, err := rsvc.BuildSnapshot(ctx, result)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		data, err := rsvc.Render(ctx, snap, format)
		if err != nil {
			return dataError{Request: reportID, Err: err}
		}
		return dataLoaded{Request: reportID, Payload: reportContent{
			ReportID: reportID, Format: format, Text: string(data)}}
	}
}

// graphStatsCmd reads the persisted graph statistics for the Data screen.
func graphStatsCmd(c *ScreenCtx) tea.Cmd {
	ctx := c.bgCtx()
	repo := c.repo()
	return func() tea.Msg {
		if repo == nil {
			return dataError{Request: "graph-stats", Err: errNoCase}
		}
		b := graph.NewBuilder(repo)
		st, err := b.GraphStats(ctx)
		if err != nil {
			return dataError{Request: "graph-stats", Err: err}
		}
		return dataLoaded{Request: "graph-stats", Payload: st}
	}
}

// --- Optional local-LLM summary (design §E) ---------------------------------

// SummaryLoaded carries the plain-language summary back to the screen that
// requested it (design §E.4). Text is the summary string to render; Degraded is
// true when the LLM was disabled/unreachable and the deterministic summary is
// shown instead; Note is the honest provenance/degrade line the pane renders in
// a muted style. It NEVER carries a computed risk/evidence/pattern value — only
// a projection of values the services already produced.
type SummaryLoaded struct {
	Text     string
	Degraded bool
	Note     string
}

// BuildSummaryInput projects an already-computed InvestigationResult into the
// read-only llm.SummaryInput (design §E.2). It computes NOTHING new: RiskBand is
// the EXISTING unexported presentation mapping riskBand(res.Risk.Score) — in
// scope here in package screens, so riskBand stays unexported — and every other
// field is copied verbatim from the result. It is the single place the band is
// applied for the LLM pane, keeping llm free of pkg/schema and keeping the band
// a presentation value, never a schema field or a new computation.
func BuildSummaryInput(res schema.InvestigationResult) llm.SummaryInput {
	return llm.SummaryInput{
		Subject:   res.Subject,
		RiskScore: res.Risk.Score,
		RiskBand:  riskBand(res.Risk.Score),
		Evidence:  topEvidenceDescriptions(res.Evidence, 5),
		Patterns:  patternLabels(res.Patterns),
		Related:   res.RelatedWallets,
		Txs:       res.RelevantTxs,
	}
}

// topEvidenceDescriptions copies up to n evidence descriptions verbatim. It
// invents nothing: it only slices the already-produced evidence list.
func topEvidenceDescriptions(ev []schema.EvidenceItem, n int) []string {
	if len(ev) < n {
		n = len(ev)
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, ev[i].Description)
	}
	return out
}

// patternLabels copies each detected pattern as "<type>: <description>" using
// the already-produced Type + Description (PatternResult has no Name field).
func patternLabels(ps []schema.PatternResult) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		if p.Description == "" {
			out = append(out, string(p.Type))
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", p.Type, p.Description))
	}
	return out
}

// explainCmd dispatches the OPTIONAL summary off the UI thread (design §E.3/E.4).
// It is invoked ONLY on the explicit L key / :explain verb — never from Init,
// View, or a tick. A nil summarizer degrades to llm.NewDeterministic() so a bare
// ScreenCtx (tests / no wiring) still produces a summary with no network call.
// The returned command yields a SummaryLoaded the screen renders in its pane;
// an error from the summarizer degrades honestly to the deterministic summary
// plus a "not reachable" note, so the pane is never empty and never fabricates.
func explainCmd(ctx context.Context, sum llm.Summarizer, in llm.SummaryInput) tea.Cmd {
	if sum == nil {
		sum = llm.NewDeterministic()
	}
	return func() tea.Msg {
		text, err := sum.Summarize(ctx, in)
		if err != nil {
			// Degrade: the summarizer returns the deterministic text alongside
			// the error, so Text is non-empty and fact-grounded.
			if text == "" {
				det, derr := llm.NewDeterministic().Summarize(ctx, in)
				if derr != nil {
					return SummaryLoaded{Text: "", Degraded: true,
						Note: "local LLM not reachable — summary unavailable"}
				}
				text = det
			}
			return SummaryLoaded{Text: text, Degraded: true,
				Note: "local LLM not reachable — deterministic summary"}
		}
		return SummaryLoaded{Text: text, Degraded: false}
	}
}

// Ensure the orchestrator package import is retained for the type the analyze
// command constructs through the app seam.
var _ = (*orchestrator.Orchestrator)(nil)
var _ = (*reporting.Service)(nil)
