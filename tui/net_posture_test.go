package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/storage/sqlite"
	tea "github.com/charmbracelet/bubbletea"
)

// TestNetPostureIsAirgappedFirst asserts the top bar presents the air-gapped
// posture — DISCONNECTED at rest (online/offline mode) and AIRGAPPED under the
// airgap policy lock — and NEVER a live-probe "CONNECTED" headline, matching the
// offline-first forensic identity.
func TestNetPostureIsAirgappedFirst(t *testing.T) {
	build := func(mode configs.NetworkMode) *Root {
		repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "n.db"))
		if err != nil {
			t.Fatalf("repo: %v", err)
		}
		t.Cleanup(func() { _ = repo.Close() })
		cfg := configs.Default()
		cfg.Network.Mode = mode
		built, err := app.Build(cfg)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		a := &app.App{Repo: repo, CaseID: "n", Cleanup: func() {}, Engine: built.Engine, Cases: built.Cases}
		r := NewRoot(context.Background(), a)
		r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
		r.refreshStatus()
		return r
	}

	// Online mode: resting posture is DISCONNECTED (never CONNECTED).
	r := build(configs.ModeOnline)
	if got := string(r.state.Network); got != "DISCONNECTED" {
		t.Errorf("online resting NET = %q, want DISCONNECTED (air-gapped-first)", got)
	}
	out := r.View()
	if strings.Contains(out, "NET CONNECTED") {
		t.Errorf("top bar must not advertise CONNECTED for an air-gapped tool:\n%s",
			firstLine(out))
	}

	// Airgap mode: posture is AIRGAPPED.
	ra := build(configs.ModeAirgap)
	if got := string(ra.state.Network); got != "AIRGAPPED" {
		t.Errorf("airgap NET = %q, want AIRGAPPED", got)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
