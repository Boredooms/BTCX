package tui

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
)

func TestClassifyWidthBreakpoints(t *testing.T) {
	cases := []struct {
		w    int
		want Breakpoint
	}{
		{0, BreakpointCompact},
		{80, BreakpointCompact},
		{99, BreakpointCompact},
		{100, BreakpointStandard},
		{159, BreakpointStandard},
		{160, BreakpointWide},
		{199, BreakpointWide},
		{200, BreakpointUltrawide},
		{400, BreakpointUltrawide},
	}
	for _, c := range cases {
		if got := ClassifyWidth(c.w); got != c.want {
			t.Errorf("ClassifyWidth(%d) = %v, want %v", c.w, got, c.want)
		}
	}
}

func TestNavWidthPerBreakpoint(t *testing.T) {
	cases := map[Breakpoint]int{
		BreakpointCompact:   0,
		BreakpointStandard:  20,
		BreakpointWide:      24,
		BreakpointUltrawide: 28,
	}
	for bp, want := range cases {
		if got := bp.NavWidth(); got != want {
			t.Errorf("%v.NavWidth() = %d, want %d", bp, got, want)
		}
	}
}

func TestComputeLayoutTooSmallFloor(t *testing.T) {
	// Below either dimension floor -> TooSmall with message, empty regions.
	for _, dim := range [][2]int{{79, 24}, {80, 23}, {40, 10}, {0, 0}} {
		lay := ComputeLayout(dim[0], dim[1])
		if !lay.TooSmall {
			t.Errorf("ComputeLayout(%d,%d) should be TooSmall", dim[0], dim[1])
		}
		if lay.Message == "" {
			t.Errorf("ComputeLayout(%d,%d) TooSmall must carry a message", dim[0], dim[1])
		}
		if !lay.Workspace.Empty() || !lay.TopBar.Empty() {
			t.Errorf("ComputeLayout(%d,%d) regions must be empty when too small", dim[0], dim[1])
		}
	}
}

func TestComputeLayoutRegionsNoOverlap(t *testing.T) {
	sizes := [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}}
	for _, s := range sizes {
		w, h := s[0], s[1]
		lay := ComputeLayout(w, h)
		if lay.TooSmall {
			t.Fatalf("ComputeLayout(%d,%d) should not be too small", w, h)
		}
		// TopBar spans full width at the top.
		if lay.TopBar.W != w || lay.TopBar.Y != 0 {
			t.Errorf("%dx%d TopBar wrong: %+v", w, h, lay.TopBar)
		}
		// SideNav + Workspace together span the full width with no overlap.
		if lay.SideNav.W+lay.Workspace.W != w {
			t.Errorf("%dx%d nav(%d)+workspace(%d) != width %d", w, h, lay.SideNav.W, lay.Workspace.W, w)
		}
		if lay.Workspace.X != lay.SideNav.W {
			t.Errorf("%dx%d workspace must start where nav ends", w, h)
		}
		// ContextBar sits below the body, within the terminal height.
		if lay.ContextBar.Y+lay.ContextBar.H > h {
			t.Errorf("%dx%d context bar overflows height", w, h)
		}
		// The body starts after the top bar PLUS the shell's one-row spacer, so
		// the context bar is directly below the body at Workspace.Y+Workspace.H.
		// (The spacer row is why this is not simply TopBar.H+Workspace.H — the
		// Workspace height already reserves that spacer so the screen renders
		// exactly what the shell displays; see ComputeLayoutNav gapH.)
		if lay.ContextBar.Y != lay.Workspace.Y+lay.Workspace.H {
			t.Errorf("%dx%d context bar not directly below body: ctxY=%d workspaceY=%d workspaceH=%d",
				w, h, lay.ContextBar.Y, lay.Workspace.Y, lay.Workspace.H)
		}
		// The whole stack (top bar + spacer + body + context bar) fits exactly
		// within the terminal height with no row unaccounted for.
		if lay.ContextBar.Y+lay.ContextBar.H != h {
			t.Errorf("%dx%d regions do not fill height exactly: ctxBottom=%d h=%d",
				w, h, lay.ContextBar.Y+lay.ContextBar.H, h)
		}
	}
}

// TestShellBodyLastRowNotClipped is the regression guard for the "panel bottom
// border chop": the body frame the shell hands a screen (ComputeLayoutNav's
// Workspace) must be EXACTLY the number of rows the shell then displays, so a
// screen that fills its frame to the last row (a bottom panel border) is never
// clipped by the shell's final clamp. It drives the real AppShell with a body
// of Workspace.H sentinel rows and asserts the last row survives in the output.
func TestShellBodyLastRowNotClipped(t *testing.T) {
	shell := components.NewAppShell(theme.Build(theme.Default()))
	sizes := [][2]int{{80, 24}, {100, 30}, {120, 40}, {156, 41}, {160, 50}, {200, 60}, {156, 28}}
	for _, s := range sizes {
		w, h := s[0], s[1]
		lay := ComputeLayoutNav(w, h, false)
		if lay.TooSmall {
			continue
		}
		bh := lay.Workspace.H
		if bh < 1 {
			continue
		}
		lines := make([]string, bh)
		for i := range lines {
			lines[i] = "fillrow"
		}
		const sentinel = "ZZ_LAST_ROW_SENTINEL_ZZ"
		lines[bh-1] = sentinel
		out := shell.View(components.ShellFrame{
			Width: w, Height: h, Title: "Dashboard",
			Body: strings.Join(lines, "\n"), NavWidth: lay.SideNav.W,
		})
		if !strings.Contains(out, sentinel) {
			t.Errorf("%dx%d: shell clipped the body's last row (workspaceH=%d); the "+
				"layout gave the screen more rows than the shell displays", w, h, bh)
		}
	}
}
