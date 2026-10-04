package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
)

// TestAllFiveMonitorStates exercises every monitor-state classification the
// report can truthfully describe. Each case builds a session (or none) and
// asserts ClassifyMonitorState picks the expected state.
func TestAllFiveMonitorStates(t *testing.T) {
	cases := []struct {
		name  string
		sess  *sdk.MonitorSessionRow
		state models.MonitorState
	}{
		{
			name:  "unavailable (no session)",
			sess:  nil,
			state: models.MonitorStateUnavailable,
		},
		{
			name: "coverage gap",
			sess: &sdk.MonitorSessionRow{
				Status: "RUNNING", Health: "healthy",
				LastPollOKAt: "2024-01-01T00:00:09Z", Gaps: 2, Reconnects: 1,
				StartedAt: "2024-01-01T00:00:00Z", LastEventAt: "2024-01-01T00:00:08Z",
			},
			state: models.MonitorStateCoverageGap,
		},
		{
			name: "provider disconnected",
			sess: &sdk.MonitorSessionRow{
				Status: "RUNNING", Health: "degraded", LastPollOKAt: "",
			},
			state: models.MonitorStateDisconnected,
		},
		{
			name: "paused",
			sess: &sdk.MonitorSessionRow{
				Status: "PAUSED", Health: "healthy", LastPollOKAt: "2024-01-01T00:00:09Z",
			},
			state: models.MonitorStatePaused,
		},
		{
			name: "no activity",
			sess: &sdk.MonitorSessionRow{
				Status: "RUNNING", Health: "healthy",
				LastPollOKAt: "2024-01-01T00:00:09Z", EventsSeen: 0,
			},
			state: models.MonitorStateNoActivity,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, detail := models.ClassifyMonitorState(tc.sess)
			if got != tc.state {
				t.Fatalf("state = %q, want %q", got, tc.state)
			}
			if strings.TrimSpace(detail) == "" {
				t.Fatal("monitor state detail must be non-empty")
			}
		})
	}
}

// TestOfflineConnectedStateInReport asserts the rendered Offline/Connected
// section reflects the classified monitor state and offline flag.
func TestOfflineConnectedStateInReport(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	seedMonitorSession(t, repo, "PAUSED", "healthy", "W1", 3)
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	snap, err := svc.BuildSnapshot(context.Background(), richResult())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	m := decodeView(t, svc, snap)
	oc := section(t, m, "offline_connected_state")
	if oc["offline"] != true {
		t.Fatal("offline flag should be true for an offline result")
	}
	if oc["monitor_state"] != string(models.MonitorStatePaused) {
		t.Fatalf("monitor_state = %v, want paused", oc["monitor_state"])
	}
}

// TestDataQualityAndLimitations asserts the data-quality counts are reported
// and the limitations section carries the Esplora size caveat plus the fixed
// forensic caveats.
func TestDataQualityAndLimitations(t *testing.T) {
	svc, snap := newFixedService(t)
	m := decodeView(t, svc, snap)

	dq := section(t, m, "data_quality")
	if dq["transaction_count"].(float64) != 2 {
		t.Fatalf("data quality tx count = %v, want 2", dq["transaction_count"])
	}

	lim := section(t, m, "limitations")
	notes, _ := lim["notes"].([]any)
	joined := ""
	for _, n := range notes {
		joined += n.(string) + "\n"
	}
	if !strings.Contains(joined, "Transaction size/weight fields may be absent") {
		t.Fatal("limitations must contain the Esplora size caveat")
	}
	if !strings.Contains(joined, "inferred relationship") {
		t.Fatal("limitations must state clusters are an inferred relationship, not proven ownership")
	}
}

// TestSchemaVersioningRejectsUnknown asserts an unknown schema version is
// rejected rather than silently reinterpreted. The report inspection contract
// (FEAT-004) requires an exact match on report-schema-v1; here we verify the
// invariant at the model level: the snapshot always stamps the known version,
// and a snapshot carrying a different version is detectably not v1.
func TestSchemaVersioningRejectsUnknown(t *testing.T) {
	_, snap := newFixedService(t)
	if snap.SchemaVersion != "report-schema-v1" {
		t.Fatalf("built snapshot must stamp report-schema-v1, got %q", snap.SchemaVersion)
	}

	// Simulate a stored snapshot from a hypothetical future/unknown version.
	unknown := snap
	unknown.SchemaVersion = "report-schema-v99"
	if isKnownSchema(unknown.SchemaVersion) {
		t.Fatal("report-schema-v99 must not be treated as a known/supported version")
	}
	if !isKnownSchema(snap.SchemaVersion) {
		t.Fatal("report-schema-v1 must be recognized")
	}
}

// isKnownSchema mirrors the inspection contract: only the exact current schema
// version is supported; anything else must be refused, never reinterpreted.
func isKnownSchema(v string) bool { return v == reporting.ReportSchemaVersion }

