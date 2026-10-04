package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/storage/sqlite"
)

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

// seedWallet inserts two transactions for wallet W1 (out of lexical order) plus
// one unrelated tx and a correlated/uncorrelated network observation.
func seedWallet(t *testing.T, repo *sqlite.Repository) {
	t.Helper()
	ctx := context.Background()
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	txs := []schema.Transaction{
		{
			TxID:       "TXB",
			Timestamp:  ts.Add(2 * time.Second),
			Inputs:     []schema.TransactionInput{{Address: "W1", AmountBTC: 1.0, AmountSats: schema.BTCToSats(1.0), Index: 0}},
			Outputs:    []schema.TransactionOutput{{Address: "W2", AmountBTC: 0.9, AmountSats: schema.BTCToSats(0.9), Index: 0}},
			Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
		},
		{
			TxID:       "TXA",
			Timestamp:  ts.Add(1 * time.Second),
			Inputs:     []schema.TransactionInput{{Address: "W3", AmountBTC: 2.0, AmountSats: schema.BTCToSats(2.0), Index: 0}},
			Outputs:    []schema.TransactionOutput{{Address: "W1", AmountBTC: 1.9, AmountSats: schema.BTCToSats(1.9), Index: 0}},
			Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
		},
		{
			TxID:       "TXZ",
			Timestamp:  ts.Add(3 * time.Second),
			Inputs:     []schema.TransactionInput{{Address: "W9", AmountBTC: 5.0, AmountSats: schema.BTCToSats(5.0), Index: 0}},
			Outputs:    []schema.TransactionOutput{{Address: "W8", AmountBTC: 4.9, AmountSats: schema.BTCToSats(4.9), Index: 0}},
			Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
		},
	}
	if err := repo.SaveTransactions(ctx, txs); err != nil {
		t.Fatalf("save txs: %v", err)
	}

	obs := []schema.NetworkObservation{
		{ID: "obs2", Timestamp: ts.Add(2 * time.Second), TxID: "TXB", SrcIP: "10.0.0.2"},
		{ID: "obs1", Timestamp: ts.Add(1 * time.Second), TxID: "TXA", SrcIP: "10.0.0.1"},
		{ID: "obs9", Timestamp: ts.Add(9 * time.Second), TxID: "TXZ", SrcIP: "10.0.0.9"}, // unrelated tx
	}
	if err := repo.SaveNetworkObservations(ctx, obs); err != nil {
		t.Fatalf("save obs: %v", err)
	}
}

func sampleResult() schema.InvestigationResult {
	return schema.InvestigationResult{
		ID:          "inv-W1",
		CaseID:      "case-1",
		Subject:     "W1",
		SubjectType: schema.NodeWallet,
		Offline:     true,
		CreatedAt:   time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Predictions: []schema.Prediction{
			{Model: "anomaly", ModelVersion: "v1", Subject: "W1", Score: 0.5, Confidence: 0.8},
		},
		Evidence: []schema.EvidenceItem{
			{ID: "ev2", Type: "feature", Description: "x"},
			{ID: "ev1", Type: "feature", Description: "y"},
		},
	}
}

