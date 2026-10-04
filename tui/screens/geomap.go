package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/geoenrich"
	"github.com/bctx/bctx/tui/geoip"
	"github.com/bctx/bctx/tui/mapdata"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// GeoMap is the Geo Map screen (design §2, §13). It resolves the case's network
// observations through the installed GeoIP City DB (tui/geoenrich) and plots
// the binned IP clusters at their lat/lon over the local world-geometry
// coastline (tui/mapdata), pannable/zoomable, with a selection detail panel
// showing IP/country/city/ASN/observation-count/related tx + first-last-seen.
//
// It is metadata-only and says so: a persistent note states geolocation is
// observation metadata, not proof of ownership (AGENTS §18). It degrades
// honestly: with no City DB it shows the country/ASN density bins + the
// documented install message; with no geometry asset it still lists the bins —
// never a fabricated coordinate or coastline (design §13).
type GeoMap struct {
	ctx    *ScreenCtx
	styles theme.Styles

	view     components.MapView
	detail   components.DetailPanel
	geoNote  string
	mapNote  string
	hasGeoDB bool
	geom     *mapdata.Geometry

	// obsCount / gotObs track the loaded observation set so the screen can show
	// an explicit, actionable empty state (geo data comes from network
	// observations, which the Esplora provider does NOT serve — only live
	// monitoring or a dataset/packet import produces them) instead of a bare,
	// confusing "0 bins" map.
	obsCount int
	gotObs   bool

	// detailOpen records that Enter has confirmed/opened the detail panel for
	// the current selection (design §D.2). A selection via n/N shows the panel
	// too; Enter is the explicit "open/focus the detail" confirmation.
	detailOpen bool
}

// NewGeoMap builds the Geo Map screen.
func NewGeoMap(ctx *ScreenCtx) *GeoMap {
	g := &GeoMap{
		ctx:    ctx,
		styles: ctx.styles(),
		view:   components.NewMapView(ctx.styles()),
		detail: components.NewDetailPanel(ctx.styles()),
	}
	g.geoNote = geoDBNote()
	g.mapNote = mapAssetNote()
	g.hasGeoDB = GeoIPStatus() != "NOT INSTALLED"
	g.geom = loadMapGeometry()
	g.detail.SetSections(nil) // honest "no detail" until a point is selected
	return g
}

// Init loads all observations (bounded) for enrichment + binning.
func (g *GeoMap) Init() tea.Cmd {
	repo := g.ctx.repo()
	if repo == nil {
		return nil
	}
	return allObservationsCmd(g.ctx.bgCtx(), repo, networkObsLimit)
}

// Update folds in observations (resolving + binning via geoenrich), handles the
// bin-cycle key, and forwards pan/zoom/selection to the map view.
func (g *GeoMap) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if obs, ok := m.Payload.([]schema.NetworkObservation); ok {
			g.obsCount, g.gotObs = len(obs), true
			g.view.SetObservations(obs)
			// Resolve + cluster through the installed GeoIP DB (offline). The
			// enrichment caches each distinct IP once per render.
			res := geoenrich.Enrich(obs, 0)
			g.view.SetEnrichment(res, g.geom)
			g.refreshDetail()
		}
	case tea.KeyMsg:
		switch m.String() {
		case "c":
			if g.view.BinKey() == components.BinByCountry {
				g.view.SetBinKey(components.BinByASN)
			} else {
				g.view.SetBinKey(components.BinByCountry)
			}
			return g, nil
		case "enter":
			// Enter OPENS/ensures the detail panel for the currently selected
			// cluster — it does NOT navigate away (design §D.2). The split-view
			// detail is shown whenever a point is selected; refreshDetail keeps
			// the panel in sync with the current selection.
			g.detailOpen = true
			g.refreshDetail()
			return g, nil
		case "g":
			// "go" cross-link: open the related transaction for the selected
			// point, so a point on the map navigates to its tx (which in turn
			// links to wallets + the graph). NavSearch is the recognised
			// cross-screen nav signal the Root translates into a NavMsg. `g` is
			// handled ONLY here (map-screen Body-local, never a global binding,
			// design §D.2) so Table-based screens keep g = "jump to top".
			if cl, ok := g.view.Selected(); ok && len(cl.Members) > 0 {
				if tx := firstRelatedTx(cl.Members[0]); tx != "" {
					return g, func() tea.Msg { return NavSearch{Kind: "tx", ID: tx} }
				}
				// No related tx — fall back to inspecting the IP on the Network
				// screen (observed-from endpoint).
				return g, func() tea.Msg { return NavSearch{Kind: "ip", ID: cl.Members[0].IP} }
			}
			return g, nil
		}
		// Everything else (pan/zoom hjkl/arrows, n/N selection, +/- zoom, 0
		// reset) forwards to the viewport. The 0-reset is honored by
		// MapView.Update even on the fallback list so the home view is always
		// recoverable.
		var cmd tea.Cmd
		g.view, cmd = g.view.Update(m)
		g.refreshDetail()
		return g, cmd
	}
	return g, nil
}

