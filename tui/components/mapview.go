package components

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/geoenrich"
	"github.com/bctx/bctx/tui/mapdata"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// BinKey selects how observations are binned (design §13).
type BinKey int

const (
	// BinByCountry bins observations by their Country code.
	BinByCountry BinKey = iota
	// BinByASN bins observations by their ASN.
	BinByASN
)

// Bin is one aggregated observation group with its density count.
type Bin struct {
	Key   string
	Count int
}

// MapView is a terminal-native renderer that plots GeoIP-resolved observation
// density on a low-res world coastline (design §13, §16). When a City-edition
// GeoIP DB and the world-geometry asset are both installed it draws the
// coastline and plots binned IP clusters at their lat/lon with a pannable,
// zoomable viewport and a selection detail panel. When either asset is missing
// it degrades HONESTLY to a country/ASN density list — it NEVER implies
// ownership and NEVER fabricates a coordinate or a coastline.
//
// It is metadata-only: every value comes from the live DB + real observations;
// zero hard-coded points. It binned-counts observations rather than plotting
// raw rows, so the marker count is bounded by the clustering grid, not the raw
// IP count.
type MapView struct {
	styles theme.Styles
	bins   []Bin
	by     BinKey
	total  int

	// Enrichment + geometry for the plotted map. enriched carries the resolved
	// clusters/locations; geom is the coastline + centroids (nil when the map
	// asset is not installed). selected indexes enriched.Clusters.
	enriched geoenrich.Result
	geom     *mapdata.Geometry
	hasEnr   bool

	// Viewport state: center lon/lat and a zoom factor (1 = whole world).
	centerLon, centerLat float64
	zoom                 float64
	selected             int
}

// NewMapView builds an empty MapView binned by country, centered on the world.
func NewMapView(styles theme.Styles) MapView {
	return MapView{styles: styles, by: BinByCountry, zoom: 1, centerLon: 0, centerLat: 20}
}

// SetBinKey switches the binning granularity.
func (m *MapView) SetBinKey(k BinKey) { m.by = k }

// BinKey returns the current binning granularity.
func (m MapView) BinKey() BinKey { return m.by }

// SetObservations bins a slice of observations by the current key. Observations
// with an empty key fall into an explicit "(unknown)" bin — never dropped and
// never fabricated into a coordinate (design §13 no-centroid fallback).
func (m *MapView) SetObservations(obs []schema.NetworkObservation) {
	counts := map[string]int{}
	for _, o := range obs {
		var key string
		switch m.by {
		case BinByASN:
			key = o.ASN
		default:
			key = o.Country
		}
		if strings.TrimSpace(key) == "" {
			key = "(unknown)"
		}
		counts[key]++
	}
	bins := make([]Bin, 0, len(counts))
	for k, c := range counts {
		bins = append(bins, Bin{Key: k, Count: c})
	}
	// Deterministic order: count desc, then key asc.
	sort.Slice(bins, func(i, j int) bool {
		if bins[i].Count != bins[j].Count {
			return bins[i].Count > bins[j].Count
		}
		return bins[i].Key < bins[j].Key
	})
	m.bins = bins
	m.total = len(obs)
}

// SetEnrichment supplies the GeoIP-resolved clusters/locations and the optional
// world geometry. When clusters with coordinates and geometry are present the
// View draws the plotted map; otherwise it falls back to the bin density list.
func (m *MapView) SetEnrichment(r geoenrich.Result, geom *mapdata.Geometry) {
	m.enriched = r
	m.geom = geom
	m.hasEnr = true
	if m.selected >= len(r.Clusters) {
		m.selected = 0
	}
}

// Bins returns the computed bins (for tests and the detail readout).
func (m MapView) Bins() []Bin { return m.bins }

// Selected returns the currently selected cluster, if any (for the detail panel
// a parent screen draws beside the map).
func (m MapView) Selected() (geoenrich.Cluster, bool) {
	if m.selected < 0 || m.selected >= len(m.enriched.Clusters) {
		return geoenrich.Cluster{}, false
	}
	return m.enriched.Clusters[m.selected], true
}

// plotting reports whether the plotted-map path is available (coordinate-bearing
// clusters AND an installed geometry asset). Without both, the honest fallback
// is the density list.
func (m MapView) plotting() bool {
	if m.geom == nil || len(m.geom.Coastlines) == 0 {
		return false
	}
	for _, c := range m.enriched.Clusters {
		if c.Observations > 0 {
			return true
		}
	}
	return false
}

