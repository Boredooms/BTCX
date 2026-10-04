package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
)

// TestWalletPipelineShowsStages asserts the Wallet screen renders the staged
// analysis pipeline, and that folding in a real InvestigationResult marks the
// pre-report stages done with honest details drawn from the result (the risk
// score, evidence count), with the report stage awaiting the r key.
func TestWalletPipelineShowsStages(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	w := NewWallet(ctx)

	// Before any result: the pipeline shows querying running + report skipped.
	out0 := w.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out0, "ANALYSIS PIPELINE") {
		t.Fatalf("wallet should render the analysis pipeline band; got:\n%s", out0)
	}
	if !strings.Contains(out0, "querying local evidence") {
		t.Errorf("pipeline should list the querying stage; got:\n%s", out0)
	}

	// Fold in a real-shaped result: ML prediction + risk + evidence present.
	res := &schema.InvestigationResult{
		Subject: "WFIXTURE",
		Risk:    schema.RiskAssessment{Score: 72, Confidence: 0.85},
		Predictions: []schema.Prediction{
			{Model: "anomaly", Score: 1.0, Confidence: 1.0},
		},
		Evidence: []schema.EvidenceItem{
			{Severity: "HIGH", Description: "peeling_chain-like"},
			{Severity: "MED", Description: "rapid_flow"},
		},
	}
	m, _ := w.Update(dataLoaded{Request: "WFIXTURE", Payload: res})
	w = m.(*Wallet)
	out := w.View(components.Frame{W: 160, H: 50})

	for _, want := range []string{
		"risk 72/100",      // risk stage detail
		"2 evidence items", // evidence stage detail
		"anomaly 1.00",     // ML stage detail
		"press r to build", // report stage awaiting the key
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pipeline missing %q after result; got:\n%s", want, out)
		}
	}
}

// TestWalletPipelinePreview prints the Wallet screen with the pipeline so a
// human can eyeball the staged readout.
func TestWalletPipelinePreview(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	w := NewWallet(ctx)
	res := &schema.InvestigationResult{
		Subject:     "WFIXTURE",
		Risk:        schema.RiskAssessment{Score: 72, Confidence: 0.85},
		Predictions: []schema.Prediction{{Model: "anomaly", Score: 1.0}},
		Evidence:    []schema.EvidenceItem{{Severity: "HIGH", Description: "x"}},
	}
	m, _ := w.Update(dataLoaded{Request: "WFIXTURE", Payload: res})
	w = m.(*Wallet)
	t.Logf("\n%s", w.View(components.Frame{W: 120, H: 40}))
}
