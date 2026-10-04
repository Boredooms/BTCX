// Package app is the single wiring seam shared by the BCTX CLI and TUI. It
// exists so neither front end duplicates engine construction, the offline gate,
// the models-directory resolution, or the analysis-service constructors (audit
// risk #1). The dependency direction is strictly:
//
//	CLI / TUI  ->  app  ->  existing BCTX services  ->  repository/domain
//
// app owns NO canonical forensic state and runs NO business logic itself: it
// constructs the Engine exactly as cli/commands/engine.go:buildEngine does
// (Engine service fields left nil — analysis services are built on demand in
// services.go), attaches the active case's repository, and hands out
// correctly-scoped services. All operations are local; nothing here dials the
// network.
package app

import (
	"github.com/bctx/bctx/cases/manager"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/sdk"
)

// App is the shared bootstrap result. It mirrors the tuple returned by the
// legacy buildEngine (*sdk.Engine, *manager.Manager, func(), error) as a named
// struct so the CLI and TUI share exactly one wiring path.
type App struct {
	// Engine exposes Config/Layout/NetworkStatus only. Its service fields
	// (Graph/Features/ML/Detection/Risk/Evidence/Reports) are intentionally
	// NIL — the engine degrades honestly and callers must obtain analysis
	// services from services.go, never from Engine.* (which are nil).
	Engine *sdk.Engine
	// Cases is the case-lifecycle manager (Active/Open/List/OpenRepository).
	// It is case-agnostic and preserved across ReopenCase.
	Cases *manager.Manager
	// Repo is the active case's repository, or nil when no case is open.
	Repo sdk.Repository
	// CaseID is the active case id, or "" when no case is open.
	CaseID string
	// Cleanup closes the active-case repo (and any per-run registries). It is
	// always safe to call and always non-nil; defer app.Cleanup() after Build.
	Cleanup func()
}

// Build wires exactly what buildEngine wires: an Engine with nil service
// fields, the Cases manager, and — when a case is open — the active-case Repo
// plus a Cleanup that closes it. It constructs no analysis services itself;
// those come from services.go on demand so each screen receives a fresh,
// correctly case-scoped instance. The cfg passed in MUST already have had any
// --offline/--airgap flags folded into cfg.Network.Mode by the caller
// (loadConfig), because the offline gate (gates.go) relies on that upstream
// fold.
func Build(cfg *configs.Config) (*App, error) {
	layout := configs.NewLayout(cfg)
	cases := manager.New(layout)

	opts := sdk.Options{
		Config: cfg,
		Cases:  cases,
	}

	a := &App{
		Cases:   cases,
		Cleanup: func() {},
	}

	if active, ok := cases.Active(); ok {
		repo, err := cases.OpenRepository(active.ID)
		if err == nil {
			opts.Repo = repo
			a.Repo = repo
			a.CaseID = active.ID
			a.Cleanup = func() { _ = repo.Close() }
		}
	}

	a.Engine = sdk.NewEngine(opts)
	return a, nil
}

// ReopenCase switches the active case in place. It is the single, committed
// case-switch contract (design §1.1): it closes the old active-case repo via
// the existing Cleanup, opens the new case's repo through the Cases manager,
// and resets Repo/CaseID + re-points Cleanup. Engine and Cases are preserved
// (they are case-agnostic). Because the old repo handle is closed before the
// new one opens, no cross-case data can survive.
//
// A full Cleanup()+Build() rebuild is NOT the case-switch mechanism; it is
// reserved for process teardown. On error the App is left on its previous case
// (the old repo is closed only once the new one opens successfully).
func (a *App) ReopenCase(id string) error {
	repo, err := a.Cases.OpenRepository(id)
	if err != nil {
		return err
	}
	// New repo is open; now retire the old one and swap in place.
	if a.Cleanup != nil {
		a.Cleanup()
	}
	a.Repo = repo
	a.CaseID = id
	a.Cleanup = func() { _ = repo.Close() }
	return nil
}
