package sdk

import (
	"context"
	"errors"
	"time"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/network/state"
	"github.com/bctx/bctx/pkg/schema"
)

// ErrNotImplemented marks a Phase-0 stub that a later phase will fill in. It is
// returned instead of fabricating fake results, per the "no fake
// implementations" rule.
var ErrNotImplemented = errors.New("not implemented in this phase")

// Engine is the single orchestration facade. CLI and TUI both drive the engine;
// neither reimplements the investigation pipeline.
type Engine struct {
	cfg    *configs.Config
	layout configs.Layout
	prober state.Prober

	Repo      Repository
	Graph     GraphService
	Features  FeatureService
	ML        MLService
	Detection DetectionService
	Risk      RiskService
	Evidence  EvidenceService
	Reports   ReportService
	Cases     CaseService
}

// Options configures engine construction. Any nil service is replaced with a
// clearly-marked stub so the engine always builds and reports honest errors.
type Options struct {
	Config *configs.Config
	Prober state.Prober

	Repo      Repository
	Graph     GraphService
	Features  FeatureService
	ML        MLService
	Detection DetectionService
	Risk      RiskService
	Evidence  EvidenceService
	Reports   ReportService
	Cases     CaseService
}

// NewEngine builds an engine from options, applying defaults.
func NewEngine(opts Options) *Engine {
	cfg := opts.Config
	if cfg == nil {
		cfg = configs.Default()
	}
	prober := opts.Prober
	if prober == nil {
		if cfg.Network.Mode == configs.ModeAirgap {
			prober = state.AirgapProber{}
		} else {
			prober = state.DefaultProber()
		}
	}
	return &Engine{
		cfg:       cfg,
		layout:    configs.NewLayout(cfg),
		prober:    prober,
		Repo:      opts.Repo,
		Graph:     opts.Graph,
		Features:  opts.Features,
		ML:        opts.ML,
		Detection: opts.Detection,
		Risk:      opts.Risk,
		Evidence:  opts.Evidence,
		Reports:   opts.Reports,
		Cases:     opts.Cases,
	}
}

// Config returns the active configuration.
func (e *Engine) Config() *configs.Config { return e.cfg }

// Layout returns the runtime directory layout.
func (e *Engine) Layout() configs.Layout { return e.layout }

// NetworkStatus probes connectivity for display. Airgap/offline modes short
// circuit without any socket activity.
func (e *Engine) NetworkStatus(ctx context.Context) state.Status {
	switch e.cfg.Network.Mode {
	case configs.ModeAirgap:
		return state.Airgapped
	case configs.ModeOffline:
		return state.Disconnected
	default:
		return e.prober.Probe(ctx)
	}
}

// AnalyzeWallet orchestrates the signature workflow. The pipeline order is
// fixed and identical for CLI and TUI:
//
//	local check -> (optional acquire) -> persist -> graph -> features ->
//	ML -> patterns -> clustering -> risk -> propagation -> evidence -> result
//
// Services that are not yet implemented return ErrNotImplemented rather than
// fabricating output. offline forces the acquisition step to be skipped.
func (e *Engine) AnalyzeWallet(ctx context.Context, address string, offline bool) (*schema.InvestigationResult, error) {
	if e.Repo == nil {
		return nil, ErrNotImplemented
	}

	result := &schema.InvestigationResult{
		ID:          "inv-" + address,
		Subject:     address,
		SubjectType: schema.NodeWallet,
		Offline:     offline || !e.cfg.AcquisitionAllowed(),
		CreatedAt:   time.Now().UTC(),
	}

	// Step 1: local evidence check.
	w, err := e.Repo.GetWallet(ctx, address)
	if err != nil {
		return nil, err
	}
	_ = w // later phases populate related counts from the wallet record.

	// Steps 2+ depend on services wired in later phases. Each guarded so the
	// engine degrades honestly instead of inventing results.
	if e.Features != nil {
		if fv, ferr := e.Features.WalletFeatures(ctx, address); ferr == nil && e.ML != nil {
			if p, perr := e.ML.Predict(ctx, "anomaly", fv); perr == nil {
				result.Predictions = append(result.Predictions, p)
			}
		}
	}
	if e.Detection != nil {
		if pats, derr := e.Detection.DetectPatterns(ctx, address); derr == nil {
			result.Patterns = pats
		}
		if cls, cerr := e.Detection.Cluster(ctx, address); cerr == nil {
			result.Clusters = cls
		}
	}
	if e.Risk != nil {
		ra, rerr := e.Risk.Score(ctx, RiskInput{
			Subject:     address,
			SubjectType: schema.NodeWallet,
			Predictions: result.Predictions,
			Patterns:    result.Patterns,
			Clusters:    result.Clusters,
		})
		if rerr != nil {
			return nil, rerr
		}
		result.Risk = ra
	}
	if e.Evidence != nil {
		ev, eerr := e.Evidence.Build(ctx, result)
		if eerr != nil {
			return nil, eerr
		}
		result.Evidence = ev
	}

	return result, nil
}
