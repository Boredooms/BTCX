package explorer

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/pkg/schema"
)

// Live Esplora integration tests. These open real network connections and are
// therefore opt-in: they run only when BCTX_LIVE=1. They prove the real
// provider deserializes on-chain data and maps satoshi values correctly. If the
// network is unreachable the test skips (not fails) so offline CI stays green.
func liveProvider(t *testing.T) *Provider {
	t.Helper()
	if os.Getenv("BCTX_LIVE") != "1" {
		t.Skip("live test: set BCTX_LIVE=1 to run (opens network connections)")
	}
	p := New(WithTimeout(20 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := p.TipHeight(ctx); err != nil {
		t.Skipf("live test: network unreachable (%v)", err)
	}
	return p
}

// Genesis coinbase: a single 50 BTC output, confirmed at height 0. Its satoshi
// value is an exact, immutable on-chain fact — ideal for serialization proof.
const genesisCoinbaseTxID = "4a5e1e4baab89f3a32518a88c31bc87f618f76673e2cc77ab2127b7afdeda33b"

func TestLiveFetchGenesisSerialization(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	at, err := p.FetchTransaction(ctx, genesisCoinbaseTxID)
	if err != nil {
		t.Fatalf("fetch genesis: %v", err)
	}
	if at.TxID != genesisCoinbaseTxID {
		t.Fatalf("txid=%q", at.TxID)
	}
	// Coinbase has no addressed inputs; exactly one 50 BTC output.
	if len(at.Inputs) != 0 {
		t.Fatalf("coinbase inputs=%d, want 0", len(at.Inputs))
	}
	if len(at.Outputs) != 1 {
		t.Fatalf("outputs=%d, want 1", len(at.Outputs))
	}
	// 50 BTC == 5_000_000_000 sats. Verify the sat round-trip is exact.
	wantSats := int64(50 * schema.SatsPerBTC)
	gotSats := schema.BTCToSats(at.Outputs[0].AmountBTC)
	if gotSats != wantSats {
		t.Fatalf("output sats=%d, want %d (BTC=%v)", gotSats, wantSats, at.Outputs[0].AmountBTC)
	}
	if !at.Confirmed || at.BlockHeight != 0 {
		t.Fatalf("confirmed=%v height=%d, want true/0", at.Confirmed, at.BlockHeight)
	}
	if at.Weight <= 0 || at.VSize <= 0 || at.VSize != vsizeFromWeight(at.Weight) {
		t.Fatalf("size fields invalid: weight=%d vsize=%d", at.Weight, at.VSize)
	}
	t.Logf("genesis OK: 1 output = %d sats, weight=%d vsize=%d confirmed=%v",
		gotSats, at.Weight, at.VSize, at.Confirmed)
}

func TestLiveFetchWalletHistoryMapsSats(t *testing.T) {
	p := liveProvider(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Satoshi's genesis output address. Rich, immutable confirmed history.
	const addr = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	page, err := p.FetchWalletHistory(ctx, acquisition.WalletHistoryRequest{Address: addr, PageSize: 50})
	if err != nil {
		t.Fatalf("wallet history: %v", err)
	}
	if len(page.Transactions) == 0 {
		t.Fatalf("expected transactions for %s", addr)
	}
	// Every mapped tx must have a txid and consistent sat/BTC values.
	for _, at := range page.Transactions {
		if at.TxID == "" {
			t.Fatalf("empty txid in page")
		}
		for _, o := range at.Outputs {
			if schema.BTCToSats(o.AmountBTC) < 0 {
				t.Fatalf("negative sats for %s", at.TxID)
			}
			// BTC->sats->BTC must be stable (no float drift at 8 dp).
			if schema.SatsToBTC(schema.BTCToSats(o.AmountBTC)) != o.AmountBTC {
				t.Fatalf("sat round-trip drift tx=%s btc=%v", at.TxID, o.AmountBTC)
			}
		}
	}
	t.Logf("wallet history OK: %d txs mapped, pagination cursor=%q done=%v",
		len(page.Transactions), page.Next.Cursor, page.Done)
}
