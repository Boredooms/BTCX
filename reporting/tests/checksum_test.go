package tests

import (
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
)

// TestCanonicalJSONStableUnderMapReorder proves CanonicalJSON is independent of
// Go map iteration order: the same logical map always serializes identically.
func TestCanonicalJSONStableUnderMapReorder(t *testing.T) {
	a := map[string]any{"b": 1, "a": 2, "c": map[string]any{"z": 9, "y": 8}}
	b := map[string]any{"c": map[string]any{"y": 8, "z": 9}, "a": 2, "b": 1}

	ca, err := reporting.CanonicalJSON(a)
	if err != nil {
		t.Fatalf("canonical a: %v", err)
	}
	cb, err := reporting.CanonicalJSON(b)
	if err != nil {
		t.Fatalf("canonical b: %v", err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("canonical json differs under map reorder:\n a=%s\n b=%s", ca, cb)
	}
	want := `{"a":2,"b":1,"c":{"y":8,"z":9}}`
	if string(ca) != want {
		t.Fatalf("canonical json = %s, want %s", ca, want)
	}
}

// TestSnapshotHashStableUnderReorder proves SnapshotHash does not depend on the
// insertion order of map entries or the order of equivalent slice content when
// the snapshot's total-ordering invariant holds. Two snapshots with identical
// content but ModelVersions maps built in a different order must hash equal.
func TestSnapshotHashStableUnderReorder(t *testing.T) {
	mk := func(models1 map[string]string) models.ReportSnapshot {
		return models.ReportSnapshot{
			SchemaVersion:    reporting.ReportSchemaVersion,
			GeneratorVersion: reporting.GeneratorVersion,
			Case:             models.CaseMeta{ID: "case-1"},
			Result: schema.InvestigationResult{
				ID:          "inv-1",
				CaseID:      "case-1",
				Subject:     "W1",
				SubjectType: schema.NodeWallet,
			},
			Datasets:            []models.RedactedDataset{},
			MonitorEvents:       []sdk.MonitorEventRow{},
			RiskDeltas:          []sdk.RiskDeltaRow{},
			MonitorAlerts:       []sdk.MonitorAlertRow{},
			Alerts:              []schema.Alert{},
			NetworkObservations: []schema.NetworkObservation{},
			Transactions:        []schema.Transaction{},
			ModelVersions:       models1,
			FeatureSchema:       schema.FeatureSchemaVersion,
			FeatureSchemaSHA:    schema.FeatureSchemaSHA256,
			EvidenceIDs:         []string{},
			TxIDs:               []string{},
			AlertIDs:            []string{},
		}
	}

	// Same logical model-version map built in two different orders.
	m1 := map[string]string{}
	m1["anomaly"] = "v1"
	m1["cluster"] = "v2"
	m2 := map[string]string{}
	m2["cluster"] = "v2"
	m2["anomaly"] = "v1"

	h1 := reporting.SnapshotHash(mk(m1))
	h2 := reporting.SnapshotHash(mk(m2))
	if h1 != h2 {
		t.Fatalf("snapshot hash differs under map reorder: %s != %s", h1, h2)
	}
	if h1 == "" {
		t.Fatal("snapshot hash empty")
	}

	// A content change must change the hash.
	m3 := map[string]string{"anomaly": "v1", "cluster": "v3"}
	if reporting.SnapshotHash(mk(m3)) == h1 {
		t.Fatal("snapshot hash did not change when content changed")
	}
}
