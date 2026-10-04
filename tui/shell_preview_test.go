package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// shell_preview_test.go prints the FULL interactive shell for the four required
// scenes at 160x50 and 80x24 (FEAT-005, design Test Plan tail). The previews
// mirror TestGeoMapPreview / scripts/geomap_preview.sh: they are deterministic
// (a seeded in-memory case, a frozen clock, a throwaway HOME with the synthetic
// GeoIP + geometry fixtures, no network) and NOT assertions — run them with
//
//	go test ./tui -run Preview -v
//
// to eyeball the rendered shell. Each scene is rendered through the real Root
// so the preview reflects the shipped focus/overlay/viewport behavior, not a
// component snapshot. scripts/shell_preview.sh wraps these for a human.

// previewSizes are the two terminal rectangles every scene is printed at: the
// wide reference (160x50) and the acceptance floor (80x24).
var previewSizes = []struct {
	name string
	w, h int
}{
	{"160x50", 160, 50},
	{"80x24", 80, 24},
}

// renderPreview sizes the root, renders the full shell, and asserts the hard
// no-overflow invariant (design §6) at the given size before returning the
// frame for the human-readable log. The assertion makes the preview a real
// gate: a broken box at either size fails the test, not just a visual.
func renderPreview(t *testing.T, r *Root, w, h int) string {
	t.Helper()
	r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	out := r.View()
	assertFitsWithin(t, out, w, h)
	return out
}

// drivePreview applies a sequence of messages to the root, draining each
// returned command so async nav/data loads settle before the next step.
func drivePreview(r *Root, msgs ...tea.Msg) {
	for _, msg := range msgs {
		_, cmd := r.Update(msg)
		for _, m := range drainGateCmd(cmd) {
			r.Update(m)
		}
	}
}

// TestShellDashboardPreview — scene (a): the dashboard cockpit with the SideNav
// focused and a non-default item under the selection cursor.
func TestShellDashboardPreview(t *testing.T) {
	for _, sz := range previewSizes {
		sz := sz
		t.Run(sz.name, func(t *testing.T) {
			r := gateRoot(t)
			// Nav is focused at startup; move the cursor so a non-active item is
			// highlighted (focused + selected SideNav item).
			drivePreview(r, tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyDown})
			t.Logf("\n[SHELL — DASHBOARD, SideNav focused+selected @ %s]\n%s",
				sz.name, renderPreview(t, r, sz.w, sz.h))
		})
	}
}

// TestShellPalettePreview — scene (b): the command palette overlay open and
// centered over the body.
func TestShellPalettePreview(t *testing.T) {
	for _, sz := range previewSizes {
		sz := sz
		t.Run(sz.name, func(t *testing.T) {
			r := gateRoot(t)
			drivePreview(r, rkey(":"))
			t.Logf("\n[SHELL — COMMAND PALETTE open @ %s]\n%s",
				sz.name, renderPreview(t, r, sz.w, sz.h))
		})
	}
}

// TestShellSearchPreview — scene (c): the Search screen with a query typed and
// its bounded results populated (seeded via the global '/' overlay submit).
func TestShellSearchPreview(t *testing.T) {
	for _, sz := range previewSizes {
		sz := sz
		t.Run(sz.name, func(t *testing.T) {
			r := gateRoot(t)
			// Open the global search overlay, type a known subject, submit; the
			// Root navigates to Search seeding the query + running the lookup.
			msgs := []tea.Msg{rkey("/")}
			for _, c := range "WGATE" {
				msgs = append(msgs, rkey(string(c)))
			}
			msgs = append(msgs, tea.KeyMsg{Type: tea.KeyEnter})
			drivePreview(r, msgs...)
			t.Logf("\n[SHELL — SEARCH query+results @ %s]\n%s",
				sz.name, renderPreview(t, r, sz.w, sz.h))
		})
	}
}

// TestShellGeoMapPreview — scene (d): the Geo Map zoomed into a region, with a
// cluster selected and the detail panel open.
func TestShellGeoMapPreview(t *testing.T) {
	for _, sz := range previewSizes {
		sz := sz
		t.Run(sz.name, func(t *testing.T) {
			r := gateRoot(t)
			// Jump to the Geo Map, focus Body, select a cluster (recenters on
			// it), zoom in a couple of steps, and open the detail panel.
			drivePreview(r,
				rkey("8"),                    // nav to Geo Map
				tea.KeyMsg{Type: tea.KeyTab}, // focus Body so viewport keys land
				rkey("n"),                    // select + recenter on a cluster
				rkey("+"), rkey("+"),         // zoom into the region
				tea.KeyMsg{Type: tea.KeyEnter}, // open the selection detail panel
			)
			t.Logf("\n[SHELL — GEO MAP zoomed+selected @ %s]\n%s",
				sz.name, renderPreview(t, r, sz.w, sz.h))
		})
	}
}
