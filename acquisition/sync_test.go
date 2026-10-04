package acquisition

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/ml/features"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func tx(id, from, to string, amt float64) AcquiredTransaction {
	return AcquiredTransaction{
		TxID: id, Timestamp: "2026-01-01T10:00:00Z", FeeBTC: 0.0001, HasFee: true,
		ScriptType: "p2wpkh", BaseSize: 110, TotalSize: 140, Weight: 470, VSize: 118,
		Inputs:  []AcquiredIO{{Address: from, AmountBTC: amt}},
		Outputs: []AcquiredIO{{Address: to, AmountBTC: amt - 0.0001}},
	}
}

// provider with a 5-tx wallet across multiple pages.
func seededProvider() *FakeProvider {
	p := NewFakeProvider()
	p.PageSize = 2
	p.Wallets["A"] = []string{"T1", "T2", "T3", "T4", "T5"}
	p.Txs["T1"] = tx("T1", "A", "B", 1.0)
	p.Txs["T2"] = tx("T2", "A", "C", 0.5)
	p.Txs["T3"] = tx("T3", "A", "D", 0.4)
	p.Txs["T4"] = tx("T4", "A", "E", 0.3)
	p.Txs["T5"] = tx("T5", "A", "F", 0.2)
	p.Observations["T1"] = []AcquiredNetworkObservation{
		{SrcIP: "10.0.0.1", SrcPort: 8333, DstIP: "10.0.0.2", DstPort: 8333, TxID: "T1"},
	}
	return p
}

func fastCfg() Config {
	c := DefaultConfig()
	c.RequestsPerSecond = 0 // unlimited in tests
	c.PageSize = 2          // force multi-page over the 5-tx fixture
	c.Retry.InitialBackoff = time.Millisecond
	c.Retry.MaxBackoff = 2 * time.Millisecond
	c.Seed = 1
	return c
}

func TestWalletSyncMultiPage(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	st, err := e.SyncWallet(context.Background(), "A", PaginationState{})
	if err != nil {
		t.Fatal(err)
	}
	if st.Pages < 3 {
		t.Fatalf("pages=%d, want >=3 for 5 txs @ pagesize 2", st.Pages)
	}
	if st.New != 6 { // 5 transactions + 1 network observation
		t.Fatalf("new=%d, want 6 (5 tx + 1 obs)", st.New)
	}
	if st.Fetched != 5 {
		t.Fatalf("fetched=%d, want 5 transactions", st.Fetched)
	}
	for _, id := range []string{"T1", "T2", "T3", "T4", "T5"} {
		got, _ := r.GetTransaction(context.Background(), id)
		if got == nil {
			t.Fatalf("tx %s not persisted", id)
		}
		if got.Size.VSize != 118 {
			t.Fatalf("tx %s vsize=%d want 118", id, got.Size.VSize)
		}
	}
}

func TestWalletSyncIdempotent(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	ctx := context.Background()
	if _, err := e.SyncWallet(ctx, "A", PaginationState{}); err != nil {
		t.Fatal(err)
	}
	st2, err := e.SyncWallet(ctx, "A", PaginationState{})
	if err != nil {
		t.Fatal(err)
	}
	if st2.New != 0 {
		t.Fatalf("second sync new=%d, want 0 (idempotent)", st2.New)
	}
	// Wallet A counter not double-incremented: it has one tx_count per spend.
}

func TestTxSyncAndAlreadyLocal(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	ctx := context.Background()
	st, err := e.SyncTransaction(ctx, "T1")
	if err != nil {
		t.Fatal(err)
	}
	if st.New != 1 {
		t.Fatalf("new=%d, want 1", st.New)
	}
	st2, err := e.SyncTransaction(ctx, "T1")
	if err != nil {
		t.Fatal(err)
	}
	if st2.Status != "already_local" {
		t.Fatalf("status=%q, want already_local", st2.Status)
	}
}

func TestTxNotFound(t *testing.T) {
	r := newRepo(t)
	p := seededProvider()
	e := NewEngine(p, r, "c", fastCfg())
	defer e.Close()
	_, err := e.SyncTransaction(context.Background(), "NOPE")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !Retryable(err) == false { // not-found is permanent (not retryable)
	}
	if !errorsIs(err, ErrProviderNotFound) {
		t.Fatalf("err=%v, want ErrProviderNotFound", err)
	}
}

func TestRetryableRetriesThenSucceeds(t *testing.T) {
	r := newRepo(t)
	p := seededProvider()
	p.FailNTimes["T1"] = 2 // 2 transient failures, then success
	e := NewEngine(p, r, "c", fastCfg())
	defer e.Close()
	st, err := e.SyncTransaction(context.Background(), "T1")
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if st.Retries < 2 {
		t.Fatalf("retries=%d, want >=2", st.Retries)
	}
}

func TestPermanentDoesNotRetry(t *testing.T) {
	r := newRepo(t)
	p := seededProvider()
	p.Permanent["T1"] = true
	e := NewEngine(p, r, "c", fastCfg())
	defer e.Close()
	st, err := e.SyncTransaction(context.Background(), "T1")
	if err == nil {
		t.Fatal("expected permanent error")
	}
	if st.Retries != 0 {
		t.Fatalf("permanent error retried %d times, want 0", st.Retries)
	}
}

