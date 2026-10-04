package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
)

// TestNoSecretLeakageAcrossAllFormats builds a snapshot whose datasets and
// provenance carry provider metadata, renders every format, and asserts no
// credential-looking substrings appear and that only the redact.go
// allow-listed Dataset fields are present in the rendered dataset provenance.
func TestNoSecretLeakageAcrossAllFormats(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	ctx := context.Background()

	// A dataset whose allow-listed fields are populated. The allow-list is the
	// security boundary: even if schema.Dataset ever grows a secret-bearing
	// field, RedactDataset would not copy it, so it can never reach a renderer.
	if err := repo.SaveDataset(ctx, schema.Dataset{
		ID:              "ds1",
		CaseID:          "case-1",
		SourceFile:      "mempool-export.json",
		SHA256:          "abc123",
		SchemaVersion:   "ds-v1",
		ToolVersion:     "tool-v2",
		Format:          "esplora-json",
		ParserVersion:   "parser-v3",
		ImporterVersion: "importer-v4",
		ImportedAt:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("save dataset: %v", err)
	}

	// A sync checkpoint + monitor session with provider metadata.
	if err := repo.SaveSyncCheckpoint(ctx, sdk.SyncCheckpoint{
		SyncID: "sync1", Provider: "esplora", ProviderVersion: "1.0",
		TargetType: "wallet", Target: "W1", Status: "complete",
		Discovered: 2, Acquired: 2, Persisted: 2,
		StartedAt: "2024-01-01T00:00:00Z", UpdatedAt: "2024-01-01T00:00:10Z",
	}); err != nil {
		t.Fatalf("save sync: %v", err)
	}
	seedMonitorSession(t, repo, "RUNNING", "healthy", "W1", 2)

	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	snap, err := svc.BuildSnapshot(ctx, richResult())
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Render every format and assert no secret substrings.
	for _, f := range allFormats() {
		data, err := svc.Render(ctx, snap, f)
		if err != nil {
			t.Fatalf("render %s: %v", f, err)
		}
		assertNoSecrets(t, string(f), data)
	}

	// Assert the rendered dataset provenance exposes ONLY the allow-listed
	// fields — no extra key that could carry an operator-local path or secret.
	jsonData, err := svc.Render(ctx, snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(jsonData, &m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	dp, _ := m["dataset_provenance"].(map[string]any)
	datasets, _ := dp["datasets"].([]any)
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}
	allowed := map[string]bool{
		"id": true, "source_file": true, "sha256": true,
		"schema_version": true, "tool_version": true, "format": true,
		"parser_version": true, "importer_version": true, "imported_at": true,
	}
	ds0, _ := datasets[0].(map[string]any)
	for k := range ds0 {
		if !allowed[k] {
			t.Fatalf("rendered dataset exposes non-allow-listed field %q", k)
		}
	}
	// The redacted dataset type itself must only have the allow-listed JSON keys.
	rd := models.RedactDataset(schema.Dataset{ID: "x"})
	rb, _ := json.Marshal(rd)
	var rdm map[string]any
	_ = json.Unmarshal(rb, &rdm)
	for k := range rdm {
		if !allowed[k] {
			t.Fatalf("RedactedDataset JSON exposes non-allow-listed field %q", k)
		}
	}
}

// TestRedactedDatasetDropsNonAllowListed is a direct guard on the redaction
// boundary: counts and other schema.Dataset fields outside the allow-list must
// never appear in the redacted projection's serialization.
func TestRedactedDatasetDropsNonAllowListed(t *testing.T) {
	full := schema.Dataset{
		ID:            "ds",
		CaseID:        "case-should-not-appear",
		SourceFile:    "f.json",
		RecordsRead:   999,
		RecordsValid:  999,
		RecordsReject: 7,
		Transactions:  123,
		Wallets:       45,
		NetworkRecs:   6,
	}
	b, err := json.Marshal(models.RedactDataset(full))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"case_id", "records_read", "records_valid", "transactions", "wallets", "network_records"} {
		if bytes.Contains(b, []byte(`"`+forbidden+`"`)) {
			t.Fatalf("redacted dataset leaked non-allow-listed field %q: %s", forbidden, b)
		}
	}
}
