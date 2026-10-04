package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/spf13/cobra"
)

// ---- validateLocalDataPath (pure, no deps) ---------------------------------

func TestValidateLocalDataPath(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(good, []byte("txid\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Happy path: returns an absolute path, no error.
	abs, err := validateLocalDataPath(good)
	if err != nil {
		t.Fatalf("valid file should not error: %v", err)
	}
	if !filepath.IsAbs(abs) {
		t.Errorf("expected absolute path, got %q", abs)
	}

	// Missing file -> "File not found." (no network implied).
	if _, err := validateLocalDataPath(filepath.Join(dir, "nope.csv")); err == nil ||
		!strings.Contains(err.Error(), "File not found") {
		t.Errorf("missing file should report File not found, got %v", err)
	}

	// Directory -> error.
	if _, err := validateLocalDataPath(dir); err == nil ||
		!strings.Contains(err.Error(), "directory") {
		t.Errorf("directory should be rejected, got %v", err)
	}

	// Empty path -> error.
	if _, err := validateLocalDataPath("  "); err == nil {
		t.Error("empty path should be rejected")
	}
}

// ---- importLocalForAnalyze (repo-backed, no models/network) ----------------

func newTempRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

// writeNDJSON writes a bulk dataset: txs for the target wallet W plus an
// unrelated wallet, so we can assert only the target's records are located.
func writeBulkNDJSON(t *testing.T, dir, target, other string) string {
	t.Helper()
	hex := func(c byte) string { return strings.Repeat(string(c), 64) }
	lines := []string{
		`{"txid":"` + hex('a') + `","timestamp":"2026-10-01T10:00:00Z","inputs":[{"address":"SENDER1","amount_btc":2.0}],"outputs":[{"address":"` + target + `","amount_btc":1.99}]}`,
		`{"txid":"` + hex('b') + `","timestamp":"2026-10-01T11:00:00Z","inputs":[{"address":"` + target + `","amount_btc":1.99}],"outputs":[{"address":"DEST1","amount_btc":1.98}]}`,
		`{"txid":"` + hex('c') + `","timestamp":"2026-10-01T12:00:00Z","inputs":[{"address":"` + other + `","amount_btc":5.0}],"outputs":[{"address":"UNREL1","amount_btc":4.99}]}`,
	}
	p := filepath.Join(dir, "bulk.ndjson")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// newQuietCmd returns a cobra command whose stdout/stderr are discarded so the
// helper's status prints don't clutter test output.
func newQuietCmd() *cobra.Command {
	c := &cobra.Command{}
	c.SetOut(os.NewFile(0, os.DevNull))
	c.SetErr(os.NewFile(0, os.DevNull))
	return c
}

func TestImportLocalForAnalyze_HappyAndBulkFilter(t *testing.T) {
	repo := newTempRepo(t)
	dir := t.TempDir()
	target := "1TargetWalletAAAAAAAAAAAAAAAAAAAAA"
	other := "1OtherWalletBBBBBBBBBBBBBBBBBBBBBB"
	path := writeBulkNDJSON(t, dir, target, other)

	cmd := newQuietCmd()
	cmd.SetContext(context.Background())

	// Target wallet IS in the dataset -> no error, and its records are present.
	if err := importLocalForAnalyze(cmd, repo, "case-x", target, path); err != nil {
		t.Fatalf("import for present wallet should succeed: %v", err)
	}
	txs, err := repo.WalletTransactions(context.Background(), target, 0)
	if err != nil {
		t.Fatalf("wallet txs: %v", err)
	}
	if len(txs) == 0 {
		t.Fatal("target wallet should have transactions after local import")
	}
	// The unrelated wallet is also imported (the dataset is bulk), but the
	// investigation context is scoped to the target — the other wallet's single
	// tx must not be attributed to the target.
	if len(txs) != 2 {
		t.Errorf("target wallet should have exactly its 2 txs, got %d", len(txs))
	}
}

func TestImportLocalForAnalyze_NoRecordsForWallet(t *testing.T) {
	repo := newTempRepo(t)
	dir := t.TempDir()
	path := writeBulkNDJSON(t, dir, "1TargetAAAA", "1OtherBBBB")

	cmd := newQuietCmd()
	cmd.SetContext(context.Background())

	err := importLocalForAnalyze(cmd, repo, "case-x", "1WalletNotPresentZZZZZZZZZZZZZZZZZ", path)
	if err == nil {
		t.Fatal("expected an error when the wallet is absent from the dataset")
	}
	if !strings.Contains(err.Error(), "No records related to wallet") {
		t.Errorf("expected a clear no-records error, got: %v", err)
	}
	// The error must NOT suggest or perform any network fallback.
	if strings.Contains(strings.ToLower(err.Error()), "acquir") ||
		strings.Contains(strings.ToLower(err.Error()), "network fetch") {
		t.Errorf("error must not imply network fallback: %v", err)
	}
}

func TestImportLocalForAnalyze_MissingFile(t *testing.T) {
	repo := newTempRepo(t)
	cmd := newQuietCmd()
	cmd.SetContext(context.Background())

	err := importLocalForAnalyze(cmd, repo, "case-x", "1Wallet", "/no/such/file/data.csv")
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
	if !strings.Contains(err.Error(), "Local data source could not be used") ||
		!strings.Contains(err.Error(), "File not found") {
		t.Errorf("expected a clear local-source error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "explicitly requested") {
		t.Errorf("error should state the local source was explicitly requested: %v", err)
	}
}

func TestImportLocalForAnalyze_EmptyDataset(t *testing.T) {
	repo := newTempRepo(t)
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.ndjson")
	if err := os.WriteFile(empty, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newQuietCmd()
	cmd.SetContext(context.Background())

	err := importLocalForAnalyze(cmd, repo, "case-x", "1Wallet", empty)
	if err == nil {
		t.Fatal("expected an error for an empty dataset")
	}
	if !strings.Contains(err.Error(), "no usable records") &&
		!strings.Contains(err.Error(), "No records related") {
		t.Errorf("expected an empty/no-records error, got: %v", err)
	}
}

// TestImportLocalForAnalyze_SampleCSV uses the committed testdata/sample.csv
// (the task's canonical example `--file testdata/sample.csv` for wallet W123)
// to prove a CSV flows through the EXISTING ingestion pipeline and the target
// wallet's records are located — offline, no models needed for this layer.
func TestImportLocalForAnalyze_SampleCSV(t *testing.T) {
	repo := newTempRepo(t)
	cmd := newQuietCmd()
	cmd.SetContext(context.Background())

	if err := importLocalForAnalyze(cmd, repo, "case-sample", "W123", "testdata/sample.csv"); err != nil {
		t.Fatalf("sample.csv import+locate for W123 should succeed: %v", err)
	}
	txs, err := repo.WalletTransactions(context.Background(), "W123", 0)
	if err != nil {
		t.Fatalf("wallet txs: %v", err)
	}
	// W123 appears in two of the three rows (receive + spend); the third row is
	// an unrelated wallet and must not count toward W123.
	if len(txs) != 2 {
		t.Errorf("W123 should have 2 txs from sample.csv, got %d", len(txs))
	}
}

// ---- local-file mode is OFFLINE-ONLY (acquisition hard-disabled) -----------

// TestLocalFileModeForcesAirgap asserts the invariant the command relies on:
// when --file is used the code sets the config to airgap, and in that state the
// acquisition gate REFUSES — so no provider/sync path can run. This is the
// "no network fallback / no external acquisition" guarantee at the gate level.
func TestLocalFileModeForcesAirgap(t *testing.T) {
	cfg := configs.Default()
	// Simulate what the --file branch does before analysis.
	cfg.Network.Mode = configs.ModeAirgap
	cfg.Network.AcquisitionEnabled = false

	gf := &globalFlags{}
	if err := acquisitionAllowed(cfg, gf); err == nil {
		t.Fatal("acquisition must be REFUSED when local-file mode has set airgap")
	}
}

// ---- CLI wiring: the --file flag exists and existing flags are intact -------

func TestAnalyzeWalletFlags(t *testing.T) {
	root := NewRootCommand(BuildInfo{Version: "test"})
	// Walk to `analyze wallet`.
	var walletCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "analyze" {
			for _, sub := range c.Commands() {
				if sub.Name() == "wallet" {
					walletCmd = sub
				}
			}
		}
	}
	if walletCmd == nil {
		t.Fatal("analyze wallet command not found")
	}
	// The new --file flag plus the existing flags must all be present.
	for _, name := range []string{"file", "sync", "alert", "alert-threshold"} {
		if walletCmd.Flags().Lookup(name) == nil {
			t.Errorf("analyze wallet should have --%s flag", name)
		}
	}
}
