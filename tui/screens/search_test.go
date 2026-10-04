package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// search_test.go covers the in-screen Search screen's reachability and result
// handling (design §B.2): a submit runs a bounded lookup, a matched wallet row
// becomes a selectable row whose Enter emits NavSearch{Kind:"wallet"}, empty and
// too-short submits show honest notes with no lookup, a no-match shows honest
// empty text, a stale result (from a superseded query) is dropped, and the Seed
// hook pre-fills + looks up when navigated from the global `/` overlay.

// runSearchBatch resolves a (possibly batched) search cmd into its messages so
// the lookup results fold into the screen deterministically. The fixture repo
// returns real local rows; no network or timing is involved.
func runSearchBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	switch m := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, runSearchBatch(c)...)
		}
	case nil:
	default:
		out = append(out, m)
	}
	return out
}

// TestSearchWalletResultSelectable submits a wallet query against the seeded
// fixture repo and asserts a wallet row appears and its RowSelected emits a
// NavSearch{Kind:"wallet"}.
func TestSearchWalletResultSelectable(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	s := NewSearch(ctx)
	s.Init()

	// Submit the fixture wallet address.
	m, cmd := s.Update(components.SearchSubmitted{Query: "WFIXTURE"})
	for _, msg := range runSearchBatch(cmd) {
		m, _ = m.Update(msg)
	}
	out := m.View(components.Frame{W: 120, H: 40})
	if !strings.Contains(out, "wallet") {
		t.Fatalf("search for WFIXTURE should show a wallet row:\n%s", out)
	}

	// A RowSelected (as the table's Enter emits) yields NavSearch{wallet}.
	_, selCmd := m.Update(components.RowSelected{Kind: "wallet", ID: "WFIXTURE"})
	if selCmd == nil {
		t.Fatal("selecting a wallet row should emit a NavSearch cmd")
	}
	nav, ok := selCmd().(NavSearch)
	if !ok {
		t.Fatalf("expected NavSearch, got %T", selCmd())
	}
	if nav.Kind != "wallet" || nav.ID != "WFIXTURE" {
		t.Fatalf("NavSearch = %+v, want {wallet WFIXTURE}", nav)
	}
}

// TestAnalysisAutoRunsSingleMatch asserts the Analysis window auto-opens the
// subject (emitting NavSearch) when all local lookups complete with exactly one
// wallet/tx match — so entering a wallet runs the pipeline without a second
// keypress. The last dataLoaded's returned command carries the nav.
func TestAnalysisAutoRunsSingleMatch(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	s := NewSearch(ctx)
	s.Init()
	var m Model = s
	m, cmd := m.Update(components.SearchSubmitted{Query: "WFIXTURE"})

	var navCmd tea.Cmd
	for _, msg := range runSearchBatch(cmd) {
		var c tea.Cmd
		m, c = m.Update(msg)
		if c != nil {
			navCmd = c
		}
	}
	if navCmd == nil {
		t.Fatal("single wallet match should auto-emit a NavSearch to run the pipeline")
	}
	nav, ok := navCmd().(NavSearch)
	if !ok {
		t.Fatalf("expected NavSearch, got %T", navCmd())
	}
	if nav.Kind != "wallet" || nav.ID != "WFIXTURE" {
		t.Fatalf("auto-run NavSearch = %+v, want {wallet WFIXTURE}", nav)
	}
}

// TestSearchEmptyAndTooShort asserts an empty / too-short submit shows an honest
// note and performs no lookup (no results, no error).
func TestSearchEmptyAndTooShort(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	for _, q := range []string{"", "ab"} {
		s := NewSearch(ctx)
		s.Init()
		m, cmd := s.Update(components.SearchSubmitted{Query: q})
		if cmd != nil {
			// A too-short/empty query must not dispatch a lookup command.
			t.Fatalf("query %q should not trigger a lookup", q)
		}
		out := m.View(components.Frame{W: 120, H: 40})
		if !strings.Contains(out, "too short") {
			t.Fatalf("query %q should show the honest too-short note:\n%s", q, out)
		}
	}
}

// TestSearchNoMatchHonestEmpty asserts a well-formed query that matches nothing
// shows honest empty text, not a fabricated row.
func TestSearchNoMatchHonestEmpty(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	s := NewSearch(ctx)
	s.Init()
	m, cmd := s.Update(components.SearchSubmitted{Query: "NO-SUCH-SUBJECT"})
	for _, msg := range runSearchBatch(cmd) {
		m, _ = m.Update(msg)
	}
	out := m.View(components.Frame{W: 120, H: 40})
	if !strings.Contains(out, "no wallet, tx, or IP match") {
		t.Fatalf("no-match query should show honest empty text:\n%s", out)
	}
}

// TestSearchStaleResultDropped asserts a dataLoaded whose Request does not match
// the current query (a superseded lookup) is dropped and does not add a row.
func TestSearchStaleResultDropped(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	s := NewSearch(ctx)
	s.Init()
	// Current query is "WFIXTURE".
	s.Update(components.SearchSubmitted{Query: "WFIXTURE"})
	// A stale wallet result keyed to a different (superseded) query.
	stale := &schema.Wallet{Address: "WSTALE"}
	m, _ := s.Update(dataLoaded{Request: "OLD-QUERY", Payload: stale})
	out := m.View(components.Frame{W: 120, H: 40})
	if strings.Contains(out, "WSTALE") {
		t.Fatalf("stale result should be dropped, but WSTALE appeared:\n%s", out)
	}
}

// TestSearchSeedPrefillsAndLooks asserts the Seed hook pre-fills the box and
// runs a lookup for a long-enough query, and only pre-fills (with the honest
// note) for a too-short seed.
func TestSearchSeedPrefillsAndLooks(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})

	// Long-enough seed -> a lookup cmd is returned and the box is pre-filled.
	s := NewSearch(ctx)
	cmd := s.Seed("WFIXTURE")
	if cmd == nil {
		t.Fatal("Seed with a valid query should return a cmd (focus+lookup)")
	}
	if got := s.box.Value(); got != "WFIXTURE" {
		t.Fatalf("Seed should pre-fill the box, got %q", got)
	}
	for _, msg := range runSearchBatch(cmd) {
		var m Model = s
		m, _ = m.Update(msg)
		s = m.(*Search)
	}
	out := s.View(components.Frame{W: 120, H: 40})
	if !strings.Contains(out, "wallet") {
		t.Fatalf("seeded lookup should populate a wallet row:\n%s", out)
	}

	// Too-short seed -> pre-fills and shows the honest note.
	s2 := NewSearch(ctx)
	s2.Seed("ab")
	out2 := s2.View(components.Frame{W: 120, H: 40})
	if !strings.Contains(out2, "too short") {
		t.Fatalf("too-short seed should show the honest note:\n%s", out2)
	}
}
