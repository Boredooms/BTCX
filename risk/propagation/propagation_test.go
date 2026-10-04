package propagation

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

func TestDistanceDecay(t *testing.T) {
	r, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ctx := context.Background()
	base := time.Now().UTC()
	txs := []schema.Transaction{
		{TxID: "TX1", Timestamp: base,
			Inputs:  []schema.TransactionInput{{Address: "SEED", AmountBTC: 10}},
			Outputs: []schema.TransactionOutput{{Address: "H1", AmountBTC: 9}}},
		{TxID: "TX2", Timestamp: base,
			Inputs:  []schema.TransactionInput{{Address: "H1", AmountBTC: 9}},
			Outputs: []schema.TransactionOutput{{Address: "H2", AmountBTC: 8}}},
	}
	if err := r.SaveTransactions(ctx, txs); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.NewBuilder(r).BuildAll(ctx); err != nil {
		t.Fatal(err)
	}

	e := New(r, DefaultConfig())
	steps, err := e.Propagate(ctx, Seed{NodeID: "SEED", Risk: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) == 0 {
		t.Fatal("expected propagation steps")
	}
	// Contribution must strictly decay with distance.
	byTarget := map[string]schema.RiskPropagationStep{}
	for _, s := range steps {
		byTarget[s.Target] = s
		if s.Contribution <= 0 || s.Contribution > 1 {
			t.Fatalf("contribution out of range: %+v", s)
		}
		if len(s.Path) != s.Distance+1 {
			t.Fatalf("path length %d != distance+1 %d", len(s.Path), s.Distance+1)
		}
	}
	// A closer node should receive at least as much as a farther one.
	if h1, ok := byTarget["H1"]; ok {
		if h2, ok2 := byTarget["H2"]; ok2 && h2.Contribution > h1.Contribution {
			t.Fatalf("decay violated: H2(%.3f) > H1(%.3f)", h2.Contribution, h1.Contribution)
		}
	}
}
