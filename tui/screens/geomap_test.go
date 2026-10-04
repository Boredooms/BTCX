package screens

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/geoip"
	"github.com/bctx/bctx/tui/mapdata"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMsg builds a tea.KeyMsg for a key name, handling the special keys the Geo
// Map uses (tab/enter) and single-rune keys (c, +, -, hjkl).
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// cityFixtureMMDB is the committed synthetic City-edition fixture (BCTX's own,
// NOT MaxMind data). Its single-node tree maps the whole IPv4 space to East
// Finchley, GB (51.5967, -0.1593) so any seeded IP resolves to coordinates.
const cityFixtureMMDB = "testdata/city-fixture.mmdb"

// mapAssetFixture is the committed tiny world-geometry fixture (shared with the
// mapdata package tests).
const mapAssetFixture = "../mapdata/testdata/world-110m.asset"

// geoFixtureCtx builds a ScreenCtx wired to a seeded repo AND installs the City
// GeoIP fixture + the world-geometry asset under a throwaway HOME, so the Geo
// Map screen's geoenrich.Enrich (which reads ~/.bctx) resolves coordinates and
// draws the plotted map deterministically — no real ~/.bctx, no network.
func geoFixtureCtx(t *testing.T, obs []schema.NetworkObservation) *ScreenCtx {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	// Install the City GeoIP fixture under $HOME/.bctx/geoip.
	geoDir := filepath.Join(home, ".bctx", "geoip")
	if _, err := geoip.Install(geoDir, cityFixtureMMDB, geoip.RegistryEntry{
		Source: "BCTX synthetic City fixture", Version: "fixture",
	}); err != nil {
		t.Fatalf("install city fixture: %v", err)
	}
	// Install the world-geometry asset under $HOME/.bctx/mapdata.
	mapDir := filepath.Join(home, ".bctx", "mapdata")
	if _, err := mapdata.Install(mapDir, mapAssetFixture, mapdata.RegistryEntry{
		Source: "Natural Earth (fixture)", Version: "fixture", License: "public domain",
	}); err != nil {
		t.Fatalf("install map asset: %v", err)
	}

	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "geo.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	ctx := context.Background()
	if len(obs) > 0 {
		if err := repo.SaveNetworkObservations(ctx, obs); err != nil {
			t.Fatalf("seed observations: %v", err)
		}
	}
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	return &ScreenCtx{
		Ctx:    ctx,
		App:    &app.App{Repo: repo, CaseID: "geo-case", Cleanup: func() {}},
		Cfg:    cfg,
		Styles: theme.Build(theme.Default()),
	}
}

// seededGeoObs returns a small, deterministic set of network observations whose
// IPs the City fixture resolves to GB coordinates, with a couple of correlated
// transactions for the related-tx cross-link.
func seededGeoObs() []schema.NetworkObservation {
	t0 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	return []schema.NetworkObservation{
		{ID: "o1", TxID: "TXA", SrcIP: "81.2.69.142", DstIP: "151.101.1.69", Timestamp: t0},
		{ID: "o2", TxID: "TXB", SrcIP: "81.2.69.143", DstIP: "151.101.1.69", Timestamp: t0.Add(time.Hour)},
		{ID: "o3", TxID: "TXA", SrcIP: "81.2.69.142", DstIP: "203.0.113.9", Timestamp: t0.Add(2 * time.Hour)},
	}
}

// driveGeoMap runs a GeoMap through Init + its async result and returns it.
func driveGeoMap(t *testing.T, ctx *ScreenCtx) *GeoMap {
	t.Helper()
	g := NewGeoMap(ctx)
	if cmd := g.Init(); cmd != nil {
		for _, msg := range runBatch(cmd) {
			var m Model = g
			m, _ = m.Update(msg)
			g = m.(*GeoMap)
		}
	}
	return g
}

// TestGeoMapPlotsResolvedPoints asserts the Geo Map, with the City DB + map
// asset installed and resolvable observations, renders the plotted map (not the
// bin fallback) with the honest estimate wording and no fabricated-point claims.
func TestGeoMapPlotsResolvedPoints(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	g := driveGeoMap(t, ctx)
	out := g.View(components.Frame{W: 160, H: 50})

	assertNoOverflow(t, "geomap-plot", out, components.Frame{W: 160, H: 50})

	for _, want := range []string{
		"GEO MAP",
		"IP geolocation (estimate)",
		"metadata, not proof of custody or ownership",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("geo map missing %q in:\n%s", want, out)
		}
	}
	// No GeoIP / map NOT-INSTALLED notices when both assets are present.
	if strings.Contains(out, geoip.NotInstalledMessage) {
		t.Errorf("geo map must not show GeoIP-not-installed when the DB is installed:\n%s", out)
	}
	if strings.Contains(out, mapdata.NotInstalledMessage) {
		t.Errorf("geo map must not show map-not-installed when the asset is installed:\n%s", out)
	}
}

// TestGeoMapSelectionDetail asserts cycling to a point (n) opens a detail
// panel with the resolved city/country/ASN and the related transaction, all
// framed as metadata/estimate.
func TestGeoMapSelectionDetail(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	g := driveGeoMap(t, ctx)
	// Select the first cluster (n = next point; tab is reserved for the global
	// focus cycle, design §D.2).
	var m Model = g
	m, _ = m.Update(keyMsg("n"))
	g = m.(*GeoMap)
	out := g.View(components.Frame{W: 160, H: 50})

	for _, want := range []string{
		"POINT DETAIL",
		"IP (observed from)",
		"East Finchley",      // resolved city from the fixture
		"country (estimate)", // honest metadata framing on the row label
		"ASN metadata",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("geo map detail missing %q in:\n%s", want, out)
		}
	}
}

