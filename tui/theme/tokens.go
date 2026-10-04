// Package theme owns every color, spacing, and border decision in the BCTX
// TUI. Views and components consume semantic tokens (theme.Role) and style
// builders (styles.go); they NEVER construct a lipgloss.Color/AdaptiveColor
// themselves. This is the single place a lipgloss color literal may appear
// (design §7, acceptance §19.5), which keeps the BCTX identity consistent and
// lets 256/16-color terminals downgrade automatically.
//
// Identity: a cyan/teal information language on a dark workspace — deliberately
// distinct from the reference image's orange branding and from the previous
// inline bitcoin-orange styling that lived in tui/screens/home.go.
package theme

import "github.com/charmbracelet/lipgloss"

// Palette is the raw BCTX color palette. Values carry no meaning on their own;
// theme.go maps them onto semantic Roles. Each entry is an adaptive triple
// (TrueColor hex / 256-safe hex) so lipgloss downgrades gracefully; the Dark
// column is authoritative because BCTX is a dark terminal workspace.
var Palette = struct {
	BgBase, BgPanel, BgRaised         lipgloss.AdaptiveColor
	Border, BorderActive              lipgloss.AdaptiveColor
	TextPrimary, TextDim, TextInverse lipgloss.AdaptiveColor
	Cyan, Teal                        lipgloss.AdaptiveColor // primary information language
	Green                             lipgloss.AdaptiveColor // healthy / local / confirmed
	Blue                              lipgloss.AdaptiveColor // neutral information
	Amber                             lipgloss.AdaptiveColor // warning
	Red                               lipgloss.AdaptiveColor // high-risk (restrained use)
}{
	BgBase:   lipgloss.AdaptiveColor{Light: "#F5F7FA", Dark: "#0B0F12"},
	BgPanel:  lipgloss.AdaptiveColor{Light: "#ECEFF3", Dark: "#11171C"},
	BgRaised: lipgloss.AdaptiveColor{Light: "#E2E6EB", Dark: "#182028"},

	Border:       lipgloss.AdaptiveColor{Light: "#C3CAD3", Dark: "#283139"},
	BorderActive: lipgloss.AdaptiveColor{Light: "#17B2A3", Dark: "#30D5E8"},

	TextPrimary: lipgloss.AdaptiveColor{Light: "#1B2229", Dark: "#E6EDF3"},
	TextDim:     lipgloss.AdaptiveColor{Light: "#5A6672", Dark: "#7D8A96"},
	TextInverse: lipgloss.AdaptiveColor{Light: "#F5F7FA", Dark: "#0B0F12"},

	Cyan: lipgloss.AdaptiveColor{Light: "#0E8FA0", Dark: "#30D5E8"},
	Teal: lipgloss.AdaptiveColor{Light: "#0F8F83", Dark: "#17B2A3"},

	Green: lipgloss.AdaptiveColor{Light: "#2E9E43", Dark: "#3FB950"},
	Blue:  lipgloss.AdaptiveColor{Light: "#2E6FD6", Dark: "#4C8DF6"},
	Amber: lipgloss.AdaptiveColor{Light: "#B5831A", Dark: "#E3B341"},
	Red:   lipgloss.AdaptiveColor{Light: "#C83F3F", Dark: "#F05C5C"},
}

// Space holds the compact, high-density spacing tokens used across the shell
// and components (design §7: PanelPadX=1, PanelPadY=0, Gap=1).
var Space = struct {
	PanelPadX int
	PanelPadY int
	Gap       int
}{
	PanelPadX: 1,
	PanelPadY: 0,
	Gap:       1,
}
