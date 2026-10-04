package screens

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// spySummarizer counts Summarize calls so a test can prove the pane never
// invokes the summarizer from render paths (design §E.4).
type spySummarizer struct{ calls int32 }

func (s *spySummarizer) Summarize(ctx context.Context, in llm.SummaryInput) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	return "spy summary", nil
}

func (s *spySummarizer) count() int { return int(atomic.LoadInt32(&s.calls)) }

// fixedResult is a deterministic InvestigationResult for the projection tests.
func fixedResult() schema.InvestigationResult {
	return schema.InvestigationResult{
		Subject: "WFIXTURE",
		Risk:    schema.RiskAssessment{Score: 72},
		Patterns: []schema.PatternResult{
			{Type: "peeling_chain", Description: "sequential small spends"},
			{Type: "rapid_flow", Description: "burst of transfers"},
		},
		Evidence: []schema.EvidenceItem{
			{Description: "e1"}, {Description: "e2"}, {Description: "e3"},
			{Description: "e4"}, {Description: "e5"}, {Description: "e6"},
		},
		RelatedWallets: 5,
		RelevantTxs:    41,
	}
}

// TestBuildSummaryInput asserts the projection copies/derives presentation
// fields verbatim and derives RiskBand via the unexported riskBand helper —
// never computing a new value (design §E.2).
func TestBuildSummaryInput(t *testing.T) {
	res := fixedResult()
	in := BuildSummaryInput(res)

	if in.Subject != res.Subject {
		t.Errorf("Subject = %q, want %q", in.Subject, res.Subject)
	}
	if in.RiskScore != res.Risk.Score {
		t.Errorf("RiskScore = %d, want %d", in.RiskScore, res.Risk.Score)
	}
	if in.RiskBand != riskBand(res.Risk.Score) {
		t.Errorf("RiskBand = %q, want riskBand(%d) = %q", in.RiskBand, res.Risk.Score, riskBand(res.Risk.Score))
	}
	if in.Related != res.RelatedWallets {
		t.Errorf("Related = %d, want %d", in.Related, res.RelatedWallets)
	}
	if in.Txs != res.RelevantTxs {
		t.Errorf("Txs = %d, want %d", in.Txs, res.RelevantTxs)
	}
	// Evidence is capped at the top 5, copied verbatim.
	if len(in.Evidence) != 5 {
		t.Fatalf("Evidence len = %d, want top-5", len(in.Evidence))
	}
	for i, want := range []string{"e1", "e2", "e3", "e4", "e5"} {
		if in.Evidence[i] != want {
			t.Errorf("Evidence[%d] = %q, want %q", i, in.Evidence[i], want)
		}
	}
	// Patterns are "<type>: <description>", in order.
	wantPatterns := []string{"peeling_chain: sequential small spends", "rapid_flow: burst of transfers"}
	if len(in.Patterns) != len(wantPatterns) {
		t.Fatalf("Patterns len = %d, want %d", len(in.Patterns), len(wantPatterns))
	}
	for i, want := range wantPatterns {
		if in.Patterns[i] != want {
			t.Errorf("Patterns[%d] = %q, want %q", i, in.Patterns[i], want)
		}
	}
}

// TestExplainNoResultIsHonest asserts NIT-4: pressing L with no loaded result
// is a no-op note "open a subject first" and dispatches NO summarizer command.
func TestExplainNoResultIsHonest(t *testing.T) {
	spy := &spySummarizer{}
	c := &ScreenCtx{Ctx: context.Background(), Summarizer: spy, Styles: theme.Build(theme.Default())}
	var p summaryPane
	cmd := p.requestExplain(c, nil)
	if cmd != nil {
		t.Fatal("no-result explain must dispatch no command")
	}
	if !p.active || !p.loaded {
		t.Fatal("no-result explain should open the pane in a settled state")
	}
	if !strings.Contains(p.note, "open a subject first") {
		t.Fatalf("expected honest no-result note, got %q", p.note)
	}
	if spy.count() != 0 {
		t.Fatalf("summarizer must not be called with no result, got %d calls", spy.count())
	}
}

