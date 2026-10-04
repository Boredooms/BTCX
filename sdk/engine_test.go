package sdk

import (
	"context"
	"testing"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/network/state"
)

func TestNewEngineAppliesDefaults(t *testing.T) {
	e := NewEngine(Options{})
	if e.Config() == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestAirgapNeverProbes(t *testing.T) {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeAirgap
	e := NewEngine(Options{Config: cfg})
	if got := e.NetworkStatus(context.Background()); got != state.Airgapped {
		t.Fatalf("airgap status = %q, want AIRGAPPED", got)
	}
}

func TestOfflineModeReportsDisconnected(t *testing.T) {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	e := NewEngine(Options{Config: cfg})
	if got := e.NetworkStatus(context.Background()); got != state.Disconnected {
		t.Fatalf("offline status = %q, want DISCONNECTED", got)
	}
}

func TestAnalyzeWalletWithoutRepoFailsHonestly(t *testing.T) {
	e := NewEngine(Options{})
	_, err := e.AnalyzeWallet(context.Background(), "W1", true)
	if err != ErrNotImplemented {
		t.Fatalf("err = %v, want ErrNotImplemented", err)
	}
}
