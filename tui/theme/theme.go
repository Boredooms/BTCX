package theme

import "github.com/charmbracelet/lipgloss"

// Role is a semantic color slot. Views reference Roles, never raw palette
// entries, so meaning (not color) is what the UI code expresses (design §7).
type Role string

const (
	RoleTitle    Role = "title"    // headings, wordmark
	RoleLabel    Role = "label"    // dim metadata labels
	RoleValue    Role = "value"    // bright primary data
	RoleInfo     Role = "info"     // neutral information (cyan/teal)
	RoleHealthy  Role = "healthy"  // healthy / local / confirmed (green)
	RoleWarning  Role = "warning"  // warning (amber)
	RoleCritical Role = "critical" // high-risk / error (red, restrained)
	RoleNeutral  Role = "neutral"  // neutral emphasis (cyan)
	RoleSelected Role = "selected" // active selection highlight
	RoleInverse  Role = "inverse"  // text drawn ON a highlight fill (reads on RoleSelected)
	RoleMuted    Role = "muted"    // de-emphasized / disabled
)

// Theme is the resolved semantic theme handed down from the root to every
// component. It is immutable once built; a degraded profile is a separate
// Theme value (Degraded()), never a mutation of Default().
type Theme struct {
	// Role maps each semantic Role to an adaptive color.
	Role map[Role]lipgloss.AdaptiveColor
	// Border is the single, consistent border used by every panel.
	Border lipgloss.Border
	// Degraded is true for the ASCII / 16-color-safe profile.
	Degraded bool
}

// Color returns the adaptive color for a role, falling back to the primary
// text color for any unmapped role so a lookup never yields the zero value.
func (t Theme) Color(r Role) lipgloss.AdaptiveColor {
	if c, ok := t.Role[r]; ok {
		return c
	}
	return Palette.TextPrimary
}

func baseRoles() map[Role]lipgloss.AdaptiveColor {
	return map[Role]lipgloss.AdaptiveColor{
		RoleTitle:    Palette.Cyan,
		RoleLabel:    Palette.TextDim,
		RoleValue:    Palette.TextPrimary,
		RoleInfo:     Palette.Teal,
		RoleHealthy:  Palette.Green,
		RoleWarning:  Palette.Amber,
		RoleCritical: Palette.Red,
		RoleNeutral:  Palette.Cyan,
		RoleSelected: Palette.BorderActive,
		RoleInverse:  Palette.TextInverse,
		RoleMuted:    Palette.TextDim,
	}
}

// Default returns the standard BCTX dark identity with rounded Unicode borders.
func Default() Theme {
	return Theme{
		Role:     baseRoles(),
		Border:   lipgloss.RoundedBorder(),
		Degraded: false,
	}
}

// Degraded returns the ASCII / 16-color-safe profile selected when the terminal
// reports no color or a NO_COLOR / limited TERM capability (design §6). It uses
// ASCII box borders and the same semantic Role set so callers need no special
// casing — only the border glyphs and the lipgloss color profile differ.
func Degraded() Theme {
	return Theme{
		Role:     baseRoles(),
		Border:   lipgloss.ASCIIBorder(),
		Degraded: true,
	}
}