// firstRelatedTx returns the first correlated transaction id for a resolved IP,
// or "" when the observation carried no tx.
func firstRelatedTx(l geoenrich.IPLocation) string {
	if len(l.RelatedTx) > 0 {
		return l.RelatedTx[0]
	}
	return ""
}

// refreshDetail rebuilds the selection detail panel from the currently selected
// cluster. All values are honest, metadata-framed, and come from the live DB +
// real observations (never ownership claims).
func (g *GeoMap) refreshDetail() {
	cl, ok := g.view.Selected()
	if !ok || len(cl.Members) == 0 {
		g.detail.SetSections(nil)
		return
	}
	top := cl.Members[0]
	rows := []components.Row{
		{Label: "IP (observed from)", Value: top.IP},
		{Label: "country (estimate)", Value: labelOrDash(countryLabel(top))},
		{Label: "city (estimate)", Value: labelOrDash(cityLabel(top))},
		{Label: "ASN metadata", Value: labelOrDash(top.ASN)},
		{Label: "observations", Value: fmt.Sprintf("%d", top.Observations)},
		{Label: "cluster IPs", Value: fmt.Sprintf("%d", cl.IPs)},
		{Label: "cluster obs", Value: fmt.Sprintf("%d", cl.Observations)},
	}
	if len(top.RelatedTx) > 0 {
		rows = append(rows, components.Row{Label: "related tx", Value: joinShort(top.RelatedTx, 3)})
	}
	if len(top.RelatedWallets) > 0 {
		rows = append(rows, components.Row{Label: "related wallets", Value: joinShort(top.RelatedWallets, 3)})
	}
	if !top.FirstSeen.IsZero() {
		rows = append(rows, components.Row{Label: "first seen", Value: top.FirstSeen.Format("2006-01-02 15:04")})
	}
	if !top.LastSeen.IsZero() {
		rows = append(rows, components.Row{Label: "last seen", Value: top.LastSeen.Format("2006-01-02 15:04")})
	}
	g.detail.SetSections([]components.Section{{
		Title: "selected point (IP geolocation estimate)",
		Rows:  rows,
		Body:  "Geolocation is observation metadata, not proof of ownership.",
	}})
}

// emptyBody renders the actionable no-observations panel. It is honest about
// WHY the map is empty (block-explorer acquisition carries no peer IPs) and
// names the two real sources of network observations, so an empty Geo Map reads
// as "no geo data yet, here's how" rather than a broken screen.
func (g *GeoMap) emptyBody(f components.Frame) string {
	lines := []string{
		g.styles.Role(theme.RoleLabel).Render("No network observations in this case — nothing to plot yet."),
		"",
		g.styles.Value.Render("Why:"),
		g.styles.Role(theme.RoleLabel).Render("  Block-explorer acquisition (Esplora: mempool.space / blockstream)"),
		g.styles.Role(theme.RoleLabel).Render("  serves transactions and blocks, never peer IP addresses. The map"),
		g.styles.Role(theme.RoleLabel).Render("  plots IP-geolocated network observations, which that path never yields."),
		"",
		g.styles.Value.Render("How to populate it:"),
		g.styles.Role(theme.RoleInfo).Render("  • Live monitoring — observed peer endpoints while a monitor session runs"),
		g.styles.Role(theme.RoleInfo).Render("  • Dataset / packet import — ingest observations that carry src/dst IPs"),
		"",
		g.styles.Muted.Render("GeoIP City DB + world geometry are installed and ready; once observations"),
		g.styles.Muted.Render("exist they resolve to country/city pins on the map automatically."),
	}
	return components.Panel(g.styles, "GEO MAP — AWAITING NETWORK OBSERVATIONS",
		strings.Join(lines, "\n"), f)
}