// Init implements the sub-model contract.
func (m MapView) Init() tea.Cmd { return nil }

// Update handles pan/zoom and selection when plotting; otherwise a no-op.
//
// Keys: arrows/hjkl pan (zoom-scaled step), +/- zoom within [1,16], n/N move
// the selection forward/backward (recentering on the cluster), 0 resets to the
// home view (centerLon=0, centerLat=20, zoom=1, selected=0). Tab is deliberately
// NOT handled here — it is reserved globally for the Nav↔Body focus cycle
// (design §A.5/§D.2), so a tab KeyMsg passes through unconsumed. When plotting()
// is false (no City DB or no geometry) the viewport keys are honest no-ops: the
// density-list fallback has no viewport, so Update returns unchanged (design
// §D.5). The 0-reset is still honored so the home view is always recoverable.
func (m MapView) Update(msg tea.Msg) (MapView, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	// Honest fallback: with no plotted viewport, pan/zoom/select are no-ops. The
	// 0-reset is allowed through so the known home view is always recoverable.
	if !m.plotting() && km.String() != "0" {
		return m, nil
	}
	step := 20.0 / m.zoom
	switch km.String() {
	case "left", "h":
		m.centerLon -= step
	case "right", "l":
		m.centerLon += step
	case "up", "k":
		m.centerLat += step
	case "down", "j":
		m.centerLat -= step
	case "+", "=":
		if m.zoom < 16 {
			m.zoom *= 2
		}
	case "-", "_":
		if m.zoom > 1 {
			m.zoom /= 2
		}
	case "n":
		if len(m.enriched.Clusters) > 0 {
			m.selected = (m.selected + 1) % len(m.enriched.Clusters)
			// Recenter on the selected cluster so it's visible.
			c := m.enriched.Clusters[m.selected]
			m.centerLon, m.centerLat = c.Lon, c.Lat
		}
	case "N":
		if n := len(m.enriched.Clusters); n > 0 {
			// Previous cluster, wrapping to the end (recenter + clamp).
			m.selected = (m.selected - 1 + n) % n
			c := m.enriched.Clusters[m.selected]
			m.centerLon, m.centerLat = c.Lon, c.Lat
		}
	case "0":
		// Reset to the known home view (design §D.2/§D.5).
		m.centerLon, m.centerLat, m.zoom, m.selected = 0, 20, 1, 0
	}
	m.clampCenter()
	return m, nil
}

func (m *MapView) clampCenter() {
	if m.centerLon > 180 {
		m.centerLon = 180
	}
	if m.centerLon < -180 {
		m.centerLon = -180
	}
	if m.centerLat > 85 {
		m.centerLat = 85
	}
	if m.centerLat < -85 {
		m.centerLat = -85
	}
	if m.zoom < 1 {
		m.zoom = 1
	}
}

// ShortHelp lists the plotted-map viewport keys. Tab is intentionally absent:
// it is reserved globally for the Nav↔Body focus cycle (design §A.5/§D.2), so
// cluster-cycling is n (next) / N (previous), never Tab.
func (m MapView) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("hjkl"), key.WithHelp("hjkl/arrows", "pan")),
		key.NewBinding(key.WithKeys("+", "-"), key.WithHelp("+/-", "zoom")),
		key.NewBinding(key.WithKeys("n", "N"), key.WithHelp("n/N", "next/prev point")),
		key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset")),
	}
}

// View renders the honesty note plus either the plotted map (geometry + binned
// coordinate clusters) or the honest density-list fallback.
func (m MapView) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(g_mapNote(m.styles))
	b.WriteString("\n")
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	if m.plotting() {
		b.WriteString(m.renderPlot(Frame{W: f.W, H: bodyH}))
	} else {
		b.WriteString(m.renderBins(Frame{W: f.W, H: bodyH}))
	}
	return clampBlock(b.String(), f)
}

