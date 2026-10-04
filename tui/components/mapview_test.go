package components

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/geoenrich"
	"github.com/bctx/bctx/tui/mapdata"
	tea "github.com/charmbracelet/bubbletea"
)

// mapKey builds a tea.KeyMsg for the MapView unit tests, handling the special
// Tab key (so the passthrough assertion is exact) and single-rune keys.
func mapKey(s string) tea.KeyMsg {
	if s == "tab" {
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// plottableMapView returns a MapView wired with coordinate-bearing clusters and
// a geometry asset (coastline + centroids) so plotting() is true and the
// viewport keys are live. The centroids make the nearest-region label
// deterministic.
func plottableMapView() MapView {
	mv := NewMapView(styles())
	res := geoenrich.Result{
		DistinctIPs: 12,
		Clusters: []geoenrich.Cluster{
			{Lon: 10, Lat: 50, Observations: 30, IPs: 5},
			{Lon: 77.6, Lat: 12.9, Observations: 12, IPs: 4},
			{Lon: -74, Lat: 40.7, Observations: 3, IPs: 3},
		},
	}
	geom := &mapdata.Geometry{
		Coastlines: []mapdata.Polyline{{Points: []mapdata.Point{{Lon: 0, Lat: 0}, {Lon: 10, Lat: 10}}}},
		Centroids: []mapdata.Centroid{
			{ISOCode: "IN", Name: "India", Point: mapdata.Point{Lon: 79, Lat: 22}},
			{ISOCode: "DE", Name: "Germany", Point: mapdata.Point{Lon: 10, Lat: 51}},
			{ISOCode: "US", Name: "United States", Point: mapdata.Point{Lon: -98, Lat: 39}},
		},
	}
	mv.SetEnrichment(res, geom)
	return mv
}

// TestMapViewPanMovesCenterByZoomStep asserts arrows/hjkl pan by the zoom-scaled
// step (20/zoom) and that opposite keys cancel out.
func TestMapViewPanMovesCenterByZoomStep(t *testing.T) {
	mv := plottableMapView()
	lon0, lat0 := mv.centerLon, mv.centerLat
	step := 20.0 / mv.zoom

	mv, _ = mv.Update(mapKey("l")) // pan right (+lon)
	if mv.centerLon != lon0+step {
		t.Errorf("pan right: centerLon = %v, want %v", mv.centerLon, lon0+step)
	}
	mv, _ = mv.Update(mapKey("h")) // pan left (-lon) back to origin
	if mv.centerLon != lon0 {
		t.Errorf("pan left should cancel right: centerLon = %v, want %v", mv.centerLon, lon0)
	}
	mv, _ = mv.Update(mapKey("k")) // pan up (+lat)
	if mv.centerLat != lat0+step {
		t.Errorf("pan up: centerLat = %v, want %v", mv.centerLat, lat0+step)
	}
	mv, _ = mv.Update(mapKey("j")) // pan down (-lat) back
	if mv.centerLat != lat0 {
		t.Errorf("pan down should cancel up: centerLat = %v, want %v", mv.centerLat, lat0)
	}

	// Arrows behave identically to hjkl.
	mv2 := plottableMapView()
	mv2, _ = mv2.Update(mapKey("right"))
	if mv2.centerLon != lon0+step {
		t.Errorf("arrow right: centerLon = %v, want %v", mv2.centerLon, lon0+step)
	}
}

// TestMapViewZoomClampsToRange asserts +/- change zoom and clamp to [1,16].
func TestMapViewZoomClampsToRange(t *testing.T) {
	mv := plottableMapView()
	if mv.zoom != 1 {
		t.Fatalf("initial zoom = %v, want 1", mv.zoom)
	}
	for i := 0; i < 10; i++ {
		mv, _ = mv.Update(mapKey("+"))
	}
	if mv.zoom != 16 {
		t.Errorf("zoom should clamp at 16, got %v", mv.zoom)
	}
	for i := 0; i < 10; i++ {
		mv, _ = mv.Update(mapKey("-"))
	}
	if mv.zoom != 1 {
		t.Errorf("zoom should clamp at 1, got %v", mv.zoom)
	}
}

// TestMapViewResetReturnsHome asserts 0 resets center/zoom/selected to the home
// view after panning/zooming/selecting away.
func TestMapViewResetReturnsHome(t *testing.T) {
	mv := plottableMapView()
	mv, _ = mv.Update(mapKey("+"))
	mv, _ = mv.Update(mapKey("l"))
	mv, _ = mv.Update(mapKey("n"))
	mv, _ = mv.Update(mapKey("0"))
	if mv.centerLon != 0 || mv.centerLat != 20 || mv.zoom != 1 || mv.selected != 0 {
		t.Errorf("reset: got lon=%v lat=%v zoom=%v selected=%d, want 0/20/1/0",
			mv.centerLon, mv.centerLat, mv.zoom, mv.selected)
	}
}

// TestMapViewSelectionAdvancesAndReverses asserts n advances and N reverses the
// selection (wrapping) and recenters on the selected cluster.
func TestMapViewSelectionAdvancesAndReverses(t *testing.T) {
	mv := plottableMapView()
	n := len(mv.enriched.Clusters)
	if n != 3 {
		t.Fatalf("fixture should have 3 clusters, got %d", n)
	}

	mv, _ = mv.Update(mapKey("n")) // 0 -> 1
	if mv.selected != 1 {
		t.Errorf("n: selected = %d, want 1", mv.selected)
	}
	// Recenters on the selected cluster.
	c := mv.enriched.Clusters[1]
	if mv.centerLon != c.Lon || mv.centerLat != c.Lat {
		t.Errorf("n should recenter on cluster 1 (%v,%v), got (%v,%v)",
			c.Lon, c.Lat, mv.centerLon, mv.centerLat)
	}

	mv, _ = mv.Update(mapKey("N")) // 1 -> 0
	if mv.selected != 0 {
		t.Errorf("N: selected = %d, want 0", mv.selected)
	}
	mv, _ = mv.Update(mapKey("N")) // 0 -> wraps to 2
	if mv.selected != n-1 {
		t.Errorf("N should wrap to last cluster, selected = %d, want %d", mv.selected, n-1)
	}
}

// TestMapViewClampBounds asserts panning far clamps center to the documented
// bounds (lon ±180, lat ±85).
func TestMapViewClampBounds(t *testing.T) {
	mv := plottableMapView()
	for i := 0; i < 100; i++ {
		mv, _ = mv.Update(mapKey("l")) // keep panning right
	}
	if mv.centerLon > 180 {
		t.Errorf("centerLon not clamped: %v > 180", mv.centerLon)
	}
	for i := 0; i < 100; i++ {
		mv, _ = mv.Update(mapKey("k")) // keep panning up
	}
	if mv.centerLat > 85 {
		t.Errorf("centerLat not clamped: %v > 85", mv.centerLat)
	}
}

// TestMapViewTabNotConsumed asserts a Tab KeyMsg does NOT change the MapView
// state — Tab is reserved globally for the Nav<->Body focus cycle and must pass
// through unconsumed (design §A.5/§D.2).
func TestMapViewTabNotConsumed(t *testing.T) {
	mv := plottableMapView()
	before := mv
	mv, cmd := mv.Update(mapKey("tab"))
	if cmd != nil {
		t.Error("MapView must not emit a command for Tab")
	}
	if mv.selected != before.selected ||
		mv.centerLon != before.centerLon ||
		mv.centerLat != before.centerLat ||
		mv.zoom != before.zoom {
		t.Errorf("Tab must not mutate MapView state: before{sel=%d lon=%v lat=%v zoom=%v} after{sel=%d lon=%v lat=%v zoom=%v}",
			before.selected, before.centerLon, before.centerLat, before.zoom,
			mv.selected, mv.centerLon, mv.centerLat, mv.zoom)
	}
}

// TestMapViewFallbackPanIsNoop asserts that without plotting (no geometry) the
// viewport keys are no-ops and nothing crashes; the density list still renders.
func TestMapViewFallbackPanIsNoop(t *testing.T) {
	mv := NewMapView(styles())
	mv.SetObservations(sampleObs()) // bins only, no clusters/geometry -> no plot
	before := mv
	for _, k := range []string{"h", "l", "j", "k", "+", "-", "n", "N"} {
		mv, _ = mv.Update(mapKey(k))
	}
	if mv.centerLon != before.centerLon || mv.centerLat != before.centerLat ||
		mv.zoom != before.zoom || mv.selected != before.selected {
		t.Errorf("fallback viewport keys must be no-ops, state changed: %+v -> %+v", before, mv)
	}
	out := mv.View(Frame{W: 80, H: 20})
	if strings.TrimSpace(out) == "" {
		t.Error("fallback should still render the density list")
	}
}

// TestMapViewShortHelpOmitsTab asserts the viewport help no longer advertises
// Tab (Finding 2) and now advertises n/N + reset.
func TestMapViewShortHelpOmitsTab(t *testing.T) {
	mv := plottableMapView()
	for _, b := range mv.ShortHelp() {
		for _, k := range b.Keys() {
			if k == "tab" {
				t.Error("MapView.ShortHelp must not advertise tab")
			}
		}
	}
	help := mv.ShortHelp()
	var sawSelect, sawReset bool
	for _, b := range help {
		for _, k := range b.Keys() {
			if k == "n" || k == "N" {
				sawSelect = true
			}
			if k == "0" {
				sawReset = true
			}
		}
	}
	if !sawSelect {
		t.Error("MapView.ShortHelp should advertise n/N selection")
	}
	if !sawReset {
		t.Error("MapView.ShortHelp should advertise 0 reset")
	}
}

// TestMapViewHeaderGoldenWide asserts the full viewport header at a WIDE frame
// (no truncation): prefix · zoom · center (E/W,N/S) · region · counts.
func TestMapViewHeaderGoldenWide(t *testing.T) {
	mv := plottableMapView()
	// Position deterministically: center over Bengaluru-ish, zoom 4.
	mv.centerLon, mv.centerLat, mv.zoom = 77.6, 12.9, 4
	got := mv.viewportHeader(120)
	want := "IP geolocation (estimate) · zoom x4 · center 77.6°E, 12.9°N · India (IN) · 3 clusters · 12 IPs"
	if got != want {
		t.Errorf("wide header golden mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestMapViewHeaderGoldenCompactDropsRegionFirst asserts the documented
// truncation order under width pressure: the region segment is dropped first
// (center coordinates and counts + zoom survive).
func TestMapViewHeaderGoldenCompactDropsRegionFirst(t *testing.T) {
	mv := plottableMapView()
	mv.centerLon, mv.centerLat, mv.zoom = 77.6, 12.9, 4

	// A width that fits the center coordinates but not the region segment.
	full := "IP geolocation (estimate) · zoom x4 · center 77.6°E, 12.9°N · India (IN) · 3 clusters · 12 IPs"
	noRegion := "IP geolocation (estimate) · zoom x4 · center 77.6°E, 12.9°N · 3 clusters · 12 IPs"
	w := len([]rune(noRegion)) // exactly fits the no-region header, not the full one
	if w >= len([]rune(full)) {
		t.Fatalf("test width %d should be smaller than the full header %d", w, len([]rune(full)))
	}
	got := mv.viewportHeader(w)
	if got != noRegion {
		t.Errorf("compact header should drop region first:\n got: %q\nwant: %q", got, noRegion)
	}
	if strings.Contains(got, "India") {
		t.Errorf("region segment should have been dropped, got: %q", got)
	}

	// Under even tighter pressure the center coordinates drop too, but counts +
	// zoom always survive.
	countsOnly := "IP geolocation (estimate) · zoom x4 · 3 clusters · 12 IPs"
	got2 := mv.viewportHeader(len([]rune(countsOnly)))
	if got2 != countsOnly {
		t.Errorf("tighter header should drop center next:\n got: %q\nwant: %q", got2, countsOnly)
	}
}
