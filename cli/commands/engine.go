package commands

import (
	"github.com/bctx/bctx/cases/manager"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/sdk"
)

// buildEngine constructs an Engine wired with the services available in this
// phase. The active case's repository is attached when a case is open so that
// status/analysis read from the correct isolated database.
//
// Services not yet implemented (graph, features, ML, detection, risk,
// evidence, reports) are left nil; the engine degrades honestly.
func buildEngine(cfg *configs.Config) (*sdk.Engine, *manager.Manager, func(), error) {
	layout := configs.NewLayout(cfg)
	cases := manager.New(layout)

	opts := sdk.Options{
		Config: cfg,
		Cases:  cases,
	}

	cleanup := func() {}
	if active, ok := cases.Active(); ok {
		repo, err := cases.OpenRepository(active.ID)
		if err == nil {
			opts.Repo = repo
			cleanup = func() { _ = repo.Close() }
		}
	}

	return sdk.NewEngine(opts), cases, cleanup, nil
}