// renderBins is the honest fallback density list (no coordinates / no geometry).
func (m MapView) renderBins(f Frame) string {
	var b strings.Builder
	binLabel, binPlural := "country", "countries"
	if m.by == BinByASN {
		binLabel, binPlural = "ASN", "ASNs"
	}
	header := fmt.Sprintf("%s bins — %d observations across %d %s",
		binLabel, m.total, len(m.bins), binPlural)
	b.WriteString(m.styles.Role(theme.RoleInfo).Render(header))
	b.WriteString("\n")
	rows := f.H - 2
	for i, bin := range m.bins {
		if i >= rows {
			break
		}
		line := fmt.Sprintf("• %s (%d)", bin.Key, bin.Count)
		b.WriteString(m.styles.Role(theme.RoleValue).Render(clampLine(line, f.W)) + "\n")
	}
	return clampBlock(b.String(), f)
}

// renderPlot draws the coastline + plotted clusters into a character grid, with
// a one-line header reporting the viewport and a selected-point marker.
func (m MapView) renderPlot(f Frame) string {
	header := m.viewportHeader(f.W)
	gridH := f.H - 1
	if gridH < 1 {
		gridH = 1
	}
	grid := newPlotGrid(f.W, gridH, m.centerLon, m.centerLat, m.zoom)

	// Coastline strokes as a dim background.
	if m.geom != nil {
		for _, pl := range m.geom.Coastlines {
			for _, p := range pl.Points {
				grid.set(p.Lon, p.Lat, '·')
			}
		}
	}
	// Cluster markers over the coastline; the selected one is highlighted.
	for i, c := range m.enriched.Clusters {
		glyph := clusterGlyph(c.Observations)
		if i == m.selected {
			glyph = '◉'
		}
		grid.set(c.Lon, c.Lat, glyph)
	}

	var b strings.Builder
	b.WriteString(m.styles.Role(theme.RoleInfo).Render(clampLine(header, f.W)))
	b.WriteString("\n")
	b.WriteString(grid.render(m.styles))
	return clampBlock(b.String(), f)
}

// viewportHeader builds the plotted-map viewport indicator (design §D.3):
//
//	IP geolocation (estimate) · zoom x4 · center 77.6°E, 12.9°N · <region> · 7 clusters · 12 IPs
//
// Center lon/lat carry hemisphere letters (E/W, N/S) derived from the signed
// center fields. The region segment is the nearest world-centroid country label
// at the viewport center, appended ONLY when geometry centroids are available
// (confirmed by reading mapdata/geometry.go: Geometry.Centroids). Under width
// pressure segments are dropped left-to-right in a DOCUMENTED order so the
// counts + zoom always survive clampLine: the region is dropped first, then the
// center coordinates. The returned string is NOT yet clamped — renderPlot clamps
// it to f.W; this method just chooses which segments fit.
func (m MapView) viewportHeader(width int) string {
	const prefix = "IP geolocation (estimate)"
	zoomSeg := fmt.Sprintf("zoom x%.0f", m.zoom)
	centerSeg := fmt.Sprintf("center %s, %s", lonLabel(m.centerLon), latLabel(m.centerLat))
	countsSeg := fmt.Sprintf("%d clusters · %d IPs", len(m.enriched.Clusters), m.enriched.DistinctIPs)
	regionSeg := m.regionLabel()

	// Build candidate segment lists from most to least droppable: try the full
	// header first, then drop region, then drop center. counts + zoom + prefix
	// are never dropped (they are the honest minimum).
	candidates := [][]string{
		{prefix, zoomSeg, centerSeg, regionSeg, countsSeg},
		{prefix, zoomSeg, centerSeg, countsSeg},
		{prefix, zoomSeg, countsSeg},
	}
	for _, segs := range candidates {
		parts := make([]string, 0, len(segs))
		for _, s := range segs {
			if s != "" {
				parts = append(parts, s)
			}
		}
		h := strings.Join(parts, " · ")
		if width <= 0 || len([]rune(h)) <= width {
			return h
		}
	}
	// Nothing fit; return the minimum and let clampLine do the final cut.
	return strings.Join([]string{prefix, zoomSeg, countsSeg}, " · ")
}

