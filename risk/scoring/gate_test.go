package scoring

import (
	"testing"

	"github.com/bctx/bctx/pkg/schema"
)

// TestCorroborationGateSpread asserts the recalibrated engine produces a real
// spread: a saturated ML anomaly with NO structural/entity corroboration reads
// much lower than the same anomaly WITH corroboration — so a structurally-
// benign wallet is not forced to HIGH/CRITICAL by an out-of-distribution
// anomaly alone.
func TestCorroborationGateSpread(t *testing.T) {
	e := New(DefaultWeights())

	// Benign: anomaly saturates (1.0) but no structural pattern, no entity.
	benign := e.Score(Inputs{
		AnomalyScore: 1.0, FlowSuspicion: 0.0, EntityStrength: 0.0, Structural: 0.0,
	})
	// Suspicious: same anomaly, but strong structural + flow corroboration.
	suspicious := e.Score(Inputs{
		AnomalyScore: 1.0, FlowSuspicion: 1.0, EntityStrength: 0.3, Structural: 1.0,
	})

	t.Logf("benign score=%d band=%s ; suspicious score=%d band=%s",
		benign.Score, benign.Band, suspicious.Score, suspicious.Band)

	if benign.Score >= suspicious.Score {
		t.Fatalf("benign (%d) should score below suspicious (%d)", benign.Score, suspicious.Score)
	}
	// Benign (uncorroborated saturated anomaly only) must not read HIGH/CRITICAL.
	if benign.Band == schema.BandHigh || benign.Band == schema.BandCritical {
		t.Fatalf("benign wallet must not be HIGH/CRITICAL, got %s (%d)", benign.Band, benign.Score)
	}
	// Suspicious (fully corroborated) should read HIGH or CRITICAL.
	if suspicious.Band != schema.BandHigh && suspicious.Band != schema.BandCritical {
		t.Fatalf("fully corroborated wallet should be HIGH/CRITICAL, got %s (%d)", suspicious.Band, suspicious.Score)
	}
	// A truly empty subject reads LOW.
	empty := e.Score(Inputs{})
	if empty.Band != schema.BandLow {
		t.Fatalf("no-signal subject should be LOW, got %s (%d)", empty.Band, empty.Score)
	}
}
