package tui

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/screens"
	tea "github.com/charmbracelet/bubbletea"
)

// TestRootForwardsInScreenSearchSubmitted is the regression for the
// "querying… forever" bug: an in-screen SearchSubmitted (from the Analysis box,
// NOT the global `/` overlay) must be forwarded to the active screen so its
// lookup runs. Previously the Root's `case components.SearchSubmitted` matched,
// its overlay guard failed, and the switch exited — swallowing the message so
// the lookup never fired. This drives the real Root and asserts the message
// reaches the Analysis screen (which dispatches a lookup command).
func TestRootForwardsInScreenSearchSubmitted(t *testing.T) {
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	// Seed one wallet so a lookup has something to resolve.
	tx := schema.Transaction{
		TxID: "TXAAA", Timestamp: time.Now().UTC(), FeeBTC: 0.0001,
		Inputs:  []schema.TransactionInput{{Address: "WSUBJECT", AmountBTC: 1, Index: 0}},
		Outputs: []schema.TransactionOutput{{Address: "WOUT", AmountBTC: 0.9, Index: 0}},
	}
	if err := repo.SaveTransactions(context.Background(), []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	a := &app.App{Repo: repo, CaseID: "a", Cleanup: func() {}}
	r := NewRoot(context.Background(), a)
	r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	// Navigate to the Analysis screen (id "search").
	r.navTo(ScreenSearch, Subject{})

	// Feed an in-screen SearchSubmitted (overlay is NOT open) through the Root.
	_, cmd := r.Update(components.SearchSubmitted{Query: "WSUBJECT"})
	if cmd == nil {
		t.Fatal("in-screen SearchSubmitted must reach the Analysis screen and dispatch a lookup cmd (was swallowed)")
	}
}

// TestNavToAnalysisTakesBodyFocusAndTypes is the regression for "can't type into
// the Analysis box": navigating to Analysis must move keyboard focus to the Body
// so typed characters reach the subject box (not the SideNav). Reached via a
// number-jump from the Dashboard, exactly as a user does.
func TestNavToAnalysisTakesBodyFocusAndTypes(t *testing.T) {
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	a := &app.App{Repo: repo, CaseID: "b", Cleanup: func() {}}
	r := NewRoot(context.Background(), a)
	r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})

	// Jump to Analysis via its nav key "2" (from the default Nav focus).
	r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if r.cur != ScreenSearch {
		t.Fatalf("'2' should open Analysis, got %q", r.cur)
	}
	if r.focus != FocusBody {
		t.Fatalf("Analysis must take Body focus so typing works, got %v", focusName(r.focus))
	}
	// Type a character — it must reach the box, not drive the SideNav.
	navBefore := r.navCursor
	r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
	if r.navCursor != navBefore {
		t.Errorf("typing in Analysis must not move the SideNav cursor (keys leaked to nav)")
	}
}

// Keep the screens import referenced (ScreenSearch is a tui id, but we assert
// the screen package's NavSearch type elsewhere).
var _ = screens.SubjectWallet
