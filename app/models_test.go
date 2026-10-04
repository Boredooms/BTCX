package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bctx/bctx/configs"
)

// chdir changes the working directory to dir for the duration of the test and
// restores it on cleanup. (Go 1.23 has no t.Chdir; this is the equivalent.)
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// writeModelManifests creates anomaly/manifest.json and flow/manifest.json
// under dir so dirHasModels(dir) is true. Content is irrelevant to resolution
// (only presence is checked).
func writeModelManifests(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"anomaly", "flow"} {
		sub := filepath.Join(dir, name)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		if err := os.WriteFile(filepath.Join(sub, "manifest.json"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write manifest %s: %v", sub, err)
		}
	}
}

// TestModelsDirRelease: the configured dir holds both manifests and the CWD is
// a non-repo dir with no ./models. Resolution returns the configured dir
// (release / operator-install case) → MODELS = LOADED.
func TestModelsDirRelease(t *testing.T) {
	// Non-repo CWD with no ./models present.
	chdir(t, t.TempDir())

	installed := t.TempDir()
	writeModelManifests(t, installed)

	cfg := configs.Default()
	cfg.Models.Directory = installed

	if got := ModelsDir(cfg); got != installed {
		t.Errorf("release: ModelsDir = %q, want configured dir %q", got, installed)
	}
}

// TestModelsDirDevCheckout: ./models (relative to CWD) holds both manifests
// while the configured dir is empty — the dev-checkout case. Resolution falls
// through to the repo-local "models" path.
func TestModelsDirDevCheckout(t *testing.T) {
	repo := t.TempDir()
	chdir(t, repo)
	writeModelManifests(t, filepath.Join(repo, "models"))

	// Configured dir exists but is empty (mirrors a populated-but-empty
	// ~/.bctx/models that must NOT shadow the dev ./models).
	cfg := configs.Default()
	cfg.Models.Directory = t.TempDir()

	if got := ModelsDir(cfg); got != "models" {
		t.Errorf("dev checkout: ModelsDir = %q, want \"models\"", got)
	}
}

// TestModelsDirMissing: neither the configured dir nor ./models hold models.
// Resolution returns the configured dir so MODELS reports MISSING honestly
// (never a fabricated path).
func TestModelsDirMissing(t *testing.T) {
	chdir(t, t.TempDir()) // no ./models here

	configured := t.TempDir() // exists but empty
	cfg := configs.Default()
	cfg.Models.Directory = configured

	if got := ModelsDir(cfg); got != configured {
		t.Errorf("missing: ModelsDir = %q, want configured dir %q", got, configured)
	}
}

// TestModelsDirConfiguredWinsOverDev: when BOTH the configured dir and ./models
// hold models, the configured dir wins (step 1 before step 2) — an operator
// override is respected.
func TestModelsDirConfiguredWinsOverDev(t *testing.T) {
	repo := t.TempDir()
	chdir(t, repo)
	writeModelManifests(t, filepath.Join(repo, "models"))

	installed := t.TempDir()
	writeModelManifests(t, installed)

	cfg := configs.Default()
	cfg.Models.Directory = installed

	if got := ModelsDir(cfg); got != installed {
		t.Errorf("configured-wins: ModelsDir = %q, want %q", got, installed)
	}
}

// TestDirHasModels covers the presence predicate directly: both manifests
// required, missing either → false.
func TestDirHasModels(t *testing.T) {
	if dirHasModels("") {
		t.Error("empty dir must be false")
	}
	only := t.TempDir()
	if err := os.MkdirAll(filepath.Join(only, "anomaly"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(only, "anomaly", "manifest.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dirHasModels(only) {
		t.Error("only anomaly manifest present must be false (flow missing)")
	}
	both := t.TempDir()
	writeModelManifests(t, both)
	if !dirHasModels(both) {
		t.Error("both manifests present must be true")
	}
}