// regionLabel returns the nearest world-centroid country label to the current
// viewport center, or "" when no centroid table is loaded (honest omission).
// The lookup is a pure O(n) great-circle minimum over the small centroid table
// (not in any per-cell hot loop); it is a presentation cue only and asserts
// nothing forensic.
func (m MapView) regionLabel() string {
	if m.geom == nil || len(m.geom.Centroids) == 0 {
		return ""
	}
	best := -1
	bestD := 0.0
	for i, c := range m.geom.Centroids {
		d := haversineDeg(m.centerLon, m.centerLat, c.Point.Lon, c.Point.Lat)
		if best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return ""
	}
	c := m.geom.Centroids[best]
	switch {
	case c.Name != "" && c.ISOCode != "":
		return fmt.Sprintf("%s (%s)", c.Name, c.ISOCode)
	case c.Name != "":
		return c.Name
	default:
		return c.ISOCode
	}
}

// lonLabel formats a signed longitude with an E/W hemisphere letter.
func lonLabel(lon float64) string {
	h := "E"
	if lon < 0 {
		h, lon = "W", -lon
	}
	return fmt.Sprintf("%.1f°%s", lon, h)
}

// latLabel formats a signed latitude with an N/S hemisphere letter.
func latLabel(lat float64) string {
	h := "N"
	if lat < 0 {
		h, lat = "S", -lat
	}
	return fmt.Sprintf("%.1f°%s", lat, h)
}

// haversineDeg returns a monotonic great-circle proximity measure (in squared
// angular terms) between two lon/lat points, used only to pick the nearest
// centroid. It avoids trig for speed while preserving the nearest-point ordering
// over the small centroid table (a plain angular distance is sufficient here).
func haversineDeg(lon1, lat1, lon2, lat2 float64) float64 {
	dLon := lon1 - lon2
	// Wrap longitude delta into [-180,180] so the antimeridian doesn't fool the
	// nearest-centroid pick.
	for dLon > 180 {
		dLon -= 360
	}
	for dLon < -180 {
		dLon += 360
	}
	dLat := lat1 - lat2
	return dLon*dLon + dLat*dLat
}

// clusterGlyph scales a marker by observation density (a visual cue only).
func clusterGlyph(obs int) rune {
	switch {
	case obs >= 50:
		return '█'
	case obs >= 20:
		return '▓'
	case obs >= 5:
		return '▒'
	default:
		return '◆'
	}
}

// plotGrid is an equirectangular projection into a fixed character grid. The
// viewport is centered on (centerLon, centerLat) and spans 360/zoom degrees of
// longitude; latitude span is derived to keep a roughly even aspect at terminal
// cell proportions (cells are ~2:1 tall:wide, so lat span ≈ lon span / 2).
type plotGrid struct {
	w, h               int
	cells              [][]rune
	centerLon, centLat float64
	lonSpan, latSpan   float64
}

func newPlotGrid(w, h int, centerLon, centerLat, zoom float64) *plotGrid {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	cells := make([][]rune, h)
	for i := range cells {
		row := make([]rune, w)
		for j := range row {
			row[j] = ' '
		}
		cells[i] = row
	}
	lonSpan := 360.0 / zoom
	// Terminal cells are about twice as tall as wide; halve the lat span so the
	// aspect reads roughly correct.
	latSpan := lonSpan / 2
	if latSpan > 170 {
		latSpan = 170
	}
	return &plotGrid{w: w, h: h, cells: cells, centerLon: centerLon, centLat: centerLat, lonSpan: lonSpan, latSpan: latSpan}
}

// set projects a lon/lat to a cell and writes glyph if it falls inside the grid.
func (g *plotGrid) set(lon, lat float64, glyph rune) {
	// x: left edge = centerLon - lonSpan/2.
	fx := (lon - (g.centerLon - g.lonSpan/2)) / g.lonSpan * float64(g.w)
	// y: top edge = centLat + latSpan/2 (north up).
	fy := ((g.centLat + g.latSpan/2) - lat) / g.latSpan * float64(g.h)
	x := int(fx)
	y := int(fy)
	if x < 0 || x >= g.w || y < 0 || y >= g.h {
		return
	}
	g.cells[y][x] = glyph
}

func (g *plotGrid) render(styles theme.Styles) string {
	var b strings.Builder
	for i, row := range g.cells {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(string(row))
	}
	return b.String()
}

// g_mapNote renders the persistent geolocation-honesty note (design §13).
func g_mapNote(s theme.Styles) string {
	return s.Role(theme.RoleWarning).Render(
		"Geolocation is network-observation metadata, not proof of custody or ownership.")
}
