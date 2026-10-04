package commands

import (
	"context"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/sdk"
)

// acquire_service.go is the concrete app.AcquireService for the TUI. It lives in
// cli/commands — the SOLE composition root that imports the explorer provider —
// so the network-touching code is injected into the TUI rather than imported by
// it, keeping tui/* transport-free (the offline-boundary test stays green).
//
// Every Acquire call re-checks the offline gate and re-reads the active case
// repo, so acquisition is never silent and always targets the current case.

type tuiAcquireService struct {
	gf  *globalFlags
	cfg *configs.Config
}

// newTUIAcquireService builds the injected acquire seam for the TUI. It returns
// nil when acquisition is not permitted (offline/airgap/disabled) so the TUI is
// offline-only and will not offer network acquisition.
func newTUIAcquireService(gf *globalFlags, cfg *configs.Config) app.AcquireService {
	return &tuiAcquireService{gf: gf, cfg: cfg}
}

// onlineCfg returns a copy of the config forced to online mode for an EXPLICIT
// acquisition. BCTX is air-gapped-first: the resting config mode is offline (so
// the top bar honestly shows DISCONNECTED and nothing probes the network), and
// acquisition is the one explicit online moment the user opts into. The --airgap
// flag still hard-blocks here (it is a policy lock, not a resting default), and
// the user's --offline flag is respected too.
func (s *tuiAcquireService) onlineCfg() *configs.Config {
	if s.gf != nil && (s.gf.airgap || s.gf.offline) {
		return s.cfg // honor explicit lock flags; do not override
	}
	c := *s.cfg
	c.Network.Mode = configs.ModeOnline
	c.Network.AcquisitionEnabled = true
	return &c
}

func (s *tuiAcquireService) Available() bool {
	// Explicit acquisition is available unless the user hard-locked airgap/
	// offline via a flag. The resting offline MODE is not a block — acquisition
	// is the explicit online moment (see onlineCfg).
	if s.gf != nil && (s.gf.airgap || s.gf.offline) {
		return false
	}
	return acquisitionAllowed(s.onlineCfg(), s.gf) == nil
}

func (s *tuiAcquireService) ProviderName() string {
	src, err := providerFromConfig(s.cfg)
	if err != nil {
		return s.cfg.Acquisition.Provider
	}
	return src.ProviderName()
}

// Acquire runs one explicit acquisition against the active case. It enforces the
// offline gate BEFORE constructing any provider (no silent network), builds the
// engine, dispatches by kind, and returns a provider-neutral result even on
// partial/cancelled so the TUI reports honestly.
func (s *tuiAcquireService) Acquire(ctx context.Context, req app.AcquireRequest) (app.AcquireResult, error) {
	cfg := s.onlineCfg()
	if err := acquisitionAllowed(cfg, s.gf); err != nil {
		return app.AcquireResult{}, err
	}
	var res app.AcquireResult
	err := withActiveRepo(s.gf, func(repo sdk.Repository) error {
		active, _ := caseManagerActive(s.gf)
		src, perr := providerFromConfig(cfg)
		if perr != nil {
			return perr
		}
		eng := acquisition.NewEngine(src, repo, active, engineConfig(cfg))
		defer eng.Close()

		var st acquisition.Stats
		var serr error
		switch req.Kind {
		case app.AcquireWallet:
			st, serr = eng.SyncWallet(ctx, req.Target, acquisition.PaginationState{})
		case app.AcquireTx:
			st, serr = eng.SyncTransaction(ctx, req.Target)
		case app.AcquireBlock:
			st, serr = eng.SyncBlock(ctx, req.Target, req.Height, req.ByHeight)
		default:
			return acquisition.ErrInvalidTarget
		}
		res = toAcquireResult(st, req)
		return serr
	})
	return res, err
}

func toAcquireResult(st acquisition.Stats, req app.AcquireRequest) app.AcquireResult {
	subject := st.Target // for by-height blocks this is the resolved hash
	if subject == "" {
		subject = req.Target
	}
	return app.AcquireResult{
		Provider: st.Provider, Target: st.Target, TargetType: st.TargetType,
		Status: st.Status, Discovered: st.Discovered, Fetched: st.Fetched,
		New: st.New, Duplicates: st.Duplicates, Partial: st.Partial,
		Retries: st.Retries, ElapsedMS: st.Elapsed.Milliseconds(), Subject: subject,
	}
}
