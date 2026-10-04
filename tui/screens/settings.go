package screens

import (
	"context"
	"strings"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/redact"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Settings is the Settings screen (design §2). It is read-mostly: it shows the
// active configuration (network mode, provider, models dir), the case list, and
// the geoip/map registry status. It displays NO secrets (AGENTS §21) — only
// non-sensitive config fields and public dataset provenance.
type Settings struct {
	ctx    *ScreenCtx
	styles theme.Styles

	detail components.DetailPanel
}

// NewSettings builds the Settings screen.
func NewSettings(ctx *ScreenCtx) *Settings {
	s := &Settings{ctx: ctx, styles: ctx.styles(), detail: components.NewDetailPanel(ctx.styles())}
	s.detail.SetSections(s.sections())
	return s
}

// Init is a no-op: Settings reads already-available config/registry state.
func (s *Settings) Init() tea.Cmd { return nil }

// Update forwards scroll keys to the detail panel.
func (s *Settings) Update(msg tea.Msg) (Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		var cmd tea.Cmd
		s.detail, cmd = s.detail.Update(km)
		return s, cmd
	}
	return s, nil
}

func (s *Settings) sections() []components.Section {
	cfg := s.ctx.Cfg
	cfgRows := []components.Row{
		{Label: "network mode", Value: networkMode(cfg)},
		{Label: "acquisition", Value: acquisitionState(cfg)},
		{Label: "provider", Value: provider(cfg)},
		{Label: "models dir", Value: modelsDir(cfg)},
		{Label: "models", Value: ModelsStatus(cfg)},
		{Label: "geoip", Value: GeoIPStatus()},
	}
	secs := []components.Section{
		{Title: "configuration (read-mostly, no secrets)", Rows: cfgRows},
		{Title: "risk bands", Rows: []components.Row{
			{Label: "LOW", Value: "0-24 (green)", Role: theme.RoleHealthy},
			{Label: "ELEVATED", Value: "25-49 (cyan)", Role: theme.RoleNeutral},
			{Label: "HIGH", Value: "50-74 (amber)", Role: theme.RoleWarning},
			{Label: "CRITICAL", Value: "75-100 (red)", Role: theme.RoleCritical},
		}},
	}
	secs = append(secs, s.casesSection())
	return secs
}

func (s *Settings) casesSection() components.Section {
	rows := []components.Row{}
	if s.ctx.App != nil && s.ctx.App.Cases != nil {
		cs, err := s.ctx.App.Cases.List(context.Background())
		if err != nil {
			rows = append(rows, components.Row{Label: "cases", Value: "error: " + err.Error()})
		} else if len(cs) == 0 {
			rows = append(rows, components.Row{Label: "cases", Value: "none — create with 'bctx case create'"})
		} else {
			active := s.ctx.caseID()
			for _, c := range cs {
				val := string(c.Status)
				if c.ID == active {
					val += " (active)"
				}
				rows = append(rows, components.Row{Label: c.ID, Value: val})
			}
		}
	} else {
		rows = append(rows, components.Row{Label: "cases", Value: "unavailable"})
	}
	return components.Section{Title: "cases", Rows: rows}
}

// View renders the settings detail.
func (s *Settings) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	title := s.styles.Title.Render("SETTINGS")
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}
	cw, ch := components.PanelInner(components.Frame{W: f.W, H: bodyH})
	panel := components.Panel(s.styles, "CONFIGURATION & CASES",
		s.detail.View(components.Frame{W: cw, H: ch - 1}),
		components.Frame{W: f.W, H: bodyH})
	return clampBlockLocal(title+"\n"+panel, f)
}

// ShortHelp lists Settings' context keys.
func (s *Settings) ShortHelp() []key.Binding { return noBinding }

// --- config presentation helpers (no secrets) ------------------------------

func networkMode(cfg *configs.Config) string {
	if cfg == nil {
		return "unknown"
	}
	return string(cfg.Network.Mode)
}

func acquisitionState(cfg *configs.Config) string {
	if cfg == nil {
		return "unknown"
	}
	if cfg.AcquisitionAllowed() {
		return "ENABLED"
	}
	switch cfg.Network.Mode {
	case configs.ModeAirgap:
		return "AIRGAPPED"
	case configs.ModeOffline:
		return "OFFLINE"
	default:
		return "PAUSED"
	}
}

func provider(cfg *configs.Config) string {
	if cfg == nil || cfg.Acquisition.Provider == "" {
		return "(none)"
	}
	// Provider is normally a short name, but it is user-controlled config and a
	// careless value could embed a token-bearing endpoint; scrub it before
	// display (design §12 invariant 5; AGENTS §21).
	return redact.String(cfg.Acquisition.Provider)
}

var _ = strings.TrimSpace
