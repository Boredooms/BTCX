package tui

import (
	"context"

	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
	tea "github.com/charmbracelet/bubbletea"
)

// commands.go wraps every service call as a cancellable tea.Cmd (design
// §11.2-§11.3). The UI thread NEVER blocks on I/O or analysis: a command runs
// on Bubble Tea's goroutine pool and returns DataLoaded or DataError. Each
// command takes a context.Context derived by the screen so Esc / screen change
// / a superseding query / quit cancels the in-flight work (no goroutine leak).
//
// These factories are the only place the TUI touches a service. They run no
// SQL, build no graph, and compute no risk themselves — they call the existing
// service and ship back the structured result (AGENTS §12).

// analyzeWalletCmd runs orchestrator.AnalyzeWallet for a subject address.
func analyzeWalletCmd(ctx context.Context, orch *orchestrator.Orchestrator, address string, offline bool) tea.Cmd {
	return func() tea.Msg {
		res, err := orch.AnalyzeWallet(ctx, address, offline)
		if err != nil {
			return DataError{Screen: ScreenWallet, Request: address, Err: err}
		}
		return DataLoaded{Screen: ScreenWallet, Request: address, Payload: res}
	}
}

// subgraphCmd fetches a bounded subgraph centered on a node at a given depth.
// Depth changes dispatch a fresh command under a new context so the previous
// in-flight query is cancelled rather than piling up (design §9, §11.3).
func subgraphCmd(ctx context.Context, gsvc *graph.Service, center string, depth int) tea.Cmd {
	return func() tea.Msg {
		sg, err := gsvc.Subgraph(ctx, center, depth)
		if err != nil {
			return DataError{Screen: ScreenGraph, Request: center, Err: err}
		}
		return DataLoaded{Screen: ScreenGraph, Request: center, Payload: sg}
	}
}

// pathCmd resolves a path between two nodes for the Graph path mode.
func pathCmd(ctx context.Context, gsvc *graph.Service, src, dst string) tea.Cmd {
	return func() tea.Msg {
		ids, err := gsvc.Path(ctx, src, dst)
		if err != nil {
			return DataError{Screen: ScreenGraph, Request: src + "->" + dst, Err: err}
		}
		return DataLoaded{Screen: ScreenGraph, Request: src + "->" + dst, Payload: ids}
	}
}

// walletTxsCmd pages a wallet's transactions (bounded by limit, design §11.4).
func walletTxsCmd(ctx context.Context, repo sdk.Repository, address string, limit int) tea.Cmd {
	return func() tea.Msg {
		txs, err := repo.WalletTransactions(ctx, address, limit)
		if err != nil {
			return DataError{Screen: ScreenWallet, Request: address, Err: err}
		}
		return DataLoaded{Screen: ScreenWallet, Request: address, Payload: txs}
	}
}

// alertsCmd lists case alerts (bounded by limit).
func alertsCmd(ctx context.Context, repo sdk.Repository, limit int) tea.Cmd {
	return func() tea.Msg {
		alerts, err := repo.ListAlerts(ctx, limit)
		if err != nil {
			return DataError{Screen: ScreenAlerts, Err: err}
		}
		return DataLoaded{Screen: ScreenAlerts, Payload: alerts}
	}
}

// countsCmd reads local corpus counts for the Dashboard/TopBar DB field.
func countsCmd(ctx context.Context, repo sdk.Repository) tea.Cmd {
	return func() tea.Msg {
		counts, err := repo.Counts(ctx)
		if err != nil {
			return DataError{Screen: ScreenHome, Err: err}
		}
		return DataLoaded{Screen: ScreenHome, Payload: counts}
	}
}

// observationsCmd fetches network observations for an IP (Network/Geo screens).
func observationsCmd(ctx context.Context, repo sdk.Repository, ip string) tea.Cmd {
	return func() tea.Msg {
		obs, err := repo.NetworkObservationsByIP(ctx, ip)
		if err != nil {
			return DataError{Screen: ScreenNetwork, Request: ip, Err: err}
		}
		return DataLoaded{Screen: ScreenNetwork, Request: ip, Payload: obs}
	}
}

// timelineCmd builds a report snapshot for a result and returns its timeline.
// Snapshot build + Timeline are both local, deterministic, and never dial
// (design §12.3). The two calls share one context so cancellation stops both.
func timelineCmd(ctx context.Context, rsvc *reporting.Service, result schema.InvestigationResult) tea.Cmd {
	return func() tea.Msg {
		snap, err := rsvc.BuildSnapshot(ctx, result)
		if err != nil {
			return DataError{Screen: ScreenTimeline, Request: result.Subject, Err: err}
		}
		events, err := rsvc.Timeline(ctx, snap)
		if err != nil {
			return DataError{Screen: ScreenTimeline, Request: result.Subject, Err: err}
		}
		return DataLoaded{Screen: ScreenTimeline, Request: result.Subject, Payload: events}
	}
}

// Ensure the models package import is retained for the TimelineEvent type that
// screens assert on the DataLoaded payload.
var _ = []models.TimelineEvent(nil)
