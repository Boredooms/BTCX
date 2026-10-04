package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProbeSearchTiming times the three Analysis-window lookups against the real
// demo-scratch case DB to find which one hangs/scans. Diagnostic — skips if the
// case DB isn't present. Run: go test ./storage/sqlite -run TestProbeSearchTiming -v
func TestProbeSearchTiming(t *testing.T) {
	home, _ := os.UserHomeDir()
	db := filepath.Join(home, ".bctx", "cases", "demo-scratch", "case.db")
	if _, err := os.Stat(db); err != nil {
		t.Skipf("demo-scratch case not present: %v", err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	ctx := context.Background()

	q := "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"

	timeIt := func(name string, fn func() error) {
		start := time.Now()
		err := fn()
		t.Logf("%-26s took %-10v err=%v", name, time.Since(start), err)
	}

	timeIt("GetWallet", func() error { _, e := repo.GetWallet(ctx, q); return e })
	timeIt("GetTransaction", func() error { _, e := repo.GetTransaction(ctx, q); return e })
	timeIt("NetworkObservationsByIP", func() error { _, e := repo.NetworkObservationsByIP(ctx, q); return e })
	timeIt("Counts", func() error { _, e := repo.Counts(ctx); return e })
}

// TestProbeIndexes lists the indexes present on the hot tables so we can see
// whether address / ip / txid lookups are index-backed.
func TestProbeIndexes(t *testing.T) {
	home, _ := os.UserHomeDir()
	db := filepath.Join(home, ".bctx", "cases", "demo-scratch", "case.db")
	if _, err := os.Stat(db); err != nil {
		t.Skipf("demo-scratch case not present: %v", err)
	}
	repo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	rows, err := repo.db.QueryContext(context.Background(),
		`SELECT name, tbl_name FROM sqlite_master WHERE type='index' ORDER BY tbl_name, name`)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, tbl string
		_ = rows.Scan(&name, &tbl)
		t.Logf("index %-40s on %s", name, tbl)
	}
}
