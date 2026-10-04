package commands

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/storage/sqlite"
)

// fixedClock returns a deterministic clock for render metadata. The clock only
// affects the non-hashed generated_at stamp, never the snapshot hash.
func fixedClock() func() time.Time {
	t := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time { return t }
}

// seedReportRepo opens a temp sqlite repo and seeds one wallet transaction so a
// snapshot has content. It returns the repo and the subject address.
func seedReportRepo(t *testing.T) (*sqlite.Repository, string) {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := context.Background()
	tx := schema.Transaction{
		TxID:       "TXREPORT1",
		Timestamp:  time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC),
		FeeBTC:     0.0001,
		Inputs:     []schema.TransactionInput{{Address: "WREPORT", AmountBTC: 2.0, Index: 0}},
		Outputs:    []schema.TransactionOutput{{Address: "WOTHER", AmountBTC: 1.9, Index: 0}},
		Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	return repo, "WREPORT"
}

// sampleResult builds a deterministic InvestigationResult the reporting service
// can snapshot. It mirrors what the orchestrator would produce for the seeded
// wallet, without needing the ONNX model pipeline.
func sampleResult(subject string) schema.InvestigationResult {
	return schema.InvestigationResult{
		ID:          "inv-" + subject,
		CaseID:      "case-report-test",
		Subject:     subject,
		SubjectType: schema.NodeWallet,
		Offline:     true,
		Risk: schema.RiskAssessment{
			Subject: subject, SubjectType: schema.NodeWallet,
			Score: 42, Confidence: 0.5,
		},
		RelevantTxs: 1,
	}
}

// TestReportGenerateJSONDeterministic proves the render path used by
// `report generate --format json` produces valid JSON whose snapshot hash is
// stable across two independent runs (acceptance criterion), with the service
// composing only the local repository (no network).
func TestReportGenerateJSONDeterministic(t *testing.T) {
	repo, subject := seedReportRepo(t)
	ctx := context.Background()
	result := sampleResult(subject)

	build := reporting.BuildInfo{GeneratedBy: "bctx test"}

	run := func() ([]byte, string) {
		svc := reporting.NewServiceWithClock(repo, build, fixedClock())
		snap, err := svc.BuildSnapshot(ctx, result)
		if err != nil {
			t.Fatalf("build snapshot: %v", err)
		}
		data, err := svc.Render(ctx, snap, schema.FormatJSON)
		if err != nil {
			t.Fatalf("render json: %v", err)
		}
		return data, reporting.SnapshotHash(snap)
	}

	d1, h1 := run()
	d2, h2 := run()

	if h1 != h2 {
		t.Fatalf("snapshot hash not stable: %s != %s", h1, h2)
	}
	if string(d1) != string(d2) {
		t.Fatal("json render not deterministic across runs")
	}
	var obj map[string]any
	if err := json.Unmarshal(d1, &obj); err != nil {
		t.Fatalf("render output is not valid JSON: %v", err)
	}
}

// TestReportExportVerifyRoundTrip exercises the ExportBundle + Verify path used
// by `report export` and `report verify` entirely offline.
func TestReportExportVerifyRoundTrip(t *testing.T) {
	repo, subject := seedReportRepo(t)
	ctx := context.Background()
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx test"}, fixedClock())

	snap, err := svc.BuildSnapshot(ctx, sampleResult(subject))
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	outDir := t.TempDir()
	formats := []schema.ReportFormat{schema.FormatJSON, schema.FormatMarkdown}
	if _, err := svc.ExportBundle(ctx, snap, formats, outDir); err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	res, err := svc.Verify(ctx, filepath.Join(outDir, "report"))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !res.OK {
		t.Fatalf("verify not OK on intact bundle: %+v", res.Files)
	}
}

func TestValidateFormat(t *testing.T) {
	for _, ok := range []string{"json", "md", "html", "pdf"} {
		if _, err := validateFormat(ok); err != nil {
			t.Errorf("validateFormat(%q) unexpected error: %v", ok, err)
		}
	}
	if _, err := validateFormat("csv"); err == nil {
		t.Error("validateFormat(csv) expected error, got nil")
	}
}

func TestParseFormats(t *testing.T) {
	cfg := configs.Default()

	got, err := parseFormats("", cfg)
	if err != nil {
		t.Fatalf("default formats: %v", err)
	}
	if len(got) != 1 || got[0] != schema.ReportFormat(cfg.Reports.DefaultFormat) {
		t.Fatalf("default should resolve to config default, got %v", got)
	}

	got, err = parseFormats("json, md ,json,html", cfg)
	if err != nil {
		t.Fatalf("csv formats: %v", err)
	}
	// Deduplicated, order preserved.
	want := []schema.ReportFormat{schema.FormatJSON, schema.FormatMarkdown, schema.FormatHTML}
	if len(got) != len(want) {
		t.Fatalf("parseFormats dedup mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseFormats order mismatch at %d: got %v want %v", i, got, want)
		}
	}

	if _, err := parseFormats("json,bogus", cfg); err == nil {
		t.Error("parseFormats with bogus token expected error")
	}
}

func TestPresentSections(t *testing.T) {
	snap := models.ReportSnapshot{
		SchemaVersion: reporting.ReportSchemaVersion,
		Result: schema.InvestigationResult{
			Subject:  "W1",
			Evidence: []schema.EvidenceItem{{ID: "ev1"}},
		},
		Transactions: []schema.Transaction{{TxID: "T1"}},
	}
	sections := presentSections(snap)

	// Always-present baseline sections.
	for _, must := range []string{"case_information", "risk_assessment", "limitations", "evidence", "transaction_summary"} {
		if !contains(sections, must) {
			t.Errorf("expected section %q present, got %v", must, sections)
		}
	}
	// Must be sorted (deterministic).
	for i := 1; i < len(sections); i++ {
		if sections[i-1] > sections[i] {
			t.Fatalf("sections not sorted: %v", sections)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