// TestGeoMapGoNavigatesToRelatedTx asserts pressing g ("go") on a selected point
// emits a NavSearch for its related transaction (IP-on-map -> related tx link).
// g is the Body-local cross-link key; Enter (tested separately) only opens the
// detail panel with NO navigation (design §D.2).
func TestGeoMapGoNavigatesToRelatedTx(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	g := driveGeoMap(t, ctx)
	var m Model = g
	m, _ = m.Update(keyMsg("n")) // select
	g = m.(*GeoMap)
	_, cmd := g.Update(keyMsg("g"))
	if cmd == nil {
		t.Fatal("g on a selected point should emit a navigation command")
	}
	msg := cmd()
	nav, ok := msg.(NavSearch)
	if !ok {
		t.Fatalf("expected NavSearch, got %T", msg)
	}
	if nav.Kind != "tx" || (nav.ID != "TXA" && nav.ID != "TXB") {
		t.Errorf("expected a tx nav to a seeded tx, got %+v", nav)
	}
}

// TestGeoMapEnterOpensDetailNoNav asserts pressing Enter on a selected point
// opens/confirms the detail panel WITHOUT emitting any navigation command
// (design §D.2: Enter opens detail, g navigates).
func TestGeoMapEnterOpensDetailNoNav(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	g := driveGeoMap(t, ctx)
	var m Model = g
	m, _ = m.Update(keyMsg("n")) // select
	g = m.(*GeoMap)
	_, cmd := g.Update(keyMsg("enter"))
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(NavSearch); ok {
				t.Fatalf("Enter must NOT navigate; got NavSearch %v", msg)
			}
		}
	}
	out := g.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out, "POINT DETAIL") {
		t.Errorf("Enter should keep the detail panel open, got:\n%s", out)
	}
}

// TestGeoMapNoGeometryPanIsNoop asserts that with no geometry asset the
// viewport degrades to the density list and pan keys are safe no-ops that never
// crash and never move a (non-existent) viewport.
func TestGeoMapNoGeometryPanIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // nothing installed -> no geometry
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "geo.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	if err := repo.SaveNetworkObservations(context.Background(), seededGeoObs()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	ctx := &ScreenCtx{
		Ctx:    context.Background(),
		App:    &app.App{Repo: repo, CaseID: "geo-case", Cleanup: func() {}},
		Cfg:    cfg,
		Styles: theme.Build(theme.Default()),
	}
	g := driveGeoMap(t, ctx)
	// Pan + select + go on the fallback list must not crash and must not emit a
	// nav from an empty selection.
	for _, k := range []string{"h", "l", "j", "k", "+", "-", "n", "N", "0"} {
		var m Model = g
		m, _ = m.Update(keyMsg(k))
		g = m.(*GeoMap)
	}
	if _, cmd := g.Update(keyMsg("g")); cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(NavSearch); ok {
				t.Error("g on an empty (no-geometry) selection must not navigate")
			}
		}
	}
	out := g.View(components.Frame{W: 160, H: 50})
	if strings.TrimSpace(out) == "" {
		t.Error("no-geometry geo map should still render the honest fallback, got empty")
	}
}

// TestGeoMapDegradesWithoutDB asserts that with NO GeoIP DB installed the screen
// shows the documented install message and the honest bin list, never a plotted
// (fabricated) point.
func TestGeoMapDegradesWithoutDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home) // empty ~/.bctx -> nothing installed
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "geo.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	if err := repo.SaveNetworkObservations(context.Background(), seededGeoObs()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	ctx := &ScreenCtx{
		Ctx:    context.Background(),
		App:    &app.App{Repo: repo, CaseID: "geo-case", Cleanup: func() {}},
		Cfg:    cfg,
		Styles: theme.Build(theme.Default()),
	}
	g := driveGeoMap(t, ctx)
	out := g.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out, geoip.NotInstalledMessage) {
		t.Errorf("expected the documented GeoIP install message, got:\n%s", out)
	}
	// Bin fallback shows country/ASN labels (the observations carry no Country,
	// so an honest "(unknown)" bin), never a plotted coordinate header.
	if strings.Contains(out, "IP geolocation (estimate)") {
		t.Errorf("no DB must not render the plotted map header:\n%s", out)
	}
}

// TestGeoMapEmptyStateIsActionable asserts that with NO observations the Geo
// Map shows the actionable "awaiting network observations" panel that explains
// WHY (block-explorer acquisition carries no IPs) and HOW to populate it (live
// monitoring / dataset import), rather than a bare "0 bins" map.
func TestGeoMapEmptyStateIsActionable(t *testing.T) {
	ctx := geoFixtureCtx(t, nil) // assets installed, but zero observations
	g := driveGeoMap(t, ctx)
	out := g.View(components.Frame{W: 160, H: 50})

	assertNoOverflow(t, "geomap-empty", out, components.Frame{W: 160, H: 50})
	for _, want := range []string{
		"AWAITING NETWORK OBSERVATIONS",
		"never peer IP",
		"Live monitoring",
		"Dataset / packet import",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("empty geo map missing actionable text %q in:\n%s", want, out)
		}
	}
}

// TestGeoMapPreview prints the 160x50 Geo Map with resolved points so a human
// can eyeball the plotted coastline + markers. Run with:
//
//	go test -run TestGeoMapPreview -v ./tui/screens
func TestGeoMapPreview(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	g := driveGeoMap(t, ctx)
	// Select the first point so the detail panel is shown in the preview too
	// (n = next point; tab is the global focus cycle, design §D.2).
	var m Model = g
	m, _ = m.Update(keyMsg("n"))
	g = m.(*GeoMap)
	out := g.View(components.Frame{W: 160, H: 50})
	t.Logf("\n%s", out)
}
