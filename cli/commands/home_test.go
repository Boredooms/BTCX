package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
)

// TestPrintHomePlainStatusSummary asserts the non-TTY plain branch still prints
// the full status summary and that MODELS is resolved live (no hard-coded
// "PENDING"). It drives printHomePlain directly — the exact code the bare
// `bctx` / `bctx tui` off-TTY path runs.
func TestPrintHomePlainStatusSummary(t *testing.T) {
	// Isolate HOME so configs/cases resolve under a temp dir (no real ~/.bctx).
	t.Setenv("HOME", t.TempDir())

	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	a, err := app.Build(cfg)
	if err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	defer a.Cleanup()

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	if err := printHomePlain(cmd, a, cfg, context.Background(), BuildInfo{Version: "test"}); err != nil {
		t.Fatalf("printHomePlain: %v", err)
	}
	out := buf.String()

	// The summary must include the honest status fields.
	for _, want := range []string{"Network:", "Case:", "Transactions:", "Wallets:", "Models:", "Graph:"} {
		if !strings.Contains(out, want) {
			t.Errorf("plain summary missing %q in:\n%s", want, out)
		}
	}
	// The legacy hard-coded "PENDING" must be gone from the home path.
	if strings.Contains(out, "PENDING") {
		t.Errorf("plain summary must not hard-code MODELS=PENDING:\n%s", out)
	}
	// Offline mode must report an honest network status (never a fake connected).
	if !strings.Contains(out, "DISCONNECTED") {
		t.Errorf("offline mode should report DISCONNECTED network, got:\n%s", out)
	}
}

// TestTuiCommandRegistered asserts the `bctx tui` command exists on the root and
// that registering it did not disturb the existing subcommands (acceptance:
// all existing CLI commands remain available).
func TestTuiCommandRegistered(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "test"})

	wantPresent := []string{
		"tui", "version", "status", "doctor", "case", "dataset", "analyze",
		"graph", "sync", "monitor", "report", "geo", "map",
	}
	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	for _, name := range wantPresent {
		if !have[name] {
			t.Errorf("expected command %q to be registered", name)
		}
	}
}

// TestVersionCommandUnaffected asserts an existing command's output is unchanged
// by the TUI integration (it prints the version summary, not the TUI).
func TestVersionCommandUnaffected(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "9.9.9-test"})
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if !strings.Contains(buf.String(), "9.9.9-test") {
		t.Errorf("version output should include the version, got:\n%s", buf.String())
	}
}
