package configs

import (
	"path/filepath"
	"testing"
)

func TestDefaultIsOfflineSafe(t *testing.T) {
	cfg := Default()
	if cfg.App.DataDir == "" {
		t.Fatal("expected non-empty data dir")
	}
	if cfg.Network.Mode != ModeOnline {
		t.Fatalf("default mode = %q, want online", cfg.Network.Mode)
	}
	if cfg.LLM.Enabled {
		t.Fatal("LLM must be disabled by default")
	}
}

func TestAcquisitionAllowed(t *testing.T) {
	cfg := Default()
	if !cfg.AcquisitionAllowed() {
		t.Fatal("online default should allow acquisition")
	}
	cfg.Network.Mode = ModeOffline
	if cfg.AcquisitionAllowed() {
		t.Fatal("offline must forbid acquisition")
	}
	cfg.Network.Mode = ModeAirgap
	if cfg.AcquisitionAllowed() {
		t.Fatal("airgap must forbid acquisition")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Network.Mode != ModeOnline {
		t.Fatal("missing file should yield defaults")
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.path = path
	cfg.Network.Mode = ModeAirgap
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Network.Mode != ModeAirgap {
		t.Fatalf("reloaded mode = %q, want airgap", got.Network.Mode)
	}
}

func TestLayoutEnsureAll(t *testing.T) {
	cfg := Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Models.Directory = filepath.Join(cfg.App.DataDir, "models")
	l := NewLayout(cfg)
	if err := l.EnsureAll(); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !l.Exists() {
		t.Fatal("layout root should exist after EnsureAll")
	}
}