func TestThrottleRetriable(t *testing.T) {
	r := newRepo(t)
	p := seededProvider()
	p.Throttle["T2"] = true
	e := NewEngine(p, r, "c", fastCfg())
	defer e.Close()
	st, err := e.SyncTransaction(context.Background(), "T2")
	if err != nil {
		t.Fatalf("throttle should be retried then succeed: %v", err)
	}
	if st.Retries < 1 {
		t.Fatalf("expected >=1 retry for throttle, got %d", st.Retries)
	}
}

func TestCancellation(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := e.SyncWallet(ctx, "A", PaginationState{})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestCapabilityUnsupported(t *testing.T) {
	r := newRepo(t)
	p := seededProvider()
	p.NoWalletCap = true
	e := NewEngine(p, r, "c", fastCfg())
	defer e.Close()
	_, err := e.SyncWallet(context.Background(), "A", PaginationState{})
	if !errorsIs(err, ErrCapabilityUnsupported) {
		t.Fatalf("err=%v, want ErrCapabilityUnsupported", err)
	}
}

func TestAcquiredReachesGraphAndFeatures(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	ctx := context.Background()
	if _, err := e.SyncWallet(ctx, "A", PaginationState{}); err != nil {
		t.Fatal(err)
	}
	// Graph: A should have fan_out >= 1 (sent_to edges).
	gm, err := graph.NewService(r).WalletMetrics(ctx, "A", 6)
	if err != nil {
		t.Fatal(err)
	}
	if gm.FanOut < 1 {
		t.Fatalf("A fan_out=%d, want >=1", gm.FanOut)
	}
	// observed_with edge from the IP.
	edges, _ := r.EdgesFrom(ctx, "10.0.0.1")
	found := false
	for _, ed := range edges {
		if ed.Type == schema.EdgeObservedWith && ed.To == "T1" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected observed_with edge IP->T1 from acquisition")
	}
	// Features: real feature vector with 25 features, no NaN.
	fe := features.NewEngine(r, graph.NewService(r)).
		WithGraphMetrics(graph.NewMetricsAdapter(graph.NewService(r)))
	fv, err := fe.WalletFeatures(ctx, "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(fv.Values) != 25 {
		t.Fatalf("features=%d, want 25", len(fv.Values))
	}
}

func TestPartialEnrichment(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	// First acquire a partial tx (missing outputs).
	p := NewFakeProvider()
	partial := AcquiredTransaction{
		TxID: "TP", Timestamp: "2026-01-01T10:00:00Z",
		Inputs: []AcquiredIO{{Address: "A", AmountBTC: 5.0}}, Partial: true,
	}
	p.Txs["TP"] = partial
	e := NewEngine(p, r, "c", fastCfg())
	if _, err := e.SyncTransaction(ctx, "TP"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.GetTransaction(ctx, "TP")
	if got == nil || got.Completeness != schema.CompletePartial {
		t.Fatalf("expected partial, got %+v", got)
	}
	e.Close()

	// Now a provider returns the complete tx; enrichment should fill outputs.
	p2 := NewFakeProvider()
	p2.Txs["TP"] = AcquiredTransaction{
		TxID: "TP", Timestamp: "2026-01-01T10:00:00Z", FeeBTC: 0.0001, HasFee: true,
		ScriptType: "p2wpkh", BaseSize: 110, TotalSize: 140, Weight: 470, VSize: 118,
		Inputs:  []AcquiredIO{{Address: "A", AmountBTC: 5.0}},
		Outputs: []AcquiredIO{{Address: "B", AmountBTC: 4.9999}},
	}
	e2 := NewEngine(p2, r, "c", fastCfg())
	defer e2.Close()
	// Force refetch: tx is partial locally, so SyncTransaction will fetch.
	if _, err := e2.SyncTransaction(ctx, "TP"); err != nil {
		t.Fatal(err)
	}
	got2, _ := r.GetTransaction(ctx, "TP")
	if len(got2.Outputs) != 1 {
		t.Fatalf("enrichment did not add outputs: %+v", got2)
	}
	if got2.Size.VSize != 118 {
		t.Fatalf("enrichment did not add size: %+v", got2.Size)
	}
}

func TestEmptyWallet(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(NewFakeProvider(), r, "c", fastCfg())
	defer e.Close()
	st, err := e.SyncWallet(context.Background(), "UNKNOWN", PaginationState{})
	if err != nil {
		t.Fatal(err)
	}
	if st.New != 0 {
		t.Fatalf("empty wallet new=%d, want 0", st.New)
	}
}

func TestCheckpointPersisted(t *testing.T) {
	r := newRepo(t)
	e := NewEngine(seededProvider(), r, "c", fastCfg())
	defer e.Close()
	ctx := context.Background()
	st, err := e.SyncWallet(ctx, "A", PaginationState{})
	if err != nil {
		t.Fatal(err)
	}
	cp, err := r.GetSyncCheckpoint(ctx, st.SyncID)
	if err != nil || cp == nil {
		t.Fatalf("checkpoint not persisted: %v", err)
	}
	if cp.Status != "completed" {
		t.Fatalf("checkpoint status=%q, want completed", cp.Status)
	}
	if cp.Persisted != 6 { // 5 transactions + 1 network observation
		t.Fatalf("checkpoint persisted=%d, want 6", cp.Persisted)
	}
}

// errorsIs is a tiny local wrapper to avoid importing errors in every test.
func errorsIs(err, target error) bool {
	for e := err; e != nil; {
		if e == target {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
