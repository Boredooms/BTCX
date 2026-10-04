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
)

// TestGeoMapRealCaseProbe renders the Geo Map against the REAL demo-geo case DB
// (if present on this machine) using the REAL installed GeoIP + map assets, and
// prints what it draws. Diagnostic: it reveals whether the map plots pins or
// falls back, and why. Skips cleanly when the case DB isn't present.
//
//	go test ./tui/screens -run TestGeoMapRealCaseProbe -v
func TestGeoMapRealCaseProbe(t *testing.T) {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".bctx", "cases", "demo-geo", "case.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("demo-geo case not present (%v)", err)
	}
	// Copy to temp so the test never mutates the live case.
	raw, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read case db: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "case.db")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatalf("copy: %v", err)
	}
	repo, err := sqlite.NewRepository(dst)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	ctx := &ScreenCtx{
		Ctx:    context.Background(),
		App:    &app.App{Repo: repo, CaseID: "demo-geo", Cleanup: func() {}},
		Cfg:    configs.Default(),
		Styles: theme.Build(theme.Default()),
	}
	g := NewGeoMap(ctx)
	if cmd := g.Init(); cmd != nil {
		for _, msg := range runBatch(cmd) {
			var m Model = g
			m, _ = m.Update(msg)
			g = m.(*GeoMap)
		}
	}
	t.Logf("obsCount=%d gotObs=%v hasGeoDB=%v geoNote=%q mapNote=%q geom=%v",
		g.obsCount, g.gotObs, g.hasGeoDB, g.geoNote, g.mapNote, g.geom != nil)
	out := g.View(components.Frame{W: 160, H: 44})
	t.Logf("\n%s", out)
}
