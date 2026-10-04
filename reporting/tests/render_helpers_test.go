package tests

import (
	"context"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
)

// fixedTime is the deterministic clock value used so generated_at is stable
// across renders in the determinism tests.
func fixedTime() time.Time { return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC) }

// newFixedService builds a reporting service over a seeded wallet case with a
// fixed clock, and returns a freshly built snapshot ready to render.
func newFixedService(t *testing.T) (*reporting.Service, models.ReportSnapshot) {
	t.Helper()
	repo := newRepo(t)
	seedWallet(t, repo)
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	snap, err := svc.BuildSnapshot(context.Background(), richResult())
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	return svc, snap
}

// richResult returns an InvestigationResult that exercises every section:
// predictions, risk signals, clusters, propagation and evidence.
func richResult() schema.InvestigationResult {
	return schema.InvestigationResult{
		ID:          "inv-W1",
		CaseID:      "case-1",
		Subject:     "W1",
		SubjectType: schema.NodeWallet,
		Offline:     true,
		CreatedAt:   fixedTime(),
		Risk: schema.RiskAssessment{
			Subject:    "W1",
			Score:      42,
			Confidence: 0.73,
			Signals: []schema.RiskSignal{
				{Name: "fan_out", Score: 0.6, Weight: 0.5, Description: "high fan-out observed"},
				{Name: "mixing_like", Score: 0.4, Weight: 0.5, Description: "mixing-like pattern match"},
			},
		},
		Predictions: []schema.Prediction{
			{Model: "anomaly", ModelVersion: "v1", FeatureSchema: schema.FeatureSchemaVersion, Subject: "W1", Score: 0.5, Confidence: 0.8},
			{Model: "cluster", ModelVersion: "v2", FeatureSchema: schema.FeatureSchemaVersion, Subject: "W1", Score: 0.3, Confidence: 0.6},
		},
		Clusters: []schema.EntityCluster{
			{ID: "cl1", Members: []string{"W1", "W3"}, Confidence: 0.7, Basis: []string{"common_input"}},
		},
		Propagation: []schema.RiskPropagationStep{
			{Seed: "W1", Target: "W2", Distance: 1, Contribution: 0.25, Path: []string{"W1", "W2"}},
		},
		Evidence: []schema.EvidenceItem{
			{ID: "ev2", Type: "feature", Severity: schema.SeverityMed, Description: "fan-out anomaly", SourceRecords: []string{"TXB", "TXA"}, Confidence: 0.6},
			{ID: "ev1", Type: "pattern", Severity: schema.SeverityHigh, Description: "mixing-like pattern match", SourceRecords: []string{"TXA"}, Confidence: 0.8},
		},
		RelatedWallets: 2,
	}
}
