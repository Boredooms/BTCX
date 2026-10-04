package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// TopBar is the persistent status line (design §8). Every field reflects real
// service state supplied by the root — the TopBar never computes or fakes a
// value. In compact widths it sheds the least-critical fields first (clock,
// provider), keeping CASE / NETWORK / ACQUISITION / MODELS.
type TopBar struct {
	styles theme.Styles

	// Fields are plain presentation strings set by the root from live services.
	Case        string
	Network     string // CONNECTED / DISCONNECTED / AIRGAPPED
	Acquisition string // ENABLED / PAUSED / OFFLINE / AIRGAPPED
	Provider    string
	Models      string // LOADED / MISSING / SCHEMA-MISMATCH
	DB          string // LOCAL / NO CASE
	GeoIP       string
	Subject     string
	Clock       string
}

// NewTopBar builds an empty TopBar bound to the active styles.
func NewTopBar(styles theme.Styles) TopBar { return TopBar{styles: styles} }

// field is one labeled status cell with a priority (lower sheds first).
type field struct {
	label    string
	value    string
	role     theme.Role
	priority int // 0 = shed first in compact
}

// networkRole maps the UPPERCASE network status constant to a semantic role
// (healthy connected, warning disconnected, neutral airgapped) — comparisons
// use the constants, never a lowercase literal (design §8).
func networkRole(status string) theme.Role {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "CONNECTED":
		return theme.RoleHealthy
	case "DISCONNECTED":
		return theme.RoleWarning
	case "AIRGAPPED":
		return theme.RoleNeutral
	default:
		return theme.RoleMuted
	}
}

func acqRole(acq string) theme.Role {
	switch strings.ToUpper(strings.TrimSpace(acq)) {
	case "ENABLED":
		return theme.RoleHealthy
	default:
		return theme.RoleWarning
	}
}

func modelsRole(m string) theme.Role {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "LOADED":
		return theme.RoleHealthy
	case "MISSING":
		return theme.RoleWarning
	case "SCHEMA-MISMATCH":
		return theme.RoleCritical
	default:
		return theme.RoleMuted
	}
}

// View renders the TopBar into its frame. Title is the active screen title for
// the breadcrumb row.
func (t TopBar) View(f Frame, title string) string {
	if f.Empty() {
		return ""
	}

	// Ordered by priority: higher survives longer under compaction.
	fields := []field{
		{"", "BCTX", theme.RoleTitle, 100},
		{"CASE", orNone(t.Case), theme.RoleValue, 90},
		{"NET", orNone(t.Network), networkRole(t.Network), 80},
		{"ACQ", orNone(t.Acquisition), acqRole(t.Acquisition), 70},
		{"MODELS", orNone(t.Models), modelsRole(t.Models), 60},
		{"DB", orNone(t.DB), theme.RoleInfo, 50},
		{"PROV", t.Provider, theme.RoleLabel, 30},
		{"GEOIP", t.GeoIP, theme.RoleLabel, 25},
		{"TIME", t.Clock, theme.RoleLabel, 10},
	}

	// Render candidate cells, then shed lowest priority until it fits.
	rendered := t.renderFields(fields, f.W)
	first := clampLine(rendered, f.W)

	// Breadcrumb row: Screen ▸ Subject.
	crumb := t.styles.Role(theme.RoleTitle).Render(orNone(title))
	if strings.TrimSpace(t.Subject) != "" {
		crumb += t.styles.Role(theme.RoleLabel).Render(" ▸ ") +
			t.styles.Role(theme.RoleValue).Render(t.Subject)
	}
	second := clampLine(crumb, f.W)

	rows := []string{first}
	if f.H >= 2 {
		rows = append(rows, second)
	}
	return clampBlock(joinLines(rows), f)
}

// renderFields drops the lowest-priority fields until the line fits in width w.
func (t TopBar) renderFields(fields []field, w int) string {
	cur := make([]field, len(fields))
	copy(cur, fields)
	for {
		line := t.join(cur)
		if lipgloss.Width(line) <= w || len(cur) <= 1 {
			return line
		}
		// find lowest-priority index and drop it.
		lowest := 0
		for i := 1; i < len(cur); i++ {
			if cur[i].priority < cur[lowest].priority {
				lowest = i
			}
		}
		cur = append(cur[:lowest], cur[lowest+1:]...)
	}
}

func (t TopBar) join(fields []field) string {
	parts := make([]string, 0, len(fields))
	for _, fl := range fields {
		if strings.TrimSpace(fl.value) == "" {
			continue
		}
		cell := ""
		if fl.label != "" {
			cell += t.styles.Role(theme.RoleLabel).Render(fl.label + " ")
		}
		cell += t.styles.Role(fl.role).Render(fl.value)
		parts = append(parts, cell)
	}
	return strings.Join(parts, t.styles.Role(theme.RoleMuted).Render(" · "))
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(none)"
	}
	return s
}
