package screens

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// TestAnalysisRealCaseResolves drives the Analysis screen against the REAL
// demo-scratch case and asserts the lookup actually RESOLVES (pipeline leaves
// 'querying' and either auto-navs or shows a result) — reproducing the
// 'querying… forever' bug. Diagnostic; skips if the case is absent.
func TestAnalysisRealCaseResolves(t *testing.T) {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".bctx", "cases", "demo-scratch", "case.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("demo-scratch not present: %v", err)
	}
	raw, _ := os.ReadFile(dbPath)
	dst := filepath.Join(t.TempDir(), "case.db")
	_ = os.WriteFile(dst, raw, 0o644)
	repo, err := sqlite.NewRepository(dst)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := &ScreenCtx{
		Ctx:    context.Background(),
		App:    &app.App{Repo: repo, CaseID: "demo-scratch", Cleanup: func() {}},
		Cfg:    configs.Default(),
		Styles: theme.Build(theme.Default()),
	}
	s := NewSearch(ctx)
	s.Init()

	q := "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"
	var m Model = s
	m, cmd := m.Update(components.SearchSubmitted{Query: q})

	var nav *NavSearch
	for _, msg := range runSearchBatch(cmd) {
		var c tea.Cmd
		m, c = m.Update(msg)
		if c != nil {
			if n, ok := c().(NavSearch); ok {
				nav = &n
			}
		}
	}
	s = m.(*Search)
	t.Logf("after lookup: pending=%d runStage=%q offerStatus=%q rows=%d nav=%v",
		s.pending, s.runStage, s.offerStatus, len(s.resultRows.rows), nav)
	out := s.View(components.Frame{W: 110, H: 34})
	t.Logf("\n%s", out)

	if s.runStage == "querying" || s.pending > 0 {
		t.Errorf("BUG REPRODUCED: still querying (pending=%d runStage=%q)", s.pending, s.runStage)
	}
}
