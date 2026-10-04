package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// AppShell is the frame: it lays out the TopBar, SideNav, workspace, and
// ContextBar for the current window size and hosts the body (the active
// screen's render) in the workspace region. It owns the 80x24 too-small floor
// (design §6): below it, nothing is rendered but the centered resize message.
//
// AppShell holds no screen logic; the root supplies the already-rendered body
// string via ShellFrame so the shell stays a pure presentation frame.
type AppShell struct {
	styles  theme.Styles
	topBar  TopBar
	nav     SideNav
	context ContextBar
}

// NewAppShell builds the shell with its chrome sub-components.
func NewAppShell(styles theme.Styles) AppShell {
	return AppShell{
		styles:  styles,
		topBar:  NewTopBar(styles),
		nav:     NewSideNav(styles),
		context: NewContextBar(styles),
	}
}

// ShellRegion names which region the shell should draw as focused (design §A.3).
// It is a presentation enum mirrored from the Root's FocusRegion so the leaf
// shell can colour the active border without importing tui.
type ShellRegion int

const (
	// ShellFocusNone draws every region muted (no active border).
	ShellFocusNone ShellRegion = iota
	// ShellFocusNav draws the SideNav column as the active region.
	ShellFocusNav
	// ShellFocusBody draws the workspace as the active region.
	ShellFocusBody
	// ShellFocusModal keeps both regions muted while an overlay owns the keyboard.
	ShellFocusModal
)

// ShellFrame is the per-render input the root hands the shell: the window size,
// the active screen title (for the breadcrumb), the already-rendered body, and
// the focus/overlay state so the shell can draw the active region and host an
// overlay. NavWidth is the authoritative column width from tui.ComputeLayout
// (NIT-2): the shell no longer derives it from SideNav.Width, so the reserved
// width and the rendered rail width can never disagree.
type ShellFrame struct {
	Width  int
	Height int
	Title  string
	Body   string

	// NavWidth is the SideNav column width chosen by ComputeLayout (0 hides it).
	NavWidth int
	// NavCursor/NavFocused drive the interactive SideNav selection cursor.
	NavCursor  int
	NavFocused bool
	// Focused names the region the shell draws with the active border.
	Focused ShellRegion
	// Overlay, when non-empty, is a pre-rendered overlay centered over the body
	// (command palette / search). Hints is a focus-state label for the context
	// bar (NAV/BODY/overlay name).
	Overlay string
	Hints   string
}

// Layout thresholds mirrored from the tui layout package. They are duplicated
// here (not imported) because components is a leaf package that tui imports;
// the single source for the real geometry is tui.ComputeLayout, and the shell's
// own math stays consistent with it via these constants.
const (
	minWidth  = 80
	minHeight = 24
	topRows   = 2
	ctxRows   = 1
)

// tooSmall reports whether the window is below the supported floor.
func (s AppShell) tooSmall(w, h int) bool { return w < minWidth || h < minHeight }

// SetTopBar replaces the TopBar fields (status values from live services).
func (s *AppShell) SetTopBar(t TopBar) { s.topBar = t }

// SetNavItems replaces the SideNav entries (built by the root from the registry,
// the single source of truth for navigation).
func (s *AppShell) SetNavItems(items []NavItem) { s.nav.SetItems(items) }

// View renders the whole shell for one frame.
func (s AppShell) View(sf ShellFrame) string {
	if s.tooSmall(sf.Width, sf.Height) {
		return s.renderTooSmall(sf.Width, sf.Height)
	}

	// NavWidth is authoritative from ComputeLayout (NIT-2): the shell no longer
	// recomputes it from SideNav.Width.
	navW := sf.NavWidth
	if navW < 0 {
		navW = 0
	}
	// One spacer row between the top-bar/breadcrumb chrome and the body so a
	// screen's heading is never flush against the top bar. The spacer is only
	// added when there is room for it plus at least one body row.
	gapRows := 0
	if sf.Height-topRows-ctxRows >= 2 {
		gapRows = 1
	}
	bodyH := sf.Height - topRows - ctxRows - gapRows
	if bodyH < 0 {
		bodyH = 0
	}
	workspaceW := sf.Width - navW
	if workspaceW < 0 {
		workspaceW = 0
	}

	// Drive the interactive SideNav from the Root's cursor/focus state.
	s.nav.SetCursor(sf.NavCursor)
	s.nav.SetFocused(sf.Focused == ShellFocusNav)

	bodyY := topRows + gapRows
	top := s.topBar.View(Frame{X: 0, Y: 0, W: sf.Width, H: topRows}, sf.Title)
	navView := s.nav.View(Frame{X: 0, Y: bodyY, W: navW, H: bodyH})
	bodyView := clampBlock(sf.Body, Frame{X: navW, Y: bodyY, W: workspaceW, H: bodyH})

	// Join nav + body horizontally; pad the body region to the body height.
	middle := bodyView
	if navW > 0 {
		middle = lipgloss.JoinHorizontal(lipgloss.Top,
			padHeight(navView, bodyH, navW),
			padHeight(bodyView, bodyH, workspaceW),
		)
	}

	// Host an overlay (palette/search) centered over the body region when open.
	if sf.Overlay != "" {
		overlayH := bodyH
		middle = lipgloss.Place(sf.Width, overlayH, lipgloss.Center, lipgloss.Center, sf.Overlay)
	}

	s.context.Hints = sf.Hints
	s.context.Focus = focusLabel(sf)
	ctx := s.context.View(Frame{X: 0, Y: bodyY + bodyH, W: sf.Width, H: ctxRows})

	// Assemble: top bar, a spacer row (breathing room under the chrome), the
	// nav+body middle, then the context bar.
	parts := []string{top}
	if gapRows > 0 {
		parts = append(parts, "")
	}
	parts = append(parts, middle, ctx)
	out := strings.Join(parts, "\n")
	// Final guard: never exceed the terminal rectangle.
	return clampBlock(out, Frame{W: sf.Width, H: sf.Height})
}

// focusLabel is the text focus-state segment for the ContextBar: NAV / BODY /
// the overlay name while a modal owns the keyboard (design §A.3).
func focusLabel(sf ShellFrame) string {
	if sf.Overlay != "" || sf.Focused == ShellFocusModal {
		return "OVERLAY"
	}
	switch sf.Focused {
	case ShellFocusNav:
		return "NAV"
	case ShellFocusBody:
		return "BODY"
	default:
		return ""
	}
}

// renderTooSmall centers the resize message and renders nothing else (§6).
func (s AppShell) renderTooSmall(w, h int) string {
	msg := "Terminal too small — resize to at least 80x24"
	msg = clampLine(msg, max(w, 1))
	style := lipgloss.NewStyle().Foreground(s.styles.Theme().Color(theme.RoleWarning))
	if w <= 0 || h <= 0 {
		return style.Render(msg)
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, style.Render(msg))
}

// padHeight pads a block to exactly h rows and clamps lines to w cells so the
// horizontal join lines up without overflow.
func padHeight(s string, h, w int) string {
	lines := splitLines(s)
	for i := range lines {
		lines[i] = clampLine(lines[i], w)
		// right-pad to the column width so the next column starts aligned.
		if gap := w - lipgloss.Width(lines[i]); gap > 0 {
			lines[i] += strings.Repeat(" ", gap)
		}
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return joinLines(lines)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
