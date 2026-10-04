package explorer

import (
	"context"
	"os"
	"testing"
	"time"
)

// Opt-in live block tests (BCTX_LIVE=1). Prove FetchBlock/FetchBlockByHeight
// return real metadata + txids from the live chain. Skip cleanly otherwise.
func TestLiveFetchBlockByHeight(t *testing.T) {
	if os.Getenv("BCTX_LIVE") != "1" {
		t.Skip("live test: set BCTX_LIVE=1")
	}
	p := New(WithTimeout(30 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Block 800000 is immutable; its hash is a known fact.
	const wantHash = "00000000000000000002a7c4c1e48d76c5a37902165a270156b7a8d72728a054"
	b, err := p.FetchBlockByHeight(ctx, 800000)
	if err != nil {
		t.Skipf("live unreachable: %v", err)
	}
	if b.Hash != wantHash {
		t.Fatalf("height 800000 hash = %q, want %q", b.Hash, wantHash)
	}
	if b.Height != 800000 {
		t.Fatalf("height = %d, want 800000", b.Height)
	}
	if b.TxCount <= 0 || len(b.TxIDs) == 0 {
		t.Fatalf("expected txids; got tx_count=%d txids=%d", b.TxCount, len(b.TxIDs))
	}
	if b.TimestampEpoch <= 0 {
		t.Fatalf("expected a block timestamp")
	}
	t.Logf("block 800000: hash=%s txs=%d size=%d weight=%d time=%s",
		b.Hash, b.TxCount, b.Size, b.Weight, time.Unix(b.TimestampEpoch, 0).UTC())
}

func TestLiveFetchBlockByHash(t *testing.T) {
	if os.Getenv("BCTX_LIVE") != "1" {
		t.Skip("live test: set BCTX_LIVE=1")
	}
	p := New(WithTimeout(30 * time.Second))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const hash = "00000000000000000002a7c4c1e48d76c5a37902165a270156b7a8d72728a054"
	b, err := p.FetchBlock(ctx, hash)
	if err != nil {
		t.Skipf("live unreachable: %v", err)
	}
	if b.Height != 800000 || len(b.TxIDs) != b.TxCount {
		t.Fatalf("block mismatch: height=%d txids=%d tx_count=%d", b.Height, len(b.TxIDs), b.TxCount)
	}
	t.Logf("block %s height=%d txids=%d", hash, b.Height, len(b.TxIDs))
}
