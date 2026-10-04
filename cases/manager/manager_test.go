package manager

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/bctx/bctx/configs"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	cfg := configs.Default()
	cfg.App.DataDir = t.TempDir()
	cfg.Models.Directory = filepath.Join(cfg.App.DataDir, "models")
	layout := configs.NewLayout(cfg)
	if err := layout.EnsureAll(); err != nil {
		t.Fatalf("ensure layout: %v", err)
	}
	return New(layout)
}

func TestCreateOpenList(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	c, err := m.Create(ctx, "Case 001")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID != "case-001" {
		t.Fatalf("slug = %q, want case-001", c.ID)
	}

	// Active case should be the one just created.
	active, ok := m.Active()
	if !ok || active.ID != "case-001" {
		t.Fatalf("active = %+v ok=%v", active, ok)
	}

	// Duplicate create must fail.
	if _, err := m.Create(ctx, "Case 001"); err == nil {
		t.Fatal("expected duplicate create to fail")
	}

	cases, err := m.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("len(cases) = %d, want 1", len(cases))
	}
}

func TestCaseIsolation(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	if _, err := m.Create(ctx, "alpha"); err != nil {
		t.Fatalf("create alpha: %v", err)
	}
	if _, err := m.Create(ctx, "beta"); err != nil {
		t.Fatalf("create beta: %v", err)
	}

	alpha, err := m.OpenRepository("alpha")
	if err != nil {
		t.Fatalf("open alpha repo: %v", err)
	}
	defer alpha.Close()
	beta, err := m.OpenRepository("beta")
	if err != nil {
		t.Fatalf("open beta repo: %v", err)
	}
	defer beta.Close()

	// Each case has its own database file; counts start at zero independently.
	ac, _ := alpha.Counts(ctx)
	bc, _ := beta.Counts(ctx)
	if ac.Transactions != 0 || bc.Transactions != 0 {
		t.Fatal("new cases should start empty")
	}
}

func TestCloseCase(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	if _, err := m.Create(ctx, "gamma"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := m.Close(ctx, "gamma"); err != nil {
		t.Fatalf("close: %v", err)
	}
	cases, _ := m.List(ctx)
	if len(cases) != 1 || cases[0].Status != "closed" {
		t.Fatalf("expected closed case, got %+v", cases)
	}
}
