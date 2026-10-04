package graph

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

// newRepo returns a fresh SQLite repo for a test.
func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// chainTx builds a tx moving value from->to.
func chainTx(id, from, to string, amt float64, ts time.Time) schema.Transaction {
	return schema.Transaction{
		TxID: id, Timestamp: ts,
		Inputs:  []schema.TransactionInput{{Address: from, AmountBTC: amt, Index: 0}},
		Outputs: []schema.TransactionOutput{{Address: to, AmountBTC: amt - 0.01, Index: 0}},
	}
}

// seedChain persists A->B->C->D and returns the repo.
func seedChain(t *testing.T) *sqlite.Repository {
	r := newRepo(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	txs := []schema.Transaction{
		chainTx("TX1", "A", "B", 10, base),
		chainTx("TX2", "B", "C", 5, base.Add(time.Minute)),
		chainTx("TX3", "C", "D", 2, base.Add(2*time.Minute)),
	}
	if err := r.SaveTransactions(ctx, txs); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := NewBuilder(r).BuildAll(ctx); err != nil {
		t.Fatalf("build: %v", err)
	}
	return r
}

func TestBuildAndDedup(t *testing.T) {
	r := seedChain(t)
	ctx := context.Background()
	b := NewBuilder(r)

	st1, _ := b.GraphStats(ctx)
	// Rebuild must not change edge count (idempotent).
	if _, err := b.BuildAll(ctx); err != nil {
		t.Fatal(err)
	}
	st2, _ := b.GraphStats(ctx)
	if st1.Edges != st2.Edges {
		t.Fatalf("non-idempotent build: %d -> %d edges", st1.Edges, st2.Edges)
	}
	if st2.Edges == 0 {
		t.Fatal("expected edges")
	}
}

func TestNeighbors(t *testing.T) {
	r := seedChain(t)
	svc := NewService(r)
	ns, err := svc.Neighbors(context.Background(), "B")
	if err != nil {
		t.Fatal(err)
	}
	// B connects to A (sent_to in), C (sent_to out), TX1, TX2.
	ids := map[string]bool{}
	for _, n := range ns {
		ids[n.ID] = true
	}
	for _, want := range []string{"A", "C", "TX1", "TX2"} {
		if !ids[want] {
			t.Fatalf("expected neighbor %s, got %v", want, ids)
		}
	}
}

func TestPath(t *testing.T) {
	r := seedChain(t)
	svc := NewService(r)
	p, err := svc.Path(context.Background(), "A", "D")
	if err != nil {
		t.Fatal(err)
	}
	if len(p) == 0 {
		t.Fatal("expected a path A..D")
	}
	if p[0] != "A" || p[len(p)-1] != "D" {
		t.Fatalf("bad path endpoints: %v", p)
	}
}

func TestWalletMetricsChain(t *testing.T) {
	r := seedChain(t)
	svc := NewService(r)
	m, err := svc.WalletMetrics(context.Background(), "A", 6)
	if err != nil {
		t.Fatal(err)
	}
	// A sends to B (fan_out >= 1), and the dominant chain A->B->C->D.
	if m.FanOut < 1 {
		t.Fatalf("expected fan_out>=1, got %d", m.FanOut)
	}
	if m.ChainLength < 2 {
		t.Fatalf("expected chain_length>=2, got %d", m.ChainLength)
	}
	if m.GraphDepth < 1 {
		t.Fatalf("expected graph_depth>=1, got %d", m.GraphDepth)
	}
	if m.ValueDecay <= 0 || m.ValueDecay > 1 {
		t.Fatalf("value_decay out of range: %v", m.ValueDecay)
	}
}

func TestBranching(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	base := time.Now().UTC()
	// B -> C, B -> D, C -> E, D -> E (diamond).
	txs := []schema.Transaction{
		chainTx("TXbc", "B", "C", 4, base),
		chainTx("TXbd", "B", "D", 4, base),
		chainTx("TXce", "C", "E", 2, base),
		chainTx("TXde", "D", "E", 2, base),
	}
	if err := r.SaveTransactions(ctx, txs); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBuilder(r).BuildAll(ctx); err != nil {
		t.Fatal(err)
	}
	svc := NewService(r)
	mB, _ := svc.WalletMetrics(ctx, "B", 6)
	if mB.FanOut < 2 {
		t.Fatalf("B fan_out expected >=2, got %d", mB.FanOut)
	}
	mE, _ := svc.WalletMetrics(ctx, "E", 6)
	if mE.FanIn < 2 {
		t.Fatalf("E fan_in expected >=2, got %d", mE.FanIn)
	}
}

func TestRebuildStable(t *testing.T) {
	r := seedChain(t)
	ctx := context.Background()
	b := NewBuilder(r)
	before, _ := b.GraphStats(ctx)
	if _, err := b.Rebuild(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := b.GraphStats(ctx)
	if before.Edges != after.Edges {
		t.Fatalf("rebuild changed edge count: %d -> %d", before.Edges, after.Edges)
	}
	// Canonical records preserved.
	tx, _ := r.GetTransaction(ctx, "TX1")
	if tx == nil {
		t.Fatal("rebuild destroyed canonical transaction")
	}
}

func TestConsistencyEdgesReferenceNodes(t *testing.T) {
	r := seedChain(t)
	ctx := context.Background()
	edges, _ := r.AllEdges(ctx, 0)
	if len(edges) == 0 {
		t.Fatal("no edges")
	}
	for _, e := range edges {
		if e.From == "" || e.To == "" || e.Type == "" {
			t.Fatalf("malformed edge: %+v", e)
		}
		if e.From == e.To {
			t.Fatalf("unexpected self-edge: %+v", e)
		}
	}
}
