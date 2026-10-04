package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// panelBalance reports (tops, bottoms) of rounded-box borders. A mismatch means
// a panel's bottom border was clipped (inner-content chop).
func panelBalance(out string) (int, int) {
	return strings.Count(out, "╭"), strings.Count(out, "╰")
}

// TestNetworkCompactLineCount instruments network at 100x30.
func TestNetworkCompactLineCount(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	n := NewNetwork(ctx)
	if cmd := n.Init(); cmd != nil {
		var m Model = n
		for _, msg := range runBatch(cmd) {
			m, _ = m.Update(msg)
		}
		n = m.(*Network)
	}
	f := components.Frame{W: 100, H: 30}
	out := n.View(f)
	lines := strings.Split(out, "\n")
	t.Logf("network lines=%d / H=%d", len(lines), f.H)
	for i := 0; i < 4 && i < len(lines); i++ {
		t.Logf("top line %d: %q", i, lines[i])
	}
}

// TestWalletLineCount instruments the wallet render to find the off-by-one.
func TestWalletLineCount(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	w := NewWallet(ctx)
	if cmd := w.Init(); cmd != nil {
		var m Model = w
		for _, msg := range runBatch(cmd) {
			m, _ = m.Update(msg)
		}
		w = m.(*Wallet)
	}
	f := components.Frame{W: 156, H: 41}
	out := w.View(f)
	lines := strings.Split(out, "\n")
	t.Logf("wallet lines=%d / H=%d", len(lines), f.H)
	tops, bots := panelBalance(out)
	t.Logf("tops=%d bots=%d", tops, bots)
	// Dump the last 4 lines with widths to see the border asymmetry.
	for i := len(lines) - 4; i < len(lines); i++ {
		if i >= 0 {
			t.Logf("line %d (w=%d): %q", i, len([]rune(lines[i])), lines[i])
		}
	}
}

// TestInnerClipDashboardPreview prints the dashboard at the user's 156x41 so a
// human can see any INNER panel whose content is clipped by its box (the outer
// frame fills, but a sub-panel body taller than its allocation loses rows).
func TestInnerClipDashboardPreview(t *testing.T) {
	ctx := fixtureCtx(t, Subject{})
	out := renderDashboard(t, ctx, components.Frame{W: 156, H: 41})
	t.Logf("\n%s", out)
	tops, bots := panelBalance(out)
	t.Logf("dashboard panel tops=%d bottoms=%d", tops, bots)
}

// TestInnerClipAllScreensBalance renders the subject + non-orchestrator screens
// at 156x41 and FAILS if any panel's bottom border is clipped (tops != bottoms)
// — the "under the hood" inner cut-off the user reports.
func TestInnerClipAllScreensBalance(t *testing.T) {
	factories := map[string]func(*ScreenCtx) Model{
		"home":        func(c *ScreenCtx) Model { return NewHome(c) },
		"analysis":    func(c *ScreenCtx) Model { return NewSearch(c) },
		"transaction": func(c *ScreenCtx) Model { return NewTransaction(c) },
		"entity":      func(c *ScreenCtx) Model { return NewEntity(c) },
		"detection":   func(c *ScreenCtx) Model { return NewDetection(c) },
		"wallet":      func(c *ScreenCtx) Model { return NewWallet(c) },
		"network":     func(c *ScreenCtx) Model { return NewNetwork(c) },
		"alerts":      func(c *ScreenCtx) Model { return NewAlerts(c) },
		"monitoring":  func(c *ScreenCtx) Model { return NewMonitoring(c) },
		"data":        func(c *ScreenCtx) Model { return NewData(c) },
		"reports":     func(c *ScreenCtx) Model { return NewReports(c) },
		"settings":    func(c *ScreenCtx) Model { return NewSettings(c) },
		"extensions":  func(c *ScreenCtx) Model { return NewExtensions(c) },
	}
	// Realistic terminal sizes (a 1080p+ fullscreen terminal is ~120-240 cols ×
	// 30-60 rows). The user's terminal is 156x41. The very tight 100x24 compact
	// floor is covered by the overflow test, not the border-balance check.
	for _, sz := range []components.Frame{{W: 156, H: 41}, {W: 120, H: 40}, {W: 160, H: 50}, {W: 200, H: 60}} {
		for name, factory := range factories {
			ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
			m := factory(ctx)
			if cmd := m.Init(); cmd != nil {
				for _, msg := range runBatch(cmd) {
					m, _ = m.Update(msg)
				}
			}
			out := m.View(sz)
			tops, bots := panelBalance(out)
			if tops != bots {
				t.Errorf("%s @ %dx%d: panel border mismatch tops=%d bots=%d (bottom clipped)\n%s",
					name, sz.W, sz.H, tops, bots, out)
			}
		}
	}
}
