package screens

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/geoenrich"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Network is the Network / IP screen (design §2). It lists network observations
// for the selected IP (NetworkObservationsByIP) or, when no IP subject is set,
// a bounded sample of all observations (AllNetworkObservations). Observations
// are telemetry metadata only — country/ASN are shown as observed metadata, not
// ownership (AGENTS §18).
type Network struct {
	ctx    *ScreenCtx
	styles theme.Styles

	ip    string
	table components.Table
	count int
}

const networkObsLimit = 500

// NewNetwork builds the Network screen.
func NewNetwork(ctx *ScreenCtx) *Network {
	cols := []table.Column{
		{Title: "when", Width: 16},
		{Title: "src", Width: 18},
		{Title: "dst", Width: 18},
		{Title: "country*", Width: 10},
		{Title: "city*", Width: 16},
		{Title: "asn*", Width: 12},
		{Title: "txid", Width: 12},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	ip := ""
	if ctx.Subject.Kind == SubjectIP {
		ip = ctx.Subject.ID
	}
	tbl.SetEmptyText("no network observations — block-explorer acquisition carries no peer IPs; " +
		"populate via live monitoring or a dataset/packet import")
	return &Network{ctx: ctx, styles: ctx.styles(), ip: ip, table: tbl}
}

// Init loads observations for the subject IP, or a bounded sample otherwise.
func (n *Network) Init() tea.Cmd {
	repo := n.ctx.repo()
	if repo == nil {
		n.table.SetError(errNoCase.Error())
		return nil
	}
	n.table.SetLoading()
	if n.ip != "" {
		return observationsByIPCmd(n.ctx.bgCtx(), repo, n.ip)
	}
	return allObservationsCmd(n.ctx.bgCtx(), repo, networkObsLimit)
}

// Update folds in observations.
func (n *Network) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case dataLoaded:
		if obs, ok := m.Payload.([]schema.NetworkObservation); ok {
			n.setObs(obs)
		}
	case dataError:
		n.table.SetError(m.Err.Error())
	case tea.KeyMsg:
		var cmd tea.Cmd
		n.table, cmd = n.table.Update(m)
		return n, cmd
	}
	return n, nil
}

func (n *Network) setObs(obs []schema.NetworkObservation) {
	n.count = len(obs)
	// Resolve each distinct SrcIP through the installed GeoIP DB (offline,
	// cached per render) so the table shows DB-resolved country/city/ASN. The
	// trailing * on the headers plus the honest wording below mark these as IP
	// geolocation metadata (estimate), never ownership (AGENTS §18).
	byIP := map[string]geoenrich.IPLocation{}
	for _, l := range geoenrich.Enrich(obs, 0).Locations {
		byIP[l.IP] = l
	}
	rows := make([]table.Row, 0, len(obs))
	ids := make([]string, 0, len(obs))
	kinds := make([]string, 0, len(obs))
	for _, o := range obs {
		loc := byIP[o.SrcIP]
		country := firstNonEmpty(countryCell(loc), o.Country, "—")
		city := firstNonEmpty(loc.City, "—")
		asn := firstNonEmpty(loc.ASN, o.ASN, "—")
		rows = append(rows, table.Row{
			o.Timestamp.Format("2006-01-02 15:04"),
			clampLineLocal(o.SrcIP, 18),
			clampLineLocal(o.DstIP, 18),
			clampLineLocal(country, 10),
			clampLineLocal(city, 16),
			clampLineLocal(asn, 12),
			shortID(o.TxID),
		})
		ids = append(ids, o.SrcIP)
		kinds = append(kinds, "ip")
	}
	n.table.SetRows(rows, ids, kinds)
}

// countryCell renders a resolved location's country as "ISO" (compact), falling
// back to empty so the caller can use the observation's own label.
func countryCell(l geoenrich.IPLocation) string {
	if l.ISOCode != "" {
		return l.ISOCode
	}
	return ""
}

// firstNonEmpty returns the first non-blank argument, or the last argument as a
// final fallback.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	if len(vals) > 0 {
		return vals[len(vals)-1]
	}
	return ""
}

// View renders the observations table.
func (n *Network) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	scope := "all observations (sample)"
	if n.ip != "" {
		scope = "IP " + n.ip
	}
	title := n.styles.Title.Render("NETWORK / IP") + " " +
		n.styles.Label.Render(fmt.Sprintf("%s · %d records", scope, n.count))
	note := n.styles.Role(theme.RoleWarning).Render(
		"* country/city/ASN are IP geolocation metadata (estimate), not proof of ownership")
	bodyH := f.H - 2
	if bodyH < 1 {
		bodyH = 1
	}
	cw, ch := components.PanelInner(components.Frame{W: f.W, H: bodyH})
	panel := components.Panel(n.styles, "NETWORK OBSERVATIONS",
		n.table.View(components.Frame{W: cw, H: ch - 1}),
		components.Frame{W: f.W, H: bodyH})
	return clampBlockLocal(title+"\n"+note+"\n"+panel, f)
}

// ShortHelp lists Network's context keys.
func (n *Network) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "inspect IP")),
	}
}
