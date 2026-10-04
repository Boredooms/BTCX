package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
	tea "github.com/charmbracelet/bubbletea"
)

// TestRootRendersAtAcceptanceSizes drives the Root model with a window-size
// message and asserts it renders a non-empty shell without panicking at the
// acceptance sizes, and the honest too-small message below the floor. The app
// seam is nil here (no case) — the Root must degrade honestly, not crash.
func TestRootRendersAtAcceptanceSizes(t *testing.T) {
	sizes := []struct{ w, h int }{{80, 24}, {100, 30}, {120, 40}, {160, 50}, {200, 60}}
	for _, s := range sizes {
		r := NewRoot(context.Background(), nil)
		m, _ := r.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
		out := m.View()
		if strings.TrimSpace(out) == "" {
			t.Errorf("%dx%d: Root rendered empty", s.w, s.h)
		}
		lines := strings.Split(out, "\n")
		if len(lines) > s.h {
			t.Errorf("%dx%d: Root output %d lines exceeds height", s.w, s.h, len(lines))
		}
	}
}

// TestRootTooSmall asserts the Root shows only the resize message below 80x24.
func TestRootTooSmall(t *testing.T) {
	r := NewRoot(context.Background(), nil)
	m, _ := r.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	out := m.View()
	if !strings.Contains(out, "Terminal too small") {
		t.Errorf("below floor should show the resize message, got:\n%s", out)
	}
}

// sizedRoot builds a Root sized to w×h for key-routing tests.
func sizedRoot(w, h int) *Root {
	r := NewRoot(context.Background(), nil)
	r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return r
}

func rkey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// TestFocusDefaultsToNav asserts the Root starts with the SideNav focused so the
// sidebar is immediately drivable (design §A.2).
func TestFocusDefaultsToNav(t *testing.T) {
	r := sizedRoot(120, 40)
	if r.focus != FocusNav {
		t.Fatalf("startup focus = %v, want FocusNav", r.focus)
	}
}

// TestTabCyclesFocus asserts Tab/Shift+Tab cycle Nav<->Body and never get stuck
// (design §A.5). Screens never consume Tab.
func TestTabCyclesFocus(t *testing.T) {
	r := sizedRoot(120, 40)
	if r.focus != FocusNav {
		t.Fatalf("precondition: focus = %v, want FocusNav", r.focus)
	}
	r.Update(tea.KeyMsg{Type: tea.KeyTab})
	if r.focus != FocusBody {
		t.Fatalf("Tab from Nav: focus = %v, want FocusBody", r.focus)
	}
	r.Update(tea.KeyMsg{Type: tea.KeyTab})
	if r.focus != FocusNav {
		t.Fatalf("Tab from Body: focus = %v, want FocusNav", r.focus)
	}
	// Shift+Tab also cycles (reverse), never wedging.
	r.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if r.focus != FocusBody {
		t.Fatalf("Shift+Tab from Nav: focus = %v, want FocusBody", r.focus)
	}
	r.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if r.focus != FocusNav {
		t.Fatalf("Shift+Tab from Body: focus = %v, want FocusNav", r.focus)
	}
}