// View renders the honesty note, the degrade notices (when assets are missing),
// and either the plotted map + detail panel or the density-list fallback.
func (g *GeoMap) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(g.styles.Title.Render("GEO MAP"))
	b.WriteString("\n")
	if g.geoNote != "" {
		b.WriteString(g.styles.Role(theme.RoleWarning).Render(g.geoNote))
		b.WriteString("\n")
	}
	if g.mapNote != "" {
		b.WriteString(g.styles.Role(theme.RoleWarning).Render(g.mapNote))
		b.WriteString("\n")
	}
	used := strings.Count(b.String(), "\n")
	bodyH := f.H - used
	if bodyH < 1 {
		bodyH = 1
	}
	// Explicit, actionable empty state: with no network observations there is
	// nothing to plot, and that is EXPECTED for a case acquired from a block
	// explorer (Esplora serves transactions, never peer IPs). Say so and name
	// the two real ways to get geo data, instead of a bare "0 bins" map.
	if g.gotObs && g.obsCount == 0 {
		b.WriteString(g.emptyBody(components.Frame{W: f.W, H: bodyH}))
		return clampBlockLocal(b.String(), f)
	}
	// When a point is selected, split the body: map on the left, detail on the
	// right. Otherwise the map (or fallback list) fills the width.
	if _, ok := g.view.Selected(); ok && f.W >= 100 {
		mapW := f.W * 62 / 100
		detW := f.W - mapW
		left := g.view.View(components.Frame{W: mapW, H: bodyH})
		right := components.Panel(g.styles, "POINT DETAIL",
			g.detail.View(components.Frame{W: detW - 2, H: bodyH - 3}),
			components.Frame{W: detW, H: bodyH})
		b.WriteString(components.Row2(components.Frame{W: f.W, H: bodyH}, 0.62, left, right))
	} else {
		b.WriteString(g.view.View(components.Frame{W: f.W, H: bodyH}))
	}
	return clampBlockLocal(b.String(), f)
}

// ShortHelp lists Geo Map's context keys. Enter opens the selection detail
// panel (no navigation) and g performs the cross-link "go to related tx/IP";
// the viewport's pan/zoom/n/N/0 keys come from the MapView (design §D.2).
func (g *GeoMap) ShortHelp() []key.Binding {
	out := []key.Binding{
		key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "bin country/ASN")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail")),
		key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "go to related tx/IP")),
	}
	return append(out, g.view.ShortHelp()...)
}

// loadMapGeometry loads the installed world-geometry asset, returning nil when
// it is absent or fails integrity (the renderer then degrades to the bin list).
func loadMapGeometry() *mapdata.Geometry {
	dir, err := mapdata.Dir()
	if err != nil {
		return nil
	}
	g, _, err := mapdata.Load(dir)
	if err != nil {
		return nil
	}
	return g
}

// labelOrDash returns s or an em-dash placeholder for an empty value.
func labelOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// countryLabel renders "Name (ISO)" from a resolved location, honestly empty
// when the DB had no country.
func countryLabel(l geoenrich.IPLocation) string {
	switch {
	case l.Country != "" && l.ISOCode != "":
		return fmt.Sprintf("%s (%s)", l.Country, l.ISOCode)
	case l.ISOCode != "":
		return l.ISOCode
	default:
		return l.Country
	}
}

// cityLabel renders "City, Subdivision" when present.
func cityLabel(l geoenrich.IPLocation) string {
	switch {
	case l.City != "" && l.Subdivision != "":
		return l.City + ", " + l.Subdivision
	case l.City != "":
		return l.City
	default:
		return l.Subdivision
	}
}

// joinShort joins up to n short ids with a trailing "+k more".
func joinShort(ids []string, n int) string {
	if len(ids) <= n {
		parts := make([]string, len(ids))
		for i, id := range ids {
			parts[i] = shortID(id)
		}
		return strings.Join(parts, ", ")
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = shortID(ids[i])
	}
	return strings.Join(parts, ", ") + fmt.Sprintf(" +%d more", len(ids)-n)
}

// geoDBNote returns the documented GeoIP install message when no DB is present,
// else "". Never fabricates; the geoip package is NETWORK-FORBIDDEN.
func geoDBNote() string {
	dir, err := geoip.Dir()
	if err != nil {
		return geoip.NotInstalledMessage
	}
	reg, err := geoip.LoadRegistry(dir)
	if err != nil || len(reg.Entries) == 0 {
		return geoip.NotInstalledMessage
	}
	return ""
}

// mapAssetNote returns the documented world-geometry install/degrade message
// when the asset is missing or checksum-mismatched, else "".
func mapAssetNote() string {
	dir, err := mapdata.Dir()
	if err != nil {
		return mapdata.NotInstalledMessage
	}
	if _, err := mapdata.VerifyInstalled(dir); err != nil {
		return mapdata.NotInstalledMessage
	}
	return ""
}
