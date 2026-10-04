package app

import (
	"context"
	"testing"

	"github.com/bctx/bctx/cases/manager"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// tempConfig returns a config whose runtime tree is isolated to t.TempDir so a
// test never touches the real ~/.bctx. The network mode is online by default
// (callers override it for the offline-gate tests).
func tempConfig(t *testing.T) *configs.Config {
	t.Helper()
	cfg := configs.Default()
	cfg.App.DataDir = t.TempDir()
	// Point the models dir somewhere empty so ModelsDir tests are explicit.
	cfg.Models.Directory = t.TempDir()
	return cfg
}

// seedCase creates an isolated case and saves one transaction whose input
// address is addr, so GetWallet(addr) and WalletTransactions(addr) return data.
// It returns the created case id.
func seedCase(t *testing.T, cfg *configs.Config, name, addr, txid string) string {
	t.Helper()
	ctx := context.Background()
	cases := manager.New(configs.NewLayout(cfg))
	c, err := cases.Create(ctx, name)
	if err != nil {
		t.Fatalf("create case %q: %v", name, err)
	}
	repo, err := cases.OpenRepository(c.ID)
	if err != nil {
		t.Fatalf("open repo %q: %v", c.ID, err)
	}
	defer repo.Close()
	tx := schema.Transaction{
		TxID:       txid,
		Inputs:     []schema.TransactionInput{{Address: addr, AmountBTC: 1.0, Index: 0}},
		Outputs:    []schema.TransactionOutput{{Address: addr + "-out", AmountBTC: 0.9, Index: 0}},
		Provenance: schema.Provenance{SourceType: schema.SourceSynthetic},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	return c.ID
}

// TestBuildNoActiveCase asserts Build wires an Engine with nil service fields
// and leaves Repo nil + CaseID empty when no case is open, with a safe Cleanup.
func TestBuildNoActiveCase(t *testing.T) {
	cfg := tempConfig(t)

	a, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer a.Cleanup()

	if a.Engine == nil {
		t.Fatal("Engine must be non-nil")
	}
	if a.Cases == nil {
		t.Fatal("Cases must be non-nil")
	}
	if a.Repo != nil {
		t.Errorf("Repo must be nil with no active case, got %T", a.Repo)
	}
	if a.CaseID != "" {
		t.Errorf("CaseID must be empty with no active case, got %q", a.CaseID)
	}
	if a.Cleanup == nil {
		t.Fatal("Cleanup must always be non-nil")
	}
	assertEngineServiceFieldsNil(t, a.Engine)
}

// TestBuildWithActiveCase asserts Build attaches the active-case Repo (and
// CaseID) when a case exists, while Engine service fields remain nil.
func TestBuildWithActiveCase(t *testing.T) {
	cfg := tempConfig(t)
	id := seedCase(t, cfg, "case-one", "W1", "TX1")

	a, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer a.Cleanup()

	if a.Repo == nil {
		t.Fatal("Repo must be non-nil with an active case")
	}
	if a.CaseID != id {
		t.Errorf("CaseID = %q, want %q", a.CaseID, id)
	}
	assertEngineServiceFieldsNil(t, a.Engine)

	// The attached Repo must read the seeded case's data.
	w, err := a.Repo.GetWallet(context.Background(), "W1")
	if err != nil || w == nil {
		t.Fatalf("expected wallet W1 in active case, err=%v w=%v", err, w)
	}
}

// TestReopenCaseIsolation asserts the single case-switch contract: after
// ReopenCase the old repo handle is closed and reads return only the new case's
// data — no cross-case leakage, and no Cleanup()+Build() rebuild.
func TestReopenCaseIsolation(t *testing.T) {
	cfg := tempConfig(t)
	idA := seedCase(t, cfg, "alpha", "WALLET-A", "TXA")
	seedCase(t, cfg, "bravo", "WALLET-B", "TXB")

	// Build picks up the active case (bravo was created last → active).
	a, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer a.Cleanup()

	ctx := context.Background()

	// Switch to case alpha via the committed contract.
	if err := a.ReopenCase(idA); err != nil {
		t.Fatalf("ReopenCase(%q): %v", idA, err)
	}
	if a.CaseID != idA {
		t.Errorf("CaseID = %q, want %q", a.CaseID, idA)
	}

	// Reads see only alpha's data.
	if w, err := a.Repo.GetWallet(ctx, "WALLET-A"); err != nil || w == nil {
		t.Fatalf("alpha wallet missing after ReopenCase: err=%v w=%v", err, w)
	}
	if w, err := a.Repo.GetWallet(ctx, "WALLET-B"); err == nil && w != nil {
		t.Error("cross-case leak: bravo's wallet visible in alpha's repo")
	}

	// Engine/Cases preserved across the switch.
	if a.Engine == nil || a.Cases == nil {
		t.Fatal("Engine/Cases must be preserved across ReopenCase")
	}
}

// TestReopenCaseClosesOldRepo asserts the previous repo handle is closed by
// ReopenCase (the isolation guarantee the design §1.1 makes). We capture the
// first repo via Build, switch away, and verify the old handle is unusable.
func TestReopenCaseClosesOldRepo(t *testing.T) {
	cfg := tempConfig(t)
	idA := seedCase(t, cfg, "alpha", "WALLET-A", "TXA")
	idB := seedCase(t, cfg, "bravo", "WALLET-B", "TXB")

	a, err := Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	defer a.Cleanup()

	// Ensure a deterministic starting case, then switch to the other.
	if err := a.ReopenCase(idB); err != nil {
		t.Fatalf("ReopenCase(%q): %v", idB, err)
	}
	oldRepo := a.Repo
	if err := a.ReopenCase(idA); err != nil {
		t.Fatalf("ReopenCase(%q): %v", idA, err)
	}
	if a.Repo == oldRepo {
		t.Fatal("ReopenCase must swap in a new repo handle")
	}
	// The retired handle must be closed: a read on it should error.
	if _, err := oldRepo.GetWallet(context.Background(), "WALLET-B"); err == nil {
		t.Error("old repo handle should be closed after ReopenCase")
	}
}

// assertEngineServiceFieldsNil enforces the core contract that Build leaves the
// Engine's analysis service fields nil (they are constructed via services.go).
func assertEngineServiceFieldsNil(t *testing.T, e *sdk.Engine) {
	t.Helper()
	if e.Graph != nil {
		t.Error("Engine.Graph must be nil")
	}
	if e.Features != nil {
		t.Error("Engine.Features must be nil")
	}
	if e.ML != nil {
		t.Error("Engine.ML must be nil")
	}
	if e.Detection != nil {
		t.Error("Engine.Detection must be nil")
	}
	if e.Risk != nil {
		t.Error("Engine.Risk must be nil")
	}
	if e.Evidence != nil {
		t.Error("Engine.Evidence must be nil")
	}
	if e.Reports != nil {
		t.Error("Engine.Reports must be nil")
	}
}
