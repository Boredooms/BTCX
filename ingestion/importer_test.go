package ingestion

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bctx/bctx/blockchain/normalizer"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

func fixture(name string) string {
	return filepath.Join("..", "tests", "fixtures", name)
}

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestNormalizerSatsAndSize(t *testing.T) {
	if schema.BTCToSats(1.0) != 100_000_000 {
		t.Fatal("sats conversion wrong")
	}
	if normalizer.VSizeFromWeight(470) != 118 { // ceil(470/4)=118
		t.Fatalf("vsize = %d, want 118", normalizer.VSizeFromWeight(470))
	}
	if normalizer.WeightFromSizes(110, 140) != 470 {
		t.Fatalf("weight = %d, want 470", normalizer.WeightFromSizes(110, 140))
	}
	fr := normalizer.FeeRateSatVB(10_000_000, 118) // 0.1 BTC fee over 118 vB
	if fr < 84000 || fr > 85000 {
		t.Fatalf("feerate = %.2f sat/vB unexpected", fr)
	}
}

func TestFormatAutoDetect(t *testing.T) {
	cases := map[string]Format{
		"sized_case.csv": FormatCSV, "sized_case.json": FormatJSON,
		"sized_case.xml": FormatXML,
	}
	for file, want := range cases {
		got, err := DetectFormat(fixture(file))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if got != want {
			t.Fatalf("%s detected %s, want %s", file, got, want)
		}
	}
}

func importAndCheck(t *testing.T, file string, format Format) *sqlite.Repository {
	t.Helper()
	r := newRepo(t)
	ctx := context.Background()
	im := NewImporter(r, "testcase")
	st, err := im.Import(ctx, fixture(file), format, nil)
	if err != nil {
		t.Fatalf("import %s: %v", file, err)
	}
	if st.Transactions != 2 {
		t.Fatalf("%s: transactions=%d want 2", file, st.Transactions)
	}
	// TX1 should carry real size + feerate.
	tx, err := r.GetTransaction(ctx, "TX1")
	if err != nil || tx == nil {
		t.Fatalf("%s: TX1 not found: %v", file, err)
	}
	if tx.Size.VSize != 118 {
		t.Fatalf("%s: TX1 vsize=%d want 118", file, tx.Size.VSize)
	}
	if tx.FeeRateSatVB <= 0 {
		t.Fatalf("%s: TX1 feerate not computed", file)
	}
	if tx.ScriptType != "p2wpkh" {
		t.Fatalf("%s: TX1 script_type=%q", file, tx.ScriptType)
	}
	if tx.FeeSats != 10_000_000 {
		t.Fatalf("%s: TX1 fee_sats=%d want 10000000", file, tx.FeeSats)
	}
	return r
}

func TestImportCSV(t *testing.T)  { importAndCheck(t, "sized_case.csv", FormatCSV) }
func TestImportJSON(t *testing.T) { importAndCheck(t, "sized_case.json", FormatJSON) }
func TestImportXML(t *testing.T)  { importAndCheck(t, "sized_case.xml", FormatXML) }

// TestFormatEquivalence verifies CSV, JSON, XML produce an equivalent canonical
// database for TX1 (same vsize, fee_sats, script, in/out).
func TestFormatEquivalence(t *testing.T) {
	ctx := context.Background()
	get := func(file string, format Format) *schema.Transaction {
		r := newRepo(t)
		im := NewImporter(r, "c")
		if _, err := im.Import(ctx, fixture(file), format, nil); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		tx, _ := r.GetTransaction(ctx, "TX1")
		if tx == nil {
			t.Fatalf("%s: TX1 missing", file)
		}
		return tx
	}
	csvTx := get("sized_case.csv", FormatCSV)
	jsonTx := get("sized_case.json", FormatJSON)
	xmlTx := get("sized_case.xml", FormatXML)

	for _, pair := range [][2]*schema.Transaction{{csvTx, jsonTx}, {csvTx, xmlTx}} {
		a, b := pair[0], pair[1]
		if a.Size.VSize != b.Size.VSize || a.FeeSats != b.FeeSats ||
			a.ScriptType != b.ScriptType || len(a.Inputs) != len(b.Inputs) ||
			len(a.Outputs) != len(b.Outputs) {
			t.Fatalf("format mismatch: %+v vs %+v", a, b)
		}
	}
}

// TestIdempotentImport verifies re-importing yields 0 new, all duplicates.
func TestIdempotentImport(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	im := NewImporter(r, "c")
	st1, err := im.Import(ctx, fixture("sized_case.csv"), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	st2, err := im.Import(ctx, fixture("sized_case.csv"), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st1.Transactions == 0 {
		t.Fatal("first import added nothing")
	}
	if st2.Transactions != 0 {
		t.Fatalf("second import added %d (want 0, idempotent)", st2.Transactions)
	}
	if st2.Duplicates == 0 {
		t.Fatal("second import reported no duplicates")
	}
}

// TestImportBuildsGraphAndNetworkObs verifies graph edges + observation ingest.
func TestImportBuildsGraphAndNetworkObs(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	im := NewImporter(r, "c")
	st, err := im.Import(ctx, fixture("sized_case.csv"), FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.NetworkObs != 1 {
		t.Fatalf("network obs = %d, want 1", st.NetworkObs)
	}
	// Graph should have an observed_with edge from the IP to TX1.
	edges, _ := r.EdgesFrom(ctx, "10.0.0.1")
	found := false
	for _, e := range edges {
		if e.Type == schema.EdgeObservedWith && e.To == "TX1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected observed_with edge IP->TX1")
	}
	// A->B sent_to edge exists.
	gm, _ := graph.NewService(r).WalletMetrics(ctx, "A", 6)
	if gm.FanOut < 1 {
		t.Fatalf("A fan_out = %d, want >=1", gm.FanOut)
	}
}

// TestPartialTransaction verifies a tx missing outputs is kept as partial.
func TestPartialTransaction(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	// Build a tiny CSV with an input-only tx inline via a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.csv")
	content := "txid,timestamp,input_address,input_amount,output_address,output_amount,fee\n" +
		"TXP,2026-01-01T10:00:00Z,A,5.0,,,0\n"
	if err := writeFile(path, content); err != nil {
		t.Fatal(err)
	}
	im := NewImporter(r, "c")
	st, err := im.Import(ctx, path, FormatCSV, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.Partial != 1 {
		t.Fatalf("partial = %d, want 1", st.Partial)
	}
	tx, _ := r.GetTransaction(ctx, "TXP")
	if tx == nil || tx.Completeness != schema.CompletePartial {
		t.Fatalf("expected partial tx, got %+v", tx)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