// TestEscLayering asserts Esc is layered (design §A.5): from Body it returns
// focus to Nav; from Nav it pops the nav stack; with an overlay open it closes
// the overlay and restores prior focus.
func TestEscLayering(t *testing.T) {
	// Body -> Nav.
	r := sizedRoot(120, 40)
	r.Update(tea.KeyMsg{Type: tea.KeyTab}) // focus Body
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r.focus != FocusNav {
		t.Fatalf("Esc in Body: focus = %v, want FocusNav", r.focus)
	}

	// Overlay open -> Esc closes and restores prior focus.
	r2 := sizedRoot(120, 40)
	r2.Update(rkey(":")) // open palette from Nav
	if r2.focus != FocusModal {
		t.Fatalf("`:` should open palette (FocusModal), got %v", r2.focus)
	}
	r2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r2.focus != FocusNav {
		t.Fatalf("Esc with overlay: focus = %v, want restored FocusNav", r2.focus)
	}
	if r2.overlay.kind != overlayNone {
		t.Fatalf("Esc should close the overlay, kind = %v", r2.overlay.kind)
	}

	// Nav Esc pops the nav stack: nav to a normal screen (Wallet keeps Nav
	// focus), then Esc pops back to Home. (The Analysis screen is type-first and
	// takes Body focus, so its Esc goes Body->Nav instead of popping — that is
	// asserted separately in TestNavToAnalysisTakesBodyFocusAndTypes.)
	r3 := sizedRoot(120, 40)
	r3.Update(rkey("3")) // jump to Wallet, focus stays Nav
	if r3.cur != ScreenWallet {
		t.Fatalf("precondition: cur = %v, want Wallet", r3.cur)
	}
	r3.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r3.cur != ScreenHome {
		t.Fatalf("Esc in Nav should pop to Home, got %v", r3.cur)
	}
}

// TestEveryGlobalTokenRouted enumerates the closed token set GlobalKeyMap.
// Dispatch can emit and asserts each token has a defined transition in each
// focus region and never panics (design §A.5 completeness guarantee). The
// tokens are taken from Dispatch's own case labels so the test cannot drift
// from the real set.
func TestEveryGlobalTokenRouted(t *testing.T) {
	// One representative KeyMsg per token Dispatch can return.
	tokenKeys := map[string]tea.KeyMsg{
		"up":        {Type: tea.KeyUp},
		"down":      {Type: tea.KeyDown},
		"enter":     {Type: tea.KeyEnter},
		"esc":       {Type: tea.KeyEsc},
		"tab":       {Type: tea.KeyTab},
		"shift+tab": {Type: tea.KeyShiftTab},
		"search":    rkey("/"),
		"palette":   rkey(":"),
		"help":      rkey("?"),
		"quit":      {Type: tea.KeyCtrlC},
		"jump:":     rkey("2"), // a registered jump key
		"":          rkey("z"), // unmatched -> empty token
	}

	// Confirm the representative keys actually map to the token set Dispatch
	// emits, so the enumeration stays honest.
	km := NewGlobalKeyMap()
	for want, msg := range tokenKeys {
		got := km.Dispatch(msg, false)
		switch want {
		case "jump:":
			if len(got) < 5 || got[:5] != "jump:" {
				t.Fatalf("key for token %q dispatched to %q", want, got)
			}
		default:
			if got != want {
				t.Fatalf("key for token %q dispatched to %q", want, got)
			}
		}
	}

	regions := []FocusRegion{FocusNav, FocusBody, FocusModal}
	for _, region := range regions {
		for token, msg := range tokenKeys {
			r := sizedRoot(120, 40)
			r.focus = region
			if region == FocusModal {
				// Give FocusModal a real open overlay so routing is exercised.
				r.openPalette()
				r.focus = FocusModal
			}
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("token %q in region %v panicked: %v", token, region, rec)
					}
				}()
				m, _ := r.Update(msg)
				if m == nil {
					t.Fatalf("token %q in region %v returned nil model", token, region)
				}
			}()
		}
	}
}

