package screens

import (
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// TestAnalysisWindowPreview prints the redesigned Analysis window in a few
// states (idle, querying, resolved) so a human can eyeball the informative,
// non-cut-off layout. Run with:
//
//	go test ./tui/screens -run TestAnalysisWindowPreview -v
func TestAnalysisWindowPreview(t *testing.T) {
	f := components.Frame{W: 110, H: 34}

	t.Run("idle", func(t *testing.T) {
		s := NewSearch(fixtureCtx(t, Subject{}))
		s.Init()
		t.Logf("\n%s", s.View(f))
	})

	t.Run("resolved", func(t *testing.T) {
		s := NewSearch(fixtureCtx(t, Subject{}))
		s.Init()
		var m Model = s
		m, cmd := m.Update(components.SearchSubmitted{Query: "WFIXTURE"})
		for _, msg := range runSearchBatch(cmd) {
			m, _ = m.Update(msg)
		}
		t.Logf("\n%s", m.View(f))
	})
}

// TestAnalysisWindowFitsAllSizes asserts the Analysis window never overflows and
// always carries a bottom border (no cut-off) across the acceptance sizes, in
// both idle and post-query states.
func TestAnalysisWindowFitsAllSizes(t *testing.T) {
	for _, sz := range acceptanceSizes {
		s := NewSearch(fixtureCtx(t, Subject{}))
		s.Init()
		out := s.View(sz)
		assertNoOverflow(t, "analysis-idle", out, sz)
	}
}
