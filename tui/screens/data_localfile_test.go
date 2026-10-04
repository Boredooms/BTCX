package screens

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// TestDataLocalFileImport drives the Data screen's local-file import end to end:
// focus the box (f), submit a CSV path, run the import command, and assert the
// dataset was imported and the target wallet is now analyzable from local
// evidence — all offline. Throwaway probe.
func TestDataLocalFileImport(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	dir := t.TempDir()
	csv := filepath.Join(dir, "wallets.csv")
	body := "txid,timestamp,input_address,input_amount,output_address,output_amount\n" +
		strings.Repeat("a", 64) + ",2026-10-01T10:00:00Z,1SENDER,2.0,1TARGETWALLETxxxxxxxxxxxxxxxxxxx,1.99\n" +
		strings.Repeat("b", 64) + ",2026-10-01T11:00:00Z,1TARGETWALLETxxxxxxxxxxxxxxxxxxx,1.99,1DEST,1.98\n"
	if err := os.WriteFile(csv, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewData(ctx)
	var m Model = d
	if cmd := d.Init(); cmd != nil {
		for _, msg := range runBatch(cmd) {
			m, _ = m.Update(msg)
		}
		d = m.(*Data)
	}

	// f focuses the file input.
	_, _ = d.Update(keyMsg("f"))
	if !d.Focused() {
		t.Fatal("f should focus the local-file input")
	}

	// Submit the path -> import command runs offline.
	_, cmd := d.Update(components.SearchSubmitted{Query: csv})
	if cmd == nil {
		t.Fatal("submitting a path should start an import")
	}
	var mm Model = d
	for _, msg := range runBatch(cmd) {
		mm, _ = mm.Update(msg)
	}
	d = mm.(*Data)

	// The import status line should report success (accepted records).
	out := d.View(components.Frame{W: 120, H: 30})
	if !strings.Contains(out, "imported") {
		t.Errorf("Data screen should report a successful import; got:\n%s", firstLines(out, 3))
	}

	// The target wallet must now be present in the case (analyzable offline).
	txs, err := ctx.repo().WalletTransactions(context.Background(), "1TARGETWALLETxxxxxxxxxxxxxxxxxxx", 0)
	if err != nil {
		t.Fatalf("wallet txs: %v", err)
	}
	if len(txs) != 2 {
		t.Errorf("imported target wallet should have 2 txs, got %d", len(txs))
	}
}
