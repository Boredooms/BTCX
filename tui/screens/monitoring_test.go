package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// TestMonitoringTargetSelector asserts the in-screen monitor-target chooser:
// 't' focuses the input, a valid wallet/tx id yields the exact gated `bctx
// monitor <sub> <id>` command, and a garbage id yields an honest reject note.
func TestMonitoringTargetSelector(t *testing.T) {
	wallet := "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	txid := strings.Repeat("a", 64)

	t.Run("wallet target -> monitor wallet command", func(t *testing.T) {
		mo := NewMonitoring(fixtureCtx(t, Subject{}))
		mo.target.Focus()
		mo.target.SetValue(wallet)
		_, _ = mo.Update(components.SearchSubmitted{Query: wallet})
		if !strings.Contains(mo.targetNote, "bctx monitor wallet "+wallet) {
			t.Fatalf("expected gated wallet command, got note: %q", mo.targetNote)
		}
	})

	t.Run("tx target -> monitor tx command", func(t *testing.T) {
		mo := NewMonitoring(fixtureCtx(t, Subject{}))
		mo.target.Focus()
		mo.target.SetValue(txid)
		_, _ = mo.Update(components.SearchSubmitted{Query: txid})
		if !strings.Contains(mo.targetNote, "bctx monitor tx "+txid) {
			t.Fatalf("expected gated tx command, got note: %q", mo.targetNote)
		}
	})

	t.Run("garbage -> honest reject", func(t *testing.T) {
		mo := NewMonitoring(fixtureCtx(t, Subject{}))
		mo.Update(components.SearchSubmitted{Query: "not-an-id"})
		if mo.targetNote == "" || strings.Contains(mo.targetNote, "bctx monitor") {
			t.Fatalf("garbage id should reject without a command, got: %q", mo.targetNote)
		}
	})

	t.Run("'t' focuses the selector", func(t *testing.T) {
		mo := NewMonitoring(fixtureCtx(t, Subject{}))
		mo.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
		if !mo.Focused() {
			t.Fatal("'t' should focus the monitor-target selector")
		}
	})
}

// TestMonitoringSeedsActiveSubject asserts the selector pre-fills with the
// active global subject so the target the user was investigating is one keypress
// away.
func TestMonitoringSeedsActiveSubject(t *testing.T) {
	addr := "1BvBMSEYstWetqTFn5Au4m4GFg7xJaNVN2"
	mo := NewMonitoring(fixtureCtx(t, Subject{ID: addr, Kind: SubjectWallet}))
	out := mo.View(components.Frame{W: 160, H: 40})
	if !strings.Contains(out, addr) {
		t.Errorf("monitor selector should pre-fill the active subject %q, got:\n%s", addr, out)
	}
}