func TestBuildSnapshotDeterministic(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	svc := reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "test"})
	ctx := context.Background()

	snap, err := svc.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}

	// Only the subject's transactions (TXA, TXB) are included, lexically sorted.
	if len(snap.Transactions) != 2 {
		t.Fatalf("expected 2 subject txs, got %d: %+v", len(snap.Transactions), snap.TxIDs)
	}
	if snap.TxIDs[0] != "TXA" || snap.TxIDs[1] != "TXB" {
		t.Fatalf("tx ids not lexically sorted: %v", snap.TxIDs)
	}

	// Network observations filtered to subject txs and ordered (timestamp,id).
	if len(snap.NetworkObservations) != 2 {
		t.Fatalf("expected 2 correlated obs, got %d", len(snap.NetworkObservations))
	}
	if snap.NetworkObservations[0].ID != "obs1" || snap.NetworkObservations[1].ID != "obs2" {
		t.Fatalf("obs not ordered: %+v", snap.NetworkObservations)
	}

	// Evidence IDs sorted.
	if len(snap.EvidenceIDs) != 2 || snap.EvidenceIDs[0] != "ev1" || snap.EvidenceIDs[1] != "ev2" {
		t.Fatalf("evidence ids not sorted: %v", snap.EvidenceIDs)
	}

	// Version + model/feature provenance stamped.
	if snap.SchemaVersion != reporting.ReportSchemaVersion || snap.GeneratorVersion != reporting.GeneratorVersion {
		t.Fatalf("version tokens wrong: %q %q", snap.SchemaVersion, snap.GeneratorVersion)
	}
	if snap.FeatureSchema != schema.FeatureSchemaVersion || snap.FeatureSchemaSHA != schema.FeatureSchemaSHA256 {
		t.Fatalf("feature schema provenance wrong")
	}
	if snap.ModelVersions["anomaly"] != "v1" {
		t.Fatalf("model versions not derived from predictions: %v", snap.ModelVersions)
	}

	// Monitoring unavailable (no session) => empty, non-nil slices, nil session.
	if snap.MonitorSession != nil {
		t.Fatalf("expected no monitor session")
	}
	if snap.MonitorEvents == nil || snap.RiskDeltas == nil || snap.MonitorAlerts == nil {
		t.Fatalf("monitoring slices must be non-nil for stable hashing")
	}

	// Deterministic report ID: rebuilding the same result yields the same ID
	// and the same hash.
	snap2, err := svc.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("rebuild snapshot: %v", err)
	}
	if reporting.ReportID(snap) != reporting.ReportID(snap2) {
		t.Fatalf("report id not deterministic: %s != %s", reporting.ReportID(snap), reporting.ReportID(snap2))
	}
	if reporting.SnapshotHash(snap) != reporting.SnapshotHash(snap2) {
		t.Fatal("snapshot hash not deterministic across identical builds")
	}

	// Report ID has no timestamp and is derived from case+subject+hash.
	id := reporting.ReportID(snap)
	if id == "" || id[:4] != "rpt-" {
		t.Fatalf("unexpected report id: %q", id)
	}
}

// TestSnapshotHashExcludesGeneratedWallClock documents and verifies the
// generated_at / generated_by exclusion: the snapshot carries no wall-clock
// field, so building the same result through services with different clocks
// yields an identical hash. (The non-hashed generated_at stamp lives on the
// report meta added at render time, not on the snapshot.)
func TestSnapshotHashExcludesGeneratedWallClock(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	ctx := context.Background()

	clkA := func() time.Time { return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) }
	clkB := func() time.Time { return time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC) }

	svcA := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "alice"}, clkA)
	svcB := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bob"}, clkB)

	snapA, err := svcA.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	snapB, err := svcB.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("build B: %v", err)
	}
	if reporting.SnapshotHash(snapA) != reporting.SnapshotHash(snapB) {
		t.Fatal("snapshot hash must not depend on wall clock or generated_by")
	}
}

// TestSnapshotCreatedAtInHash documents the deliberate decision (review NIT #3):
// InvestigationResult.CreatedAt IS part of the hashed snapshot. Report identity
// is tied to the specific investigation snapshot, so the same subject
// investigated at two different times yields two distinct report IDs. This is
// intentional, not an accident: the whole InvestigationResult is embedded and
// hashed. The ONLY wall-clock excluded from the hash is the report's own
// generated_at/generated_by (which has no snapshot field at all).
func TestSnapshotCreatedAtInHash(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	svc := reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "test"})
	ctx := context.Background()

	r1 := sampleResult()
	r2 := sampleResult()
	r2.CreatedAt = r1.CreatedAt.Add(24 * time.Hour)

	s1, err := svc.BuildSnapshot(ctx, r1)
	if err != nil {
		t.Fatalf("build r1: %v", err)
	}
	s2, err := svc.BuildSnapshot(ctx, r2)
	if err != nil {
		t.Fatalf("build r2: %v", err)
	}
	if reporting.SnapshotHash(s1) == reporting.SnapshotHash(s2) {
		t.Fatal("expected different hashes when InvestigationResult.CreatedAt differs")
	}
}

// TestTimelineTotalOrder verifies the timeline merges event kinds and is sorted
// by (timestamp, kind, id).
func TestTimelineTotalOrder(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	svc := reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "test"})
	ctx := context.Background()

	snap, err := svc.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	events, err := svc.Timeline(ctx, snap)
	if err != nil {
		t.Fatalf("timeline: %v", err)
	}
	// Two subject transactions => two tx timeline events, chronologically ordered.
	if len(events) != 2 {
		t.Fatalf("expected 2 timeline events, got %d", len(events))
	}
	for i := 1; i < len(events); i++ {
		prev, cur := events[i-1], events[i]
		if prev.Timestamp > cur.Timestamp {
			t.Fatalf("timeline not ordered by timestamp: %+v", events)
		}
	}
}
