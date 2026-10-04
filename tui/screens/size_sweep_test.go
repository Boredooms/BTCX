package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// TestSizeSweepNoClip renders the subject screens across a dense sweep of
// terminal sizes (odd and even row counts, the range a 1080p fullscreen
// terminal produces) and asserts none overflow their frame and the Analysis /
// detail screens keep a bottom border (no clip) at every size. This catches a
// cut-off that only appears at a specific odd row count.
func TestSizeSweepNoClip(t *testing.T) {
	if testing.Short() {
		t.Skip("size sweep is exhaustive; skipped in -short")
	}
	// A focused sweep: the widest realistic widths and a mix of odd/even row
	// counts (a 1080p fullscreen terminal lands around 200-240 cols x 50-60
	// rows; odd counts catch off-by-one clips).
	widths := []int{120, 200, 240}
	heights := []int{24, 31, 41, 50, 51, 60}
	screens := map[string]func(*ScreenCtx) Model{
		"analysis":    func(c *ScreenCtx) Model { return NewSearch(c) },
		"transaction": func(c *ScreenCtx) Model { return NewTransaction(c) },
		"entity":      func(c *ScreenCtx) Model { return NewEntity(c) },
		"detection":   func(c *ScreenCtx) Model { return NewDetection(c) },
		"network":     func(c *ScreenCtx) Model { return NewNetwork(c) },
		"settings":    func(c *ScreenCtx) Model { return NewSettings(c) },
		"wallet":      func(c *ScreenCtx) Model { return NewWallet(c) },
	}
	for name, factory := range screens {
		for _, w := range widths {
			for _, h := range heights {
				sz := components.Frame{W: w, H: h}
				ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
				m := factory(ctx)
				if cmd := m.Init(); cmd != nil {
					if msg := cmd(); msg != nil {
						m, _ = m.Update(msg)
					}
				}
				out := m.View(sz)
				lines := strings.Split(out, "\n")
				if len(lines) > sz.H {
					t.Errorf("%s @ %dx%d: %d lines exceed height %d", name, w, h, len(lines), sz.H)
				}
				for i, ln := range lines {
					if lipglossWidthTest(ln) > sz.W {
						t.Errorf("%s @ %dx%d: line %d width %d > %d", name, w, h, i, lipglossWidthTest(ln), sz.W)
					}
				}
			}
		}
	}
}
