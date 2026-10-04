package app

import (
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/ml/inference"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/risk/scoring"
	"github.com/bctx/bctx/sdk"
)

// The analysis services are nil on the Engine (see bootstrap.go), so callers
// build them explicitly here with the exact inputs the existing CLI call sites
// use (audit §3.6). Each screen asks app for the service it needs; the
// orchestrator and registry hold model handles and must be Close()d by the
// caller's cleanup/cancellation path.

// NewOrchestrator builds the investigation orchestrator for the active case,
// mirroring cli/commands/analyze.go's orchestrator.New call exactly. modelsDir
// should come from ModelsDir(cfg) so model loads succeed regardless of CWD.
func NewOrchestrator(repo sdk.Repository, caseID, modelsDir string) *orchestrator.Orchestrator {
	return orchestrator.New(orchestrator.Options{
		Repo:             repo,
		ModelsDir:        modelsDir,
		FeatureSchemaSHA: schema.FeatureSchemaSHA256,
		CaseID:           caseID,
		Weights:          scoring.DefaultWeights(),
	})
}

// NewGraph builds the bounded graph traversal service (MaxNodes=5000).
func NewGraph(repo sdk.Repository) *graph.Service {
	return graph.NewService(repo)
}

// NewReporting builds the reporting service, stamping the generator identity so
// reports record that the TUI produced them.
func NewReporting(repo sdk.Repository) *reporting.Service {
	return reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "bctx-tui"})
}

// NewRegistry builds the ML inference registry used both by AnalyzeWallet and
// by the MODELS status indicator. modelsDir should come from ModelsDir(cfg).
func NewRegistry(modelsDir string) *inference.Registry {
	return inference.NewRegistry(modelsDir, schema.FeatureSchemaSHA256)
}
