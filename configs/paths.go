package configs

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout describes the BCTX runtime directory layout rooted at DataDir.
type Layout struct {
	Root       string // ~/.bctx
	Cases      string // ~/.bctx/cases
	Models     string // ~/.bctx/models
	Geo        string // ~/.bctx/geo
	Cache      string // ~/.bctx/cache
	Logs       string // ~/.bctx/logs
	Tmp        string // ~/.bctx/tmp
	Extensions string // ~/.bctx/extensions
}

// NewLayout derives the directory layout from a config.
func NewLayout(cfg *Config) Layout {
	root := cfg.App.DataDir
	return Layout{
		Root:       root,
		Cases:      filepath.Join(root, "cases"),
		Models:     cfg.Models.Directory,
		Geo:        filepath.Join(root, "geo"),
		Cache:      filepath.Join(root, "cache"),
		Logs:       filepath.Join(root, "logs"),
		Tmp:        filepath.Join(root, "tmp"),
		Extensions: filepath.Join(root, "extensions"),
	}
}

// EnsureAll creates every runtime directory. This performs no network access
// and is safe to run in airgap mode.
func (l Layout) EnsureAll() error {
	for _, dir := range []string{l.Root, l.Cases, l.Models, l.Geo, l.Cache, l.Logs, l.Tmp, l.Extensions} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// CaseDir returns the directory for a specific case id.
func (l Layout) CaseDir(caseID string) string {
	return filepath.Join(l.Cases, caseID)
}

// Exists reports whether the root data directory has been initialized.
func (l Layout) Exists() bool {
	info, err := os.Stat(l.Root)
	return err == nil && info.IsDir()
}
