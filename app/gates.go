package app

import (
	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/configs"
)

// AcquisitionAllowed is the TUI/CLI-shared offline gate. It is deliberately NOT
// the verbatim cli/commands/sync.go:acquisitionAllowed(cfg, gf): it drops the
// globalFlags argument and wraps only the cfg-mode check. This is safe ONLY
// because of an upstream guarantee that MUST hold: loadConfig(gf) folds
// --offline/--airgap into cfg.Network.Mode before the engine is built, and
// configs.Config.AcquisitionAllowed() returns false for both ModeOffline and
// ModeAirgap. Callers must therefore pass the already-flag-applied cfg (the one
// loadConfig produced) — the same cfg handed to Build.
//
// It returns acquisition.ErrOfflineAcquisition when acquisition is not
// permitted, so callers block the action with an honest message instead of
// attempting (and silently failing) a network fetch.
func AcquisitionAllowed(cfg *configs.Config) error {
	if !cfg.AcquisitionAllowed() {
		return acquisition.ErrOfflineAcquisition
	}
	return nil
}