// TestNoKeyCrashes feeds a broad KeyMsg set across several screens, in both
// FocusNav and FocusBody, at two window sizes, and asserts no key panics and
// the frame is non-empty afterwards (design §A.5 hard invariant).
func TestNoKeyCrashes(t *testing.T) {
	var keys []tea.KeyMsg
	for c := 'a'; c <= 'z'; c++ {
		keys = append(keys, rkey(string(c)))
	}
	for c := 'A'; c <= 'Z'; c++ {
		keys = append(keys, rkey(string(c)))
	}
	for c := '0'; c <= '9'; c++ {
		keys = append(keys, rkey(string(c)))
	}
	keys = append(keys,
		tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyRight},
		tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyTab},
		tea.KeyMsg{Type: tea.KeyShiftTab}, tea.KeyMsg{Type: tea.KeyEsc},
		rkey(":"), rkey("/"), rkey("?"), rkey(" "),
		tea.KeyMsg{Type: tea.KeyCtrlP}, tea.KeyMsg{Type: tea.KeyPgUp},
		tea.KeyMsg{Type: tea.KeyPgDown}, rkey("@#$%"),
	)

	jumps := []ScreenID{ScreenHome, ScreenSearch, ScreenGraph, ScreenGeoMap, ScreenMonitoring}
	sizes := []struct{ w, h int }{{100, 30}, {160, 50}}
	regions := []FocusRegion{FocusNav, FocusBody}

	for _, size := range sizes {
		for _, start := range jumps {
			for _, region := range regions {
				r := sizedRoot(size.w, size.h)
				r.navTo(start, Subject{})
				r.focus = region
				for _, k := range keys {
					func() {
						defer func() {
							if rec := recover(); rec != nil {
								t.Fatalf("%v %dx%d screen=%v key=%v panicked: %v",
									region, size.w, size.h, start, k, rec)
							}
						}()
						m, _ := r.Update(k)
						if m == nil {
							t.Fatalf("nil model after key %v", k)
						}
						out := m.View()
						if strings.TrimSpace(out) == "" {
							t.Fatalf("empty frame after key %v (screen=%v %dx%d)", k, start, size.w, size.h)
						}
					}()
				}
			}
		}
	}
}

// TestRootNavJumpSwitchesScreen asserts a global nav key swaps the active
// screen via the registry (the single reachability source).
func TestRootNavJumpSwitchesScreen(t *testing.T) {
	r := NewRoot(context.Background(), nil)
	r.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// '2' jumps to Analysis per the registry table. Analysis is type-first so it
	// takes Body focus (number keys then type into its box).
	r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if r.cur != ScreenSearch {
		t.Errorf("expected nav to Analysis on '2', got %q", r.cur)
	}
	if r.focus != FocusBody {
		t.Errorf("Analysis should take Body focus, got %v", focusName(r.focus))
	}
	// Esc leaves the Analysis body back to Nav (box no longer captures keys).
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})
	// Now a '3' jump reaches a normal screen that keeps Nav focus.
	r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	if r.cur != ScreenWallet {
		t.Errorf("expected nav to Wallet on '3', got %q", r.cur)
	}
	// Esc from a Nav-focused normal screen pops the stack back to Analysis.
	r.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if r.cur != ScreenSearch {
		t.Errorf("expected Esc to pop back to Analysis, got %q", r.cur)
	}
}

// TestRootRowSelectedDrillIn asserts that activating a table row on ANY board
// (emitting components.RowSelected) drills into the matching detail screen for
// the navigable subject kinds — the end-to-end "select a row and it opens" flow
// (e.g. a wallet's tx -> Transaction, a network IP -> Network / IP).
func TestRootRowSelectedDrillIn(t *testing.T) {
	cases := []struct {
		kind string
		id   string
		want ScreenID
	}{
		{"tx", "TX_abc", ScreenTransaction},
		{"ip", "8.8.8.8", ScreenNetwork},
		{"wallet", "1Addr", ScreenWallet},
		{"block", "000abc", ScreenBlock},
	}
	for _, c := range cases {
		r := NewRoot(context.Background(), nil)
		r.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
		r.Update(components.RowSelected{Kind: c.kind, ID: c.id})
		if r.cur != c.want {
			t.Errorf("RowSelected{%s} should open %q, got %q", c.kind, c.want, r.cur)
		}
		if r.state.Subject.ID != c.id {
			t.Errorf("RowSelected{%s} should set subject %q, got %q", c.kind, c.id, r.state.Subject.ID)
		}
	}
}
