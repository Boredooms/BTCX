package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func TestSaveAndGetTransaction(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	ts := time.Now().UTC().Truncate(time.Second)

	tx := schema.Transaction{
		TxID:       "TX1",
		Timestamp:  ts,
		FeeBTC:     0.0001,
		Inputs:     []schema.TransactionInput{{Address: "W1", AmountBTC: 2.0, Index: 0}},
		Outputs:    []schema.TransactionOutput{{Address: "W2", AmountBTC: 1.9, Index: 0}},
		Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.GetTransaction(ctx, "TX1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected transaction, got nil")
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Address != "W1" {
		t.Fatalf("inputs mismatch: %+v", got.Inputs)
	}
	if len(got.Outputs) != 1 || got.Outputs[0].Address != "W2" {
		t.Fatalf("outputs mismatch: %+v", got.Outputs)
	}

	// Wallets are upserted from inputs/outputs.
	w, err := repo.GetWallet(ctx, "W1")
	if err != nil || w == nil {
		t.Fatalf("expected wallet W1, err=%v", err)
	}

	// WalletTransactions should find the tx by either address.
	txs, err := repo.WalletTransactions(ctx, "W2", 0)
	if err != nil {
		t.Fatalf("wallet txs: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("want 1 tx for W2, got %d", len(txs))
	}
}

func TestCountsAndAlerts(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	alert := schema.Alert{
		ID: "a1", CaseID: "c1", Subject: "W1", SubjectType: schema.NodeWallet,
		Type: "pattern", Risk: 91, Confidence: 0.93, Priority: 1,
		Status: schema.AlertNew, CreatedAt: time.Now().UTC(),
	}
	if err := repo.SaveAlert(ctx, alert); err != nil {
		t.Fatalf("save alert: %v", err)
	}
	alerts, err := repo.ListAlerts(ctx, 10)
	if err != nil || len(alerts) != 1 {
		t.Fatalf("list alerts err=%v n=%d", err, len(alerts))
	}
	c, err := repo.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if c.Alerts != 1 {
		t.Fatalf("alerts count = %d, want 1", c.Alerts)
	}
}

func TestSaveGetListReports(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	row := sdk.ReportRow{
		ID: "R1", CaseID: "c1", Version: 1, Subject: "W1",
		ResultJSON: `{"id":"inv1"}`, GeneratedAt: "2024-01-02T03:04:05Z",
		ModelVersions: `{"m":"v1"}`, DatasetSnapshot: "ds1",
		ReportSchemaVersion: "report-schema-v1", GeneratorVersion: "gen-v1",
		InvestigationID: "inv1", SubjectType: "wallet",
		SnapshotSHA256: "abc123", SnapshotJSON: `{"snap":true}`, GeneratedBy: "tester",
	}
	if err := repo.SaveReport(ctx, row); err != nil {
		t.Fatalf("save report: %v", err)
	}

	got, err := repo.GetReport(ctx, "R1")
	if err != nil || got == nil {
		t.Fatalf("get report err=%v nil=%v", err, got == nil)
	}
	if *got != row {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", *got, row)
	}

	// Missing report returns (nil, nil).
	missing, err := repo.GetReport(ctx, "nope")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing report")
	}

	// Insert a legacy-style row touching only the 0001 columns; the 0007
	// columns must read back empty.
	if _, err := repo.db.ExecContext(ctx,
		`INSERT INTO reports (id, case_id, version, subject, result_json, generated_at)
         VALUES (?,?,?,?,?,?)`,
		"R0", "c1", 1, "W0", `{"id":"inv0"}`, "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	legacy, err := repo.GetReport(ctx, "R0")
	if err != nil || legacy == nil {
		t.Fatalf("get legacy err=%v nil=%v", err, legacy == nil)
	}
	if legacy.ReportSchemaVersion != "" || legacy.GeneratorVersion != "" ||
		legacy.InvestigationID != "" || legacy.SubjectType != "" ||
		legacy.SnapshotSHA256 != "" || legacy.SnapshotJSON != "" ||
		legacy.GeneratedBy != "" || legacy.ModelVersions != "" ||
		legacy.DatasetSnapshot != "" {
		t.Fatalf("legacy 0007 columns should be empty, got %+v", *legacy)
	}

	// ListReports orders by generated_at then id: R0 (earlier) before R1.
	list, err := repo.ListReports(ctx)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 reports, got %d", len(list))
	}
	if list[0].ID != "R0" || list[1].ID != "R1" {
		t.Fatalf("ordering mismatch: %s, %s", list[0].ID, list[1].ID)
	}
}

func TestSaveReportExport(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	exp := sdk.ReportExportRow{
		ExportID: "E1", ReportID: "R1", CaseID: "c1",
		BundlePath: "/case/exports/E1", Formats: "json,md,pdf",
		ManifestSHA256: "deadbeef", CreatedAt: "2024-01-02T03:04:05Z",
	}
	if err := repo.SaveReportExport(ctx, exp); err != nil {
		t.Fatalf("save report export: %v", err)
	}
	var got sdk.ReportExportRow
	err := repo.db.QueryRowContext(ctx,
		`SELECT export_id, report_id, case_id, bundle_path, formats, manifest_sha256, created_at
         FROM report_exports WHERE export_id = ?`, "E1").Scan(
		&got.ExportID, &got.ReportID, &got.CaseID, &got.BundlePath,
		&got.Formats, &got.ManifestSHA256, &got.CreatedAt)
	if err != nil {
		t.Fatalf("read back export: %v", err)
	}
	if got != exp {
		t.Fatalf("export round-trip mismatch:\n got=%+v\nwant=%+v", got, exp)
	}
}

func TestListMonitorEventsOrdering(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	// Insert out of order; expect timestamp then event_id ordering.
	events := []sdk.MonitorEventRow{
		{EventID: "e2", SessionID: "s1", Type: "tx", TxID: "T2", Confirmed: true, BlockHeight: 10, FirstSeen: "2024-01-01T00:00:02Z", Timestamp: "2024-01-01T00:00:02Z"},
		{EventID: "e1", SessionID: "s1", Type: "tx", TxID: "T1", FirstSeen: "2024-01-01T00:00:01Z", Timestamp: "2024-01-01T00:00:01Z"},
		{EventID: "e3b", SessionID: "s1", Type: "tx", TxID: "T3", FirstSeen: "2024-01-01T00:00:03Z", Timestamp: "2024-01-01T00:00:03Z"},
		{EventID: "e3a", SessionID: "s1", Type: "tx", TxID: "T3", FirstSeen: "2024-01-01T00:00:03Z", Timestamp: "2024-01-01T00:00:03Z"},
		{EventID: "other", SessionID: "s2", Type: "tx", FirstSeen: "2024-01-01T00:00:00Z", Timestamp: "2024-01-01T00:00:00Z"},
	}
	for _, e := range events {
		if err := repo.SaveMonitorEvent(ctx, e); err != nil {
			t.Fatalf("save event: %v", err)
		}
	}

	got, err := repo.ListMonitorEvents(ctx, "s1", 0)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	wantOrder := []string{"e1", "e2", "e3a", "e3b"}
	if len(got) != len(wantOrder) {
		t.Fatalf("want %d events, got %d", len(wantOrder), len(got))
	}
	for i, id := range wantOrder {
		if got[i].EventID != id {
			t.Fatalf("event order[%d]=%s, want %s", i, got[i].EventID, id)
		}
	}
	if !got[1].Confirmed {
		t.Fatalf("e2 confirmed flag lost")
	}

	// Limit is honored.
	limited, err := repo.ListMonitorEvents(ctx, "s1", 2)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(limited) != 2 || limited[0].EventID != "e1" || limited[1].EventID != "e2" {
		t.Fatalf("limit not honored: %+v", limited)
	}
}

func TestListRiskDeltasOrdering(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	deltas := []sdk.RiskDeltaRow{
		{ID: "d2", SessionID: "s1", Subject: "W1", PreviousScore: 10, CurrentScore: 20, Delta: 10, ChangedSignals: []string{"sig1"}, NewPatterns: []string{"p1"}, NewEvidenceIDs: []string{"ev1"}, Timestamp: "2024-01-01T00:00:02Z"},
		{ID: "d1", SessionID: "s1", Subject: "W1", PreviousScore: 0, CurrentScore: 10, Delta: 10, Timestamp: "2024-01-01T00:00:01Z"},
		{ID: "d3", SessionID: "s2", Subject: "W9", Timestamp: "2024-01-01T00:00:00Z"},
	}
	for _, d := range deltas {
		if err := repo.SaveRiskDelta(ctx, d); err != nil {
			t.Fatalf("save delta: %v", err)
		}
	}

	got, err := repo.ListRiskDeltas(ctx, "s1")
	if err != nil {
		t.Fatalf("list deltas: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 deltas, got %d", len(got))
	}
	if got[0].ID != "d1" || got[1].ID != "d2" {
		t.Fatalf("delta order mismatch: %s, %s", got[0].ID, got[1].ID)
	}
	// JSON array columns round-trip.
	if len(got[1].ChangedSignals) != 1 || got[1].ChangedSignals[0] != "sig1" ||
		len(got[1].NewPatterns) != 1 || got[1].NewPatterns[0] != "p1" ||
		len(got[1].NewEvidenceIDs) != 1 || got[1].NewEvidenceIDs[0] != "ev1" {
		t.Fatalf("json arrays did not round-trip: %+v", got[1])
	}
}

func TestMigration0007Reopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "case.db")

	// First open applies 0001-0007.
	repo, err := NewRepository(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	row := sdk.ReportRow{
		ID: "R1", CaseID: "c1", Version: 1, Subject: "W1",
		ResultJSON: `{"id":"inv1"}`, GeneratedAt: "2024-01-02T03:04:05Z",
		ReportSchemaVersion: "report-schema-v1",
	}
	if err := repo.SaveReport(ctx, row); err != nil {
		t.Fatalf("save report: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Re-open on a DB that already has 0001-0007 applied: no error, data intact.
	repo2, err := NewRepository(path)
	if err != nil {
		t.Fatalf("re-open: %v", err)
	}
	t.Cleanup(func() { _ = repo2.Close() })
	got, err := repo2.GetReport(ctx, "R1")
	if err != nil || got == nil {
		t.Fatalf("get after re-open err=%v nil=%v", err, got == nil)
	}
	if got.ReportSchemaVersion != "report-schema-v1" {
		t.Fatalf("report_schema_version lost after re-open: %q", got.ReportSchemaVersion)
	}
}

func TestGetMissingReturnsNil(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()
	w, err := repo.GetWallet(ctx, "nope")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if w != nil {
		t.Fatal("expected nil for missing wallet")
	}
}
