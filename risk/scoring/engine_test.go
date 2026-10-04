package scoring

import (
	"testing"

	"github.com/bctx/bctx/pkg/schema"
)

func TestScoreDeterministicAndBounded(t *testing.T) {
	e := New(DefaultWeights())
	in := Inputs{
		Subject:        "W123",
		SubjectType:    schema.NodeWallet,
		AnomalyScore:   0.93,
		AnomalyConf:    0.93,
		FlowSuspicion:  0.88,
		FlowTopClass:   "peeling_chain",
		FlowConf:       0.9,
		EntityStrength: 0.81,
		EntityMembers:  18,
	}
	a := e.Score(in)
	b := e.Score(in)
	if a.Score != b.Score || a.Confidence != b.Confidence {
		t.Fatalf("non-deterministic: %+v vs %+v", a, b)
	}
	if a.Score < 0 || a.Score > 100 {
		t.Fatalf("score out of range: %d", a.Score)
	}
	if len(a.Signals) != 3 {
		t.Fatalf("expected 3 signals, got %d", len(a.Signals))
	}
	// With these inputs the combined score should be high.
	if a.Score < 80 {
		t.Fatalf("expected elevated risk, got %d", a.Score)
	}
}

func TestZeroSignalsLowRisk(t *testing.T) {
	e := New(DefaultWeights())
	a := e.Score(Inputs{Subject: "Wq", SubjectType: schema.NodeWallet})
	if a.Score != 0 {
		t.Fatalf("expected 0 risk for zero signals, got %d", a.Score)
	}
}

func TestDelta(t *testing.T) {
	e := New(DefaultWeights())
	prev := 60
	a := e.Score(Inputs{
		Subject: "Wd", SubjectType: schema.NodeWallet,
		AnomalyScore: 0.9, Previous: &prev,
	})
	if a.Delta == nil || a.Previous == nil {
		t.Fatal("expected delta/previous set")
	}
	if *a.Delta != a.Score-prev {
		t.Fatalf("delta mismatch: %d vs %d", *a.Delta, a.Score-prev)
	}
}
