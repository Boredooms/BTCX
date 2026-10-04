package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/storage/sqlite"
)

// decodeView renders JSON and decodes it into a generic map for section checks.
func decodeView(t *testing.T, svc *reporting.Service, snap models.ReportSnapshot) map[string]any {
	t.Helper()
	data, err := svc.Render(context.Background(), snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

func section(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	s, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("section %q missing or not an object", key)
	}
	return s
}

// TestModelValidation asserts the snapshot carries the mandatory identity and
// provenance fields a report-schema-v1 snapshot must always have.
func TestModelValidation(t *testing.T) {
	_, snap := newFixedService(t)

	if snap.SchemaVersion != reporting.ReportSchemaVersion {
		t.Fatalf("schema version = %q", snap.SchemaVersion)
	}
	if snap.GeneratorVersion != reporting.GeneratorVersion {
		t.Fatalf("generator version = %q", snap.GeneratorVersion)
	}
	if snap.Result.CaseID == "" || snap.Result.Subject == "" {
		t.Fatal("snapshot result must carry case id + subject")
	}
	if snap.FeatureSchema != schema.FeatureSchemaVersion {
		t.Fatalf("feature schema = %q", snap.FeatureSchema)
	}
	if snap.FeatureSchemaSHA != schema.FeatureSchemaSHA256 {
		t.Fatal("feature schema sha mismatch")
	}
	// Traceability slices must be non-nil for a stable hash.
	if snap.EvidenceIDs == nil || snap.TxIDs == nil || snap.AlertIDs == nil {
		t.Fatal("traceability id slices must be non-nil")
	}
	if reporting.ReportID(snap) == "" {
		t.Fatal("report id must be derivable")
	}
}

// TestEvidenceProvenanceReferentialIntegrity asserts every evidence item's
// source records are present (no orphaned refs) and every evidence ID appears
// in the snapshot's EvidenceIDs trace list.
func TestEvidenceProvenanceReferentialIntegrity(t *testing.T) {
	_, snap := newFixedService(t)

	traced := map[string]bool{}
	for _, id := range snap.EvidenceIDs {
		traced[id] = true
	}
	for _, e := range snap.Result.Evidence {
		if !traced[e.ID] {
			t.Fatalf("evidence %q not present in snapshot EvidenceIDs", e.ID)
		}
		for _, rec := range e.SourceRecords {
			if strings.TrimSpace(rec) == "" {
				t.Fatalf("evidence %q has an empty source record (orphaned ref)", e.ID)
			}
		}
	}

	// In the rendered Evidence section every item must still carry its id.
	svc, snap2 := newFixedService(t)
	m := decodeView(t, svc, snap2)
	ev := section(t, m, "evidence")
	items, _ := ev["items"].([]any)
	if len(items) != len(snap.Result.Evidence) {
		t.Fatalf("evidence section has %d items, snapshot has %d", len(items), len(snap.Result.Evidence))
	}
	for _, it := range items {
		obj, _ := it.(map[string]any)
		if obj["id"] == "" || obj["id"] == nil {
			t.Fatal("rendered evidence item missing id")
		}
	}
}

// TestBoundedGraphHonorsBound asserts the graph summary reports exactly the
// subgraph bound captured in the result (node/edge/depth), never more.
func TestBoundedGraphHonorsBound(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)

	r := richResult()
	r.Subgraph = &schema.Subgraph{
		Center: "W1",
		Depth:  2,
		Nodes: []schema.GraphNode{
			{ID: "W1", Type: schema.NodeWallet},
			{ID: "W2", Type: schema.NodeWallet},
			{ID: "W3", Type: schema.NodeWallet},
		},
		Edges: []schema.GraphEdge{
			{ID: "e1", Type: schema.EdgeType("flow")},
			{ID: "e2", Type: schema.EdgeType("flow")},
		},
	}
	snap, err := svc.BuildSnapshot(context.Background(), r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	m := decodeView(t, svc, snap)
	g := section(t, m, "graph_summary")
	if g["available"] != true {
		t.Fatal("graph summary should be available")
	}
	if g["node_count"].(float64) != 3 || g["edge_count"].(float64) != 2 || g["depth"].(float64) != 2 {
		t.Fatalf("graph summary did not honor the bound: %+v", g)
	}
}

// TestTimelineDeterministic asserts two timeline assemblies over the same
// snapshot are identical and totally ordered.
func TestTimelineDeterministic(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	a, err := svc.Timeline(ctx, snap)
	if err != nil {
		t.Fatalf("timeline a: %v", err)
	}
	b, err := svc.Timeline(ctx, snap)
	if err != nil {
		t.Fatalf("timeline b: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("timeline length differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("timeline event %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
	for i := 1; i < len(a); i++ {
		if a[i-1].Timestamp > a[i].Timestamp {
			t.Fatalf("timeline not ordered: %+v", a)
		}
	}
}

// TestRiskSectionDetail asserts the risk section carries per-signal weights,
// score and confidence, deterministically sorted by signal name.
func TestRiskSectionDetail(t *testing.T) {
	svc, snap := newFixedService(t)
	m := decodeView(t, svc, snap)
	risk := section(t, m, "risk_assessment")
	if risk["score"].(float64) != 42 {
		t.Fatalf("risk score = %v", risk["score"])
	}
	sigs, _ := risk["signals"].([]any)
	if len(sigs) != 2 {
		t.Fatalf("expected 2 risk signals, got %d", len(sigs))
	}
	// Signals sorted by name: fan_out before mixing_like.
	first, _ := sigs[0].(map[string]any)
	if first["name"] != "fan_out" {
		t.Fatalf("risk signals not sorted by name: %v", first["name"])
	}
	for _, s := range sigs {
		o, _ := s.(map[string]any)
		if _, ok := o["weight"]; !ok {
			t.Fatal("risk signal missing weight")
		}
	}
}

// TestRiskDeltaSection asserts a monitoring risk-delta surfaces in the Risk
// Deltas section with changed signals and before/after/delta scores.
func TestRiskDeltaSection(t *testing.T) {
	repo := newRepo(t)
	seedWallet(t, repo)
	seedMonitorSession(t, repo, "RUNNING", "healthy", "W1", 1)
	ctx := context.Background()
	if err := repo.SaveRiskDelta(ctx, sdk.RiskDeltaRow{
		ID: "rd1", SessionID: "sess-W1", Subject: "W1",
		PreviousScore: 10, CurrentScore: 42, Delta: 32,
		ChangedSignals: []string{"fan_out"}, NewEvidenceIDs: []string{"ev1"},
		Timestamp: "2024-01-01T00:00:05Z",
	}); err != nil {
		t.Fatalf("save delta: %v", err)
	}
	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	snap, err := svc.BuildSnapshot(ctx, richResult())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.RiskDeltas) != 1 {
		t.Fatalf("expected 1 risk delta in snapshot, got %d", len(snap.RiskDeltas))
	}
	m := decodeView(t, svc, snap)
	rd := section(t, m, "risk_deltas")
	deltas, _ := rd["deltas"].([]any)
	if len(deltas) != 1 {
		t.Fatalf("expected 1 rendered delta, got %d", len(deltas))
	}
	d0, _ := deltas[0].(map[string]any)
	if d0["previous_score"].(float64) != 10 || d0["current_score"].(float64) != 42 || d0["delta"].(float64) != 32 {
		t.Fatalf("risk delta scores wrong: %+v", d0)
	}
}

// TestMLSectionProvenance asserts the ML section lists each prediction with its
// model version and feature-schema version (the model manifest IDs + feature
// runtime version a forensic report must trace).
func TestMLSectionProvenance(t *testing.T) {
	svc, snap := newFixedService(t)
	m := decodeView(t, svc, snap)
	ml := section(t, m, "ml_signals")
	preds, _ := ml["predictions"].([]any)
	if len(preds) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(preds))
	}
	// Sorted by model: anomaly before cluster.
	p0, _ := preds[0].(map[string]any)
	if p0["model"] != "anomaly" || p0["model_version"] != "v1" {
		t.Fatalf("ml prediction[0] wrong: %+v", p0)
	}
	if p0["feature_schema"] != schema.FeatureSchemaVersion {
		t.Fatalf("ml prediction missing feature schema version: %v", p0["feature_schema"])
	}
	// Generation metadata carries the feature schema + model manifest ids.
	gen := section(t, m, "generation_metadata")
	if gen["feature_schema"] != schema.FeatureSchemaVersion {
		t.Fatalf("generation metadata feature schema wrong: %v", gen["feature_schema"])
	}
	mv, _ := gen["model_versions"].(map[string]any)
	if mv["anomaly"] != "v1" || mv["cluster"] != "v2" {
		t.Fatalf("generation metadata model versions wrong: %+v", mv)
	}
}

// TestLargeCaseBoundedGraph asserts that even with many transactions the graph
// summary reflects only the bounded subgraph captured in the result (never the
// full transaction set), i.e. the stated bound is honored and memory is not
// unbounded by the renderer.
func TestLargeCaseBoundedGraph(t *testing.T) {
	repo := newRepo(t)
	ctx := context.Background()

	// Seed many transactions for the subject.
	const n = 500
	txs := make([]schema.Transaction, 0, n)
	for i := 0; i < n; i++ {
		id := "TX" + pad4(i)
		txs = append(txs, schema.Transaction{
			TxID:       id,
			Inputs:     []schema.TransactionInput{{Address: "WBIG", AmountSats: schema.BTCToSats(1)}},
			Outputs:    []schema.TransactionOutput{{Address: "WOUT", AmountSats: schema.BTCToSats(1)}},
			Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
		})
	}
	if err := repo.SaveTransactions(ctx, txs); err != nil {
		t.Fatalf("seed many: %v", err)
	}

	svc := reporting.NewServiceWithClock(repo, reporting.BuildInfo{GeneratedBy: "bctx-test"}, fixedTime)
	r := schema.InvestigationResult{
		ID: "inv-WBIG", CaseID: "case-1", Subject: "WBIG",
		SubjectType: schema.NodeWallet, Offline: true, CreatedAt: fixedTime(),
		// Bounded subgraph: only a handful of nodes/edges regardless of tx count.
		Subgraph: &schema.Subgraph{
			Center: "WBIG", Depth: 1,
			Nodes: []schema.GraphNode{{ID: "WBIG", Type: schema.NodeWallet}, {ID: "WOUT", Type: schema.NodeWallet}},
			Edges: []schema.GraphEdge{{ID: "e1", Type: schema.EdgeType("flow")}},
		},
	}
	snap, err := svc.BuildSnapshot(ctx, r)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(snap.Transactions) != n {
		t.Fatalf("expected all %d txs captured, got %d", n, len(snap.Transactions))
	}
	m := decodeView(t, svc, snap)
	g := section(t, m, "graph_summary")
	// The graph summary honors the bounded subgraph, not the 500-tx fan.
	if g["node_count"].(float64) != 2 || g["edge_count"].(float64) != 1 {
		t.Fatalf("graph summary not bounded to the subgraph: %+v", g)
	}
}

// pad4 renders i as a 4-digit zero-padded string for lexically stable tx ids.
func pad4(i int) string {
	s := []byte{'0', '0', '0', '0'}
	n := i
	for p := 3; p >= 0 && n > 0; p-- {
		s[p] = byte('0' + n%10)
		n /= 10
	}
	return string(s)
}

// seedMonitorSession inserts a monitor session for subject with the given
// status/health and events-seen count. SessionID is deterministic as
// "sess-<subject>".
func seedMonitorSession(t *testing.T, repo *sqlite.Repository, status, health, subject string, events int) sdk.MonitorSessionRow {
	t.Helper()
	row := sdk.MonitorSessionRow{
		SessionID:    "sess-" + subject,
		CaseID:       "case-1",
		Target:       subject,
		TargetType:   "wallet",
		Provider:     "esplora",
		Mode:         "poll",
		Status:       status,
		Health:       health,
		StartedAt:    "2024-01-01T00:00:00Z",
		UpdatedAt:    "2024-01-01T00:00:10Z",
		LastPollOKAt: "2024-01-01T00:00:09Z",
		LastEventAt:  "2024-01-01T00:00:08Z",
		EventsSeen:   events,
	}
	if err := repo.SaveMonitorSession(context.Background(), row); err != nil {
		t.Fatalf("save monitor session: %v", err)
	}
	return row
}
