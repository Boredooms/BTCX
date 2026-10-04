package tui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

// seededSubjectRoot builds a Root over a case holding one transaction so the
// default-subject fallback has something real to resolve.
func seededSubjectRoot(t *testing.T) *Root {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "subj.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	tx := schema.Transaction{
		TxID:      "TXDEFAULT1",
		Timestamp: time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC),
		FeeBTC:    0.0002,
		Inputs:    []schema.TransactionInput{{Address: "ADDRIN", AmountBTC: 2.0, Index: 0}},
		Outputs:   []schema.TransactionOutput{{Address: "ADDROUT", AmountBTC: 1.9, Index: 0}},
	}
	if err := repo.SaveTransactions(context.Background(), []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	a := &app.App{Repo: repo, CaseID: "subj-case", Cleanup: func() {}}
	return NewRoot(context.Background(), a)
}

// TestNavToResolvesDefaultSubject asserts jumping to a subject-scoped screen
// with no selected subject resolves a case default (the most recent tx, or its
// primary address for wallet-shaped screens) instead of leaving it empty.
func TestNavToResolvesDefaultSubject(t *testing.T) {
	cases := []struct {
		id       ScreenID
		wantID   string
		wantKind SubjectKind
	}{
		{ScreenTransaction, "TXDEFAULT1", SubjectTx},
		{ScreenGraph, "TXDEFAULT1", SubjectTx},
		{ScreenWallet, "ADDRIN", SubjectWallet},
		{ScreenEntity, "ADDRIN", SubjectWallet},
		{ScreenDetection, "ADDRIN", SubjectWallet},
	}
	for _, tc := range cases {
		r := seededSubjectRoot(t)
		r.navTo(tc.id, Subject{})
		got := r.state.Subject
		if got.ID != tc.wantID || got.Kind != tc.wantKind {
			t.Errorf("navTo(%s): subject = %+v, want {ID:%s Kind:%s}",
				tc.id, got, tc.wantID, tc.wantKind)
		}
	}
}

// TestNavToReplacesMismatchedKind asserts that carrying a wallet subject into
// the Transaction screen (which needs a tx id) resolves a fresh tx default
// instead of passing the wallet id through to GetTransaction ("not found").
func TestNavToReplacesMismatchedKind(t *testing.T) {
	r := seededSubjectRoot(t)
	// Simulate a prior visit that selected the wallet subject.
	r.state.Subject = Subject{ID: "ADDRIN", Kind: SubjectWallet}
	// Now jump to the Transaction screen with no explicit subject.
	r.navTo(ScreenTransaction, Subject{})
	if r.state.Subject.Kind != SubjectTx || r.state.Subject.ID != "TXDEFAULT1" {
		t.Errorf("Transaction screen should get a tx default, got %+v", r.state.Subject)
	}
	// Conversely, going back to Wallet with the tx subject should resolve a
	// wallet default.
	r.navTo(ScreenWallet, Subject{})
	if r.state.Subject.Kind != SubjectWallet || r.state.Subject.ID != "ADDRIN" {
		t.Errorf("Wallet screen should get a wallet default, got %+v", r.state.Subject)
	}
}

// TestNavToKeepsExplicitSubject asserts an explicit subject always wins over the
// default fallback.
func TestNavToKeepsExplicitSubject(t *testing.T) {
	r := seededSubjectRoot(t)
	want := Subject{ID: "EXPLICIT", Kind: SubjectWallet}
	r.navTo(ScreenWallet, want)
	if r.state.Subject != want {
		t.Errorf("explicit subject not kept: got %+v, want %+v", r.state.Subject, want)
	}
}

// TestNavToNoDefaultWithoutData asserts an empty case yields no fabricated
// subject (the screen keeps its honest placeholder).
func TestNavToNoDefaultWithoutData(t *testing.T) {
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	a := &app.App{Repo: repo, CaseID: "empty-case", Cleanup: func() {}}
	r := NewRoot(context.Background(), a)
	r.navTo(ScreenWallet, Subject{})
	if !r.state.Subject.Empty() {
		t.Errorf("empty case should not fabricate a subject, got %+v", r.state.Subject)
	}
}