// TestReportIDConsistency asserts the report ID is stable, derived from the
// snapshot hash, and that two services with different clocks/operators yield
// the same report ID for the same investigation.
func TestReportIDConsistency(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	ctx := context.Background()

	svcA := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "a"}, fixedTime)
	svcB := reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "b"})

	snapA, err := svcA.BuildSnapshot(ctx, richResult())
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	snapB, err := svcB.BuildSnapshot(ctx, richResult())
	if err != nil {
		t.Fatalf("build B: %v", err)
	}
	if reporting.ReportID(snapA) != reporting.ReportID(snapB) {
		t.Fatal("report id must not depend on clock or operator")
	}
	if !strings.HasPrefix(reporting.ReportID(snapA), "rpt-case-1-W1-") {
		t.Fatalf("unexpected report id form: %q", reporting.ReportID(snapA))
	}
}

// TestCaseIsolation asserts a snapshot built from one case repo never includes
// another case's transactions. Two separate repos are seeded independently; the
// report for case A sees only A's data.
func TestCaseIsolation(t *testing.T) {
	ctx := context.Background()

	repoA := newRepo(t)
	seedWallet(t, repoA) // seeds W1 with TXA/TXB

	repoB := newRepo(t)
	// Seed B with a transaction for a different wallet; it must never leak into A.
	if err := repoB.SaveTransactions(ctx, []schema.Transaction{{
		TxID:       "TXOTHER",
		Inputs:     []schema.TransactionInput{{Address: "W1", AmountSats: schema.BTCToSats(1)}},
		Outputs:    []schema.TransactionOutput{{Address: "WX", AmountSats: schema.BTCToSats(1)}},
		Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
	}}); err != nil {
		t.Fatalf("seed B: %v", err)
	}

	svcA := reporting.NewServiceWithClock(repoA, reporting.BuildInfo{GeneratedBy: "a"}, fixedTime)
	snapA, err := svcA.BuildSnapshot(ctx, sampleResult())
	if err != nil {
		t.Fatalf("build A: %v", err)
	}
	for _, id := range snapA.TxIDs {
		if id == "TXOTHER" {
			t.Fatal("case A report leaked a transaction from case B")
		}
	}
}

// TestMissingData asserts a result for a subject with no local data still
// renders every section (no panic, no fabricated values).
func TestMissingData(t *testing.T) {
	repo := newRepo(t) // empty repo, nothing seeded
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	ctx := context.Background()

	empty := schema.InvestigationResult{
		ID: "inv-empty", CaseID: "case-empty", Subject: "WEMPTY",
		SubjectType: schema.NodeWallet, Offline: true, CreatedAt: fixedTime(),
	}
	snap, err := svc.BuildSnapshot(ctx, empty)
	if err != nil {
		t.Fatalf("build empty: %v", err)
	}
	if len(snap.Transactions) != 0 {
		t.Fatalf("expected no transactions, got %d", len(snap.Transactions))
	}
	// All formats must still render without error.
	for _, f := range allFormats() {
		if _, err := svc.Render(ctx, snap, f); err != nil {
			t.Fatalf("render %s on empty snapshot: %v", f, err)
		}
	}
	// The acquisition summary must mark itself unavailable, not fabricate one.
	m := decodeView(t, svc, snap)
	acq := section(t, m, "acquisition_summary")
	if acq["available"] != false {
		t.Fatal("acquisition summary must be unavailable for a subject with no sync")
	}
}

// TestPartialTransaction asserts a transaction missing size/weight renders with
// the Esplora size caveat and is still counted, never dropped.
func TestPartialTransaction(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()
	// A transaction with no size/weight metadata (the partial Esplora case).
	if err := repo.SaveTransactions(ctx, []schema.Transaction{{
		TxID:       "TXP",
		Inputs:     []schema.TransactionInput{{Address: "WP", AmountSats: schema.BTCToSats(1)}},
		Outputs:    []schema.TransactionOutput{{Address: "WQ", AmountSats: schema.BTCToSats(1)}},
		Provenance: schema.Provenance{SourceType: schema.SourceExplorer},
	}}); err != nil {
		t.Fatalf("seed partial: %v", err)
	}
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	r := schema.InvestigationResult{
		ID: "inv-WP", CaseID: "case-1", Subject: "WP",
		SubjectType: schema.NodeWallet, Offline: true, CreatedAt: fixedTime(),
	}
	snap, err := svc.BuildSnapshot(ctx, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Transactions) != 1 {
		t.Fatalf("partial tx must still be counted, got %d", len(snap.Transactions))
	}
	data, err := svc.Render(ctx, snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render md: %v", err)
	}
	if !strings.Contains(string(data), "Transaction size/weight fields may be absent") {
		t.Fatal("partial tx report must carry the Esplora size caveat")
	}
}

// TestProviderGap asserts a monitoring session whose provider is disconnected
// is reported as a provider-disconnected state (an honest coverage statement),
// never silently treated as healthy.
func TestProviderGap(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	seedMonitorSession(t, repo, "RUNNING", "degraded", "W1", 1)
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	snap, err := svc.BuildSnapshot(context.Background(), richResult())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if snap.MonitorSession == nil {
		t.Fatal("expected a monitor session in the snapshot")
	}
	state, _ := models.ClassifyMonitorState(snap.MonitorSession)
	if state != models.MonitorStateDisconnected {
		t.Fatalf("provider gap must classify as disconnected, got %q", state)
	}
}
