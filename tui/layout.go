// Package tui is the BCTX presentation + interaction layer. It is strictly a
// front end over the shared app/ seam and the existing BCTX services: it holds
// no canonical forensic state and runs no SQL/HTTP/analysis of its own (design
// §0, AGENTS §12). This file owns responsive geometry only.
package tui

import (
	"github.com/bctx/bctx/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// Breakpoint classifies the terminal width into a layout class. The ONLY source
// of size is tea.WindowSizeMsg (design §6); nothing reads os.Stdin or env.
type Breakpoint int

const (
	// BreakpointCompact is < 100 columns: SideNav collapses, single workspace pane.
	BreakpointCompact Breakpoint = iota
	// BreakpointStandard is 100-159 columns: labeled SideNav, SplitView allowed.
	BreakpointStandard
	// BreakpointWide is 160-199 columns: SplitView plus an optional detail rail.
	BreakpointWide
	// BreakpointUltrawide is >= 200 columns: body = two columns (list | detail);
	// nav is the shell column (outside the Workspace frame, which is
	// width-navW). The body therefore reflows into at most two columns at every
	// breakpoint; the sidebar is a shell concern, never a body column (§C.3).
	BreakpointUltrawide
)

// Layout thresholds (inclusive lower bounds) per design §6.
const (
	widthStandard  = 100
	widthWide      = 160
	widthUltrawide = 200

	// MinWidth/MinHeight is the absolute floor below which the shell shows only
	// the "terminal too small" message and renders nothing else (design §6).
	MinWidth  = 80
	MinHeight = 24
)

// String renders the breakpoint name (used in help/diagnostics).
func (b Breakpoint) String() string {
	switch b {
	case BreakpointCompact:
		return "compact"
	case BreakpointStandard:
		return "standard"
	case BreakpointWide:
		return "wide"
	case BreakpointUltrawide:
		return "ultrawide"
	default:
		return "unknown"
	}
}

// ClassifyWidth maps a terminal width to its Breakpoint.
func ClassifyWidth(w int) Breakpoint {
	switch {
	case w >= widthUltrawide:
		return BreakpointUltrawide
	case w >= widthWide:
		return BreakpointWide
	case w >= widthStandard:
		return BreakpointStandard
	default:
		return BreakpointCompact
	}
}

// Classify maps a WindowSizeMsg to its Breakpoint (width-driven per §6).
func Classify(msg tea.WindowSizeMsg) Breakpoint { return ClassifyWidth(msg.Width) }

// NavWidth is the SideNav column width for a breakpoint. Compact hides the
// labeled nav (a one-column rail is handled by the SideNav component itself),
// so its reserved width is 0 here and the body gets the full width.
func (b Breakpoint) NavWidth() int {
	switch b {
	case BreakpointCompact:
		return 0
	case BreakpointStandard:
		return 20
	case BreakpointWide:
		return 24
	case BreakpointUltrawide:
		return 28
	default:
		return 20
	}
}

// Frame is an allocated rectangle in terminal cells. It is defined in the
// components package (the leaf that every component renders into) and aliased
// here so tui and components share one type with no import cycle. The shell
// computes Frames and passes them down; every component clamps/truncates to its
// Frame and must never overflow it (design §6).
type Frame = components.Frame

// Layout is the computed region geometry for one render pass, derived from the
// window size. TopBar and ContextBar are single-row by default; the body is
// everything in between, split into the SideNav column and the workspace.
type Layout struct {
	Breakpoint Breakpoint
	// TooSmall is true when the terminal is below the 80x24 floor; when set,
	// only Message should be rendered (centered) and all region Frames are empty.
	TooSmall bool
	Message  string

	TopBar     Frame
	SideNav    Frame
	Workspace  Frame
	ContextBar Frame
}

const tooSmallMessage = "Terminal too small — resize to at least 80x24"

// compactRailWidth is the one-column nav rail shown in Compact when the sidebar
// toggle (navForced) overrides the auto-hide (design §C.3).
const compactRailWidth = 20

// ComputeLayout turns a window size into the region geometry for a render pass.
// Below the 80x24 floor it returns TooSmall with the centered message and empty
// region frames so the shell renders nothing else (design §6).
func ComputeLayout(width, height int) Layout {
	return ComputeLayoutNav(width, height, false)
}

// ComputeLayoutNav is ComputeLayout with the sidebar toggle (design §C.3). When
// navForced is true and the breakpoint would otherwise hide the labeled nav
// (Compact, navW==0), a narrow nav rail is reserved instead, so the reserved
// navW and the rendered rail width agree in one place (NIT-2). At every other
// breakpoint navForced is a no-op (the labeled nav is already shown).
func ComputeLayoutNav(width, height int, navForced bool) Layout {
	if width < MinWidth || height < MinHeight {
		return Layout{
			Breakpoint: ClassifyWidth(width),
			TooSmall:   true,
			Message:    tooSmallMessage,
		}
	}

	bp := ClassifyWidth(width)
	navW := bp.NavWidth()
	if navForced && navW == 0 {
		navW = compactRailWidth
	}

	const topH = 2 // wordmark/status row + breadcrumb row
	const ctxH = 1 // single context/status band

	// The shell inserts ONE blank spacer row between the top chrome and the body
	// (AppShell.View gapRows) whenever there is room for it plus a body row.
	// The Workspace height MUST reserve that same row, otherwise the shell gives
	// the screen a frame one row taller than it actually displays and then
	// clamps the body — dropping the screen's LAST row (its bottom panel border,
	// the "chop"). Keeping this in lockstep with AppShell.View is the single
	// source of the body geometry (NIT-2 / design §C).
	gapH := 0
	if height-topH-ctxH >= 2 {
		gapH = 1
	}

	bodyY := topH + gapH
	bodyH := height - topH - ctxH - gapH
	if bodyH < 0 {
		bodyH = 0
	}

	workspaceX := navW
	workspaceW := width - navW
	if workspaceW < 0 {
		workspaceW = 0
	}

	return Layout{
		Breakpoint: bp,
		TopBar:     Frame{X: 0, Y: 0, W: width, H: topH},
		SideNav:    Frame{X: 0, Y: bodyY, W: navW, H: bodyH},
		Workspace:  Frame{X: workspaceX, Y: bodyY, W: workspaceW, H: bodyH},
		ContextBar: Frame{X: 0, Y: bodyY + bodyH, W: width, H: ctxH},
	}
}
