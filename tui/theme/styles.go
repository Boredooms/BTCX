package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles is a set of ready-to-use lipgloss styles derived from an active Theme.
// Components call Build(theme) once and render through the returned builders;
// they never assemble a style from a raw color. This is the only sanctioned
// bridge between semantic Roles and lipgloss.
type Styles struct {
	theme Theme

	Panel        lipgloss.Style // bordered container
	PanelFocused lipgloss.Style // bordered container, active-border color (focused region)
	Title        lipgloss.Style // panel/screen heading
	Label        lipgloss.Style // dim key label
	Value        lipgloss.Style // bright value
	Selected     lipgloss.Style // highlighted selection row
	Muted        lipgloss.Style // de-emphasized text
}

// Build returns the style set for a theme. Panel uses the theme's single border
// and compact padding tokens; the degraded profile already carries an ASCII
// border, so no branching on Degraded is needed here.
func Build(t Theme) Styles {
	panel := lipgloss.NewStyle().
		Border(t.Border).
		BorderForeground(t.Color(RoleMuted)).
		Padding(Space.PanelPadY, Space.PanelPadX)
	return Styles{
		theme: t,
		Panel: panel,
		// PanelFocused is the same box re-coloured to the active-border token so
		// the shell can draw the focused region's border in RoleSelected cyan
		// without constructing a color literal (design §A.3).
		PanelFocused: panel.BorderForeground(t.Color(RoleSelected)),
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Color(RoleTitle)),
		Label: lipgloss.NewStyle().
			Foreground(t.Color(RoleLabel)),
		Value: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Color(RoleValue)),
		// Selected draws dark INVERSE text on the cyan highlight fill. The
		// previous foreground was RoleTitle (cyan) over RoleSelected
		// (BorderActive cyan) — the same color — so a selected+focused row's
		// label rendered cyan-on-cyan and vanished (the SideNav "6" tile that
		// hid its "Graph" label). RoleInverse (TextInverse) is the only
		// readable foreground against this fill.
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Color(RoleInverse)).
			Background(t.Color(RoleSelected)),
		Muted: lipgloss.NewStyle().
			Foreground(t.Color(RoleMuted)),
	}
}

// Theme exposes the underlying theme for callers that need role lookups.
func (s Styles) Theme() Theme { return s.theme }

// Role returns a plain foreground style for an arbitrary semantic role.
func (s Styles) Role(r Role) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(s.theme.Color(r))
}

// SeverityRole maps a textual severity to a semantic Role (presentation only;
// it invents no forensic meaning). Unknown severities render Neutral.
func SeverityRole(severity string) Role {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "HIGH", "CRITICAL":
		return RoleCritical
	case "MED", "MEDIUM", "WARNING", "ELEVATED":
		return RoleWarning
	case "LOW", "OK", "HEALTHY", "CONFIRMED":
		return RoleHealthy
	default:
		return RoleNeutral
	}
}

// SeverityChip renders a short, padded severity badge using the severity's
// semantic role. The glyph/label text is caller-supplied; coloring is enforced
// here so no component hard-codes a severity color.
func (s Styles) SeverityChip(severity, label string) string {
	role := SeverityRole(severity)
	chip := lipgloss.NewStyle().
		Bold(true).
		Foreground(s.theme.Color(role))
	return chip.Render(label)
}

// Badge renders a bracketed status badge in a given role (e.g. a graph limit
// badge or a LIVE marker). Content is caller-supplied.
func (s Styles) Badge(r Role, text string) string {
	return lipgloss.NewStyle().
		Foreground(s.theme.Color(r)).
		Render(text)
}
