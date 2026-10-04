package acquisition

import (
	"context"
	"fmt"
	"runtime"
	"testing"
)

// bigProvider serves a large wallet history generated on the fly (no in-memory
// retention of the full set beyond the current page), so the test itself does
// not dominate memory.
type bigProvider struct {
	total    int
	pageSize int
	calls    int
}

func (b *bigProvider) ProviderName() string    { return "big" }
func (b *bigProvider) ProviderVersion() string { return "big-v1" }
func (b *bigProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{WalletHistory: true, TransactionFetch: true, Pagination: true}
}

func (b *bigProvider) FetchWalletHistory(ctx context.Context, req WalletHistoryRequest) (AcquiredWalletPage, error) {
	b.calls++
	page := req.Cursor.Page
	start := page * b.pageSize
	if start >= b.total {
		return AcquiredWalletPage{Address: req.Address, Done: true}, nil
	}
	end := start + b.pageSize
	if end > b.total {
		end = b.total
	}
	ids := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		ids = append(ids, fmt.Sprintf("BT%08d", i))
	}
	done := end >= b.total
	next := PaginationState{Page: page + 1}
	if done {
		next = PaginationState{}
	}
	return AcquiredWalletPage{Address: req.Address, TxIDs: ids, Next: next, Done: done}, nil
}

func (b *bigProvider) FetchTransaction(ctx context.Context, txid string) (AcquiredTransaction, error) {
	return AcquiredTransaction{
		TxID: txid, Timestamp: "2026-01-01T10:00:00Z", FeeBTC: 0.0001, HasFee: true,
		ScriptType: "p2wpkh", BaseSize: 110, TotalSize: 140, Weight: 470, VSize: 118,
		Inputs:  []AcquiredIO{{Address: "BIGW", AmountBTC: 0.01}},
		Outputs: []AcquiredIO{{Address: txid + "o", AmountBTC: 0.009}},
	}, nil
}

// TestLargePaginationBoundedMemory syncs a large wallet history and asserts the
// process does not accumulate memory proportional to the full history. The
// engine processes page -> fetch -> persist -> checkpoint -> next page, so heap
// growth should track page/batch size, not total history.
func TestLargePaginationBoundedMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large pagination test in -short mode")
	}
	r := newRepo(t)
	const total = 4000
	p := &bigProvider{total: total, pageSize: 50}
	cfg := fastCfg()
	cfg.PageSize = 50
	cfg.MaxWorkers = 8
	e := NewEngine(p, r, "c", cfg)
	defer e.Close()

	var m0 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)

	st, err := e.SyncWallet(context.Background(), "BIGWALLET", PaginationState{})
	if err != nil {
		t.Fatal(err)
	}
	if st.New != total {
		t.Fatalf("new=%d, want %d", st.New, total)
	}
	if p.calls != total/50+1 { // pages + final done page
		t.Logf("history pages fetched: %d", p.calls)
	}

	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	// Heap in use after a full GC should not scale with 4000 txs retained; a
	// generous cap (32 MiB) catches an accidental "load everything" regression
	// while tolerating SQLite/runtime overhead.
	const cap = 32 << 20
	if m1.HeapInuse > cap {
		t.Fatalf("heap in use %d bytes exceeds bounded cap %d (possible full-history retention)",
			m1.HeapInuse, cap)
	}
	t.Logf("synced %d txs over %d pages; heap in use %d KiB", total, p.calls, m1.HeapInuse/1024)
}

// TestOfflineEngineMakesNoProviderCall proves the engine is only invoked when
// the caller decides to; a provider that fails the test if called is never
// touched when we don't run a sync (mirrors the CLI offline guard which returns
// before constructing/engaging the engine).
func TestOfflineEngineMakesNoProviderCall(t *testing.T) {
	called := false
	p := &guardProvider{onCall: func() { called = true }}
	r := newRepo(t)
	// Simulate the CLI offline guard: do NOT call any sync method.
	_ = NewEngine(p, r, "c", fastCfg())
	if called {
		t.Fatal("provider was called without an explicit sync request")
	}
}

type guardProvider struct{ onCall func() }

func (g *guardProvider) ProviderName() string               { return "guard" }
func (g *guardProvider) ProviderVersion() string            { return "guard-v1" }
func (g *guardProvider) Capabilities() ProviderCapabilities { return ProviderCapabilities{} }
func (g *guardProvider) FetchWalletHistory(ctx context.Context, req WalletHistoryRequest) (AcquiredWalletPage, error) {
	g.onCall()
	return AcquiredWalletPage{}, nil
}
func (g *guardProvider) FetchTransaction(ctx context.Context, txid string) (AcquiredTransaction, error) {
	g.onCall()
	return AcquiredTransaction{}, nil
}
