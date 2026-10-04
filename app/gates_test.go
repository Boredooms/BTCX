package app

import (
	"errors"
	"testing"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/configs"
)

// TestAcquisitionAllowedOnline asserts the gate permits acquisition when the
// config is online with acquisition enabled.
func TestAcquisitionAllowedOnline(t *testing.T) {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOnline
	cfg.Network.AcquisitionEnabled = true

	if err := AcquisitionAllowed(cfg); err != nil {
		t.Fatalf("online acquisition must be allowed, got %v", err)
	}
}

// TestAcquisitionAllowedBlocksOfflineAndAirgap asserts the gate blocks with
// acquisition.ErrOfflineAcquisition under both offline and airgap modes — the
// two modes --offline/--airgap fold into upstream (design §1.2).
func TestAcquisitionAllowedBlocksOfflineAndAirgap(t *testing.T) {
	for _, mode := range []configs.NetworkMode{configs.ModeOffline, configs.ModeAirgap} {
		cfg := configs.Default()
		cfg.Network.Mode = mode
		cfg.Network.AcquisitionEnabled = true // irrelevant: mode wins

		err := AcquisitionAllowed(cfg)
		if !errors.Is(err, acquisition.ErrOfflineAcquisition) {
			t.Errorf("mode %q: want ErrOfflineAcquisition, got %v", mode, err)
		}
	}
}

// TestAcquisitionAllowedBlocksWhenDisabled asserts the gate blocks when
// acquisition is explicitly disabled even in online mode.
func TestAcquisitionAllowedBlocksWhenDisabled(t *testing.T) {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOnline
	cfg.Network.AcquisitionEnabled = false

	if err := AcquisitionAllowed(cfg); !errors.Is(err, acquisition.ErrOfflineAcquisition) {
		t.Errorf("disabled acquisition must block, got %v", err)
	}
}