// TestExplainNeverInvokedByRender asserts the summarizer is never called from
// Init, View, or a tick — only on the explicit L action (design §E.4).
func TestExplainNeverInvokedByRender(t *testing.T) {
	spy := &spySummarizer{}
	cfg := configs.Default()
	cfg.LLM.Enabled = true // even enabled, render paths must not call it
	c := &ScreenCtx{Ctx: context.Background(), Summarizer: spy, Cfg: cfg,
		Styles: theme.Build(theme.Default()), Subject: Subject{ID: "WFIXTURE", Kind: SubjectWallet}}

	w := NewWallet(c)
	var m Model = w
	// Init.
	for _, msg := range runBatch(w.Init()) {
		m, _ = m.Update(msg)
	}
	// View at a few sizes.
	for _, f := range []components.Frame{{W: 120, H: 40}, {W: 80, H: 24}} {
		_ = m.View(f)
	}
	// A tick-like message (a stray dataLoaded of an unrelated kind).
	m, _ = m.Update(TickLikeMsg{})
	_ = m.View(components.Frame{W: 120, H: 40})

	if spy.count() != 0 {
		t.Fatalf("summarizer must not be invoked by Init/View/tick, got %d calls", spy.count())
	}
}

// TickLikeMsg is an arbitrary non-key message used to simulate a render/tick
// turn; it must never trigger the summarizer.
type TickLikeMsg struct{}

// TestExplainDegradesWhenAbsent asserts a nil/disabled summarizer yields a
// Degraded deterministic summary with NO network call (design §E.4). It drives
// the explainCmd directly so the degrade is deterministic.
func TestExplainDegradesWhenAbsent(t *testing.T) {
	in := BuildSummaryInput(fixedResult())

	// nil summarizer -> deterministic, not degraded (there was no failure), but
	// never a network call. We assert the text equals the deterministic render.
	msg := explainCmd(context.Background(), nil, in)()
	loaded, ok := msg.(SummaryLoaded)
	if !ok {
		t.Fatalf("explainCmd should yield SummaryLoaded, got %T", msg)
	}
	det, _ := llm.NewDeterministic().Summarize(context.Background(), in)
	if loaded.Text != det {
		t.Fatalf("nil summarizer should produce the deterministic summary:\n got %q\nwant %q", loaded.Text, det)
	}
	if loaded.Degraded {
		t.Error("a nil summarizer is the deterministic default, not a degrade")
	}

	// A summarizer that errors -> Degraded true, text falls back to deterministic.
	msg2 := explainCmd(context.Background(), errSummarizer{det: det}, in)()
	loaded2 := msg2.(SummaryLoaded)
	if !loaded2.Degraded {
		t.Error("an erroring summarizer must set Degraded")
	}
	if loaded2.Text != det {
		t.Fatalf("degrade must fall back to the deterministic summary, got %q", loaded2.Text)
	}
}

// errSummarizer models the ollama degrade path: it returns the deterministic
// text alongside an error, exactly as the adapter does on an unreachable model.
type errSummarizer struct{ det string }

func (e errSummarizer) Summarize(ctx context.Context, in llm.SummaryInput) (string, error) {
	return e.det, context.DeadlineExceeded
}

// TestExplainPaneRendersHonestProvenance drives the Wallet screen through a
// loaded result + L and asserts the pane renders the summary and an honest note.
func TestExplainPaneRendersHonestProvenance(t *testing.T) {
	cfg := configs.Default() // LLM disabled by default
	c := &ScreenCtx{Ctx: context.Background(), Summarizer: llm.NewDeterministic(), Cfg: cfg,
		Styles: theme.Build(theme.Default()), Subject: Subject{ID: "WFIXTURE", Kind: SubjectWallet}}

	w := NewWallet(c)
	// Inject a loaded result directly (bypassing the orchestrator).
	res := fixedResult()
	w.result = &res
	// Press L.
	m, cmd := w.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("L")})
	for _, msg := range runBatch(cmd) {
		m, _ = m.Update(msg)
	}
	out := m.View(components.Frame{W: 140, H: 44})
	if !strings.Contains(out, explainPaneTitle) {
		t.Errorf("expected the LLM pane title in:\n%s", out)
	}
	if !strings.Contains(out, "local LLM disabled") {
		t.Errorf("expected the honest 'disabled' provenance note in:\n%s", out)
	}
}
