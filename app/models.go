package app

import (
	"os"
	"path/filepath"

	"github.com/bctx/bctx/configs"
)

// ModelsDir resolves the directory the ML inference registry and the
// orchestrator load models from. It fixes audit risk #7 (the CLI's bare
// CWD-relative repoModelsDir()) by checking each candidate for the PRESENCE of
// models before accepting it, so both a release install and a dev checkout
// resolve correctly regardless of the launch CWD (design §1.4):
//
//  1. cfg.Models.Directory — but only if it actually contains models. This is
//     the release / operator-override path and is checked first so a populated
//     install dir wins. (It is NOT accepted blindly: configs.normalize always
//     fills Models.Directory to ~/.bctx/models, so a bare "use configured dir"
//     scheme would shadow a dev ./models with an empty ~/.bctx/models.)
//  2. repo-local "models" if present — the same path the CLI's repoModelsDir()
//     returns, covering a dev checkout where models live at <repo>/models and
//     nothing was copied to ~/.bctx/models.
//  3. otherwise cfg.Models.Directory so MODELS reports MISSING honestly — a
//     path is never fabricated.
func ModelsDir(cfg *configs.Config) string {
	if d := cfg.Models.Directory; d != "" && dirHasModels(d) {
		return d
	}
	if dirHasModels("models") {
		return "models"
	}
	return cfg.Models.Directory
}

// dirHasModels reports whether a candidate directory actually holds loadable
// models: both anomaly/manifest.json AND flow/manifest.json present (the two
// fail-closed models the orchestrator requires). A directory that exists but
// lacks these is treated as "no models".
func dirHasModels(dir string) bool {
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "anomaly", "manifest.json")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "flow", "manifest.json")); err != nil {
		return false
	}
	return true
}
