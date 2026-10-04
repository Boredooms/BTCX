package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// TestBottomClipProbe renders every screen against the seeded fixture at the
// acceptance sizes and reports, per screen, the rendered line count vs the
// frame height and whether the last non-blank line still carries a bottom
// border glyph (a clipped panel loses its '╰'/'─' bottom edge). Diagnostic only
// — it logs, never fails — so we can SEE which screens clip at the bottom.
func TestBottomClipProbe(t *testing.T) {
	sizes := []components.Frame{{W: 156, H: 41}, {W: 120, H: 40}, {W: 160, H: 50}}
	// Only the non-orchestrator screens here (the orchestrator-backed ones —
	// wallet/entity/detection — try to load ML models and make this diagnostic
	// slow; their fill is covered by the regular screen tests).
	factories := map[string]func(*ScreenCtx) Model{
		"home":        func(c *ScreenCtx) Model { return NewHome(c) },
		"search":      func(c *ScreenCtx) Model { return NewSearch(c) },
		"transaction": func(c *ScreenCtx) Model { return NewTransaction(c) },
		"network":     func(c *ScreenCtx) Model { return NewNetwork(c) },
		"geomap":      func(c *ScreenCtx) Model { return NewGeoMap(c) },
		"alerts":      func(c *ScreenCtx) Model { return NewAlerts(c) },
		"monitoring":  func(c *ScreenCtx) Model { return NewMonitoring(c) },
		"data":        func(c *ScreenCtx) Model { return NewData(c) },
		"reports":     func(c *ScreenCtx) Model { return NewReports(c) },
		"settings":    func(c *ScreenCtx) Model { return NewSettings(c) },
	}
	for name, factory := range factories {
		for _, sz := range sizes {
			ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
			m := factory(ctx)
			if cmd := m.Init(); cmd != nil {
				if msg := cmd(); msg != nil {
					m, _ = m.Update(msg)
				}
			}
			out := m.View(sz)
			lines := strings.Split(out, "\n")
			// Trailing blank lines => under-filled bottom (empty box region).
			trailingBlank := 0
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) == "" {
					trailingBlank++
				} else {
					break
				}
			}
			last := ""
			if len(lines) > 0 {
				last = lines[len(lines)-1]
			}
			hasBottomBorder := strings.ContainsAny(last, "╰╯┴─└┘")
			t.Logf("%-12s %dx%d: lines=%d/%d trailingBlank=%d lastHasBorder=%v",
				name, sz.W, sz.H, len(lines), sz.H, trailingBlank, hasBottomBorder)
		}
	}
}
