package acquisition_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/blockchain/acquisition/explorer"
	"github.com/bctx/bctx/storage/sqlite"
)

// Opt-in (BCTX_LIVE=1): acquire a real small block end-to-end through the sync
// engine and canonical Persister, then confirm the block + its transactions are
// stored locally. Uses an early low-tx-count block to stay fast.
func TestLiveSyncBlockEndToEnd(t *testing.T) {
	if os.Getenv("BCTX_LIVE") != "1" {
		t.Skip("live test: set BCTX_LIVE=1")
	}
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	src := explorer.New(explorer.WithTimeout(30 * time.Second))
	eng := acquisition.NewEngine(src, repo, "live-case", acquisition.DefaultConfig())
	defer eng.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Block height 170: the famous first P2P payment (Satoshi -> Hal Finney),
	// 2 transactions. Small, immutable, meaningful.
	st, err := eng.SyncBlock(ctx, "", 170, true)
	if err != nil {
		t.Skipf("live unreachable: %v", err)
	}
	t.Logf("sync block 170: status=%s discovered=%d fetched=%d new=%d dup=%d hash=%s",
		st.Status, st.Discovered, st.Fetched, st.New, st.Duplicates, st.Target)

	blk, err := repo.GetBlockByHeight(ctx, 170)
	if err != nil || blk == nil {
		t.Fatalf("block 170 not stored: %v", err)
	}
	if blk.Height != 170 || blk.TxCount < 1 {
		t.Fatalf("block 170 wrong: height=%d txcount=%d", blk.Height, blk.TxCount)
	}
	// Its transactions should be in the canonical store.
	counts, _ := repo.Counts(ctx)
	if counts.Transactions < blk.TxCount {
		t.Fatalf("expected >= %d canonical txs, got %d", blk.TxCount, counts.Transactions)
	}
	// Idempotent re-acquire: no new transactions.
	st2, err := eng.SyncBlock(ctx, blk.Hash, 0, false)
	if err != nil {
		t.Fatalf("re-sync: %v", err)
	}
	if st2.New != 0 {
		t.Fatalf("re-acquire added %d new (want 0, idempotent)", st2.New)
	}
	t.Logf("re-acquire idempotent: new=%d dup=%d", st2.New, st2.Duplicates)
}
