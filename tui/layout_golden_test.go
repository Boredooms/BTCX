package tui

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// layout_golden_test.go is the five-size layout golden suite (design §C golden
// plan). For each of {80x24,100x30,120x40,160x50,200x60} it builds a Root over
// a seeded in-memory case with a FROZEN clock, sends a WindowSizeMsg, renders
// Root.View(), and asserts three things:
//
//  1. HARD no-box-break at every size: every rendered line's lipgloss width is
//     <= the terminal width and the line count is <= the terminal height.
//  2. Border closure: at a size whose body frame clears the panel-chrome floor
//     the body renders bordered panels (box-drawing glyphs); below the floor
//     the shell degrades to a borderless fallback (none expected here because
//     all five sizes clear it; the too-small floor is covered separately).
//  3. Equality to a committed golden tui/testdata/golden/shell_<w>x<h>.txt,
//     regenerated with `go test ./tui -run Golden -update`.
//
// TestTooSmallFloor covers 79x24 and 80x23 (only the centered resize message)
// and a navForced-ON 80x24 golden asserts no line exceeds the width (NIT-2).

// updateGolden is set by the -update flag; when true the goldens are rewritten
// instead of asserted.
var updateGolden = flag.Bool("update", false, "regenerate layout golden files")

// goldenSize is one terminal rectangle under test plus whether its body is
// expected to render bordered panels (BORDERED) or a borderless fallback
// (DEGRADED). All five production sizes clear the panel-chrome floor, so each
// is BORDERED; the DEGRADED path is exercised by TestTooSmallFloor.
type goldenSize struct {
	w, h     int
	bordered bool
}

var goldenSizes = []goldenSize{
	{80, 24, true},
	{100, 30, true},
	{120, 40, true},
	{160, 50, true},
	{200, 60, true},
}

// frozenClock returns a fixed instant so the TopBar clock is deterministic
// across golden runs (design §C.4).
func frozenClock() func() time.Time {
	t := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	return func() time.Time { return t }
}

// goldenRoot builds a Root over a seeded in-memory case with the clock frozen,
// sized to w x h. The seam is a real sqlite repo seeded with one tx + one
// observation (fixtures live only in tests, AGENTS §15/§16) so the dashboard
// renders populated, deterministic panels rather than loading placeholders.
func goldenRoot(t *testing.T, w, h int) *Root {
	t.Helper()
	// Hermetic HOME: the dashboard's GEOIP badge and models status read the
	// runtime dirs under $HOME (~/.bctx). Isolating HOME to an empty temp dir
	// makes the golden independent of host-installed GeoIP DBs / models, so the
	// committed golden is identical on a dev machine and in CI (where neither is
	// installed). Without this the golden bakes in machine-specific "GeoIP City
	// DB" / "v2024" strings that CI can't reproduce.
	t.Setenv("HOME", t.TempDir())
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "golden.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	ctx := context.Background()
	tx := schema.Transaction{
		TxID:      "TXGOLDEN1",
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		FeeBTC:    0.0001,
		Inputs:    []schema.TransactionInput{{Address: "WGOLDEN", AmountBTC: 1.0, Index: 0}},
		Outputs:   []schema.TransactionOutput{{Address: "WOTHER", AmountBTC: 0.99, Index: 0}},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	obs := schema.NetworkObservation{
		ID: "OBS1", TxID: "TXGOLDEN1", SrcIP: "203.0.113.9", DstIP: "198.51.100.2",
		Country: "DE", ASN: "AS3320",
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	if err := repo.SaveNetworkObservations(ctx, []schema.NetworkObservation{obs}); err != nil {
		t.Fatalf("seed obs: %v", err)
	}
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline
	a := &app.App{Repo: repo, CaseID: "golden-case", Cleanup: func() {}}

	r := NewRoot(ctx, a)
	r.nowFn = frozenClock()
	// Drive the dashboard's Init so its async panels fold in deterministically
	// before rendering (mirrors how the program drives the active screen).
	runScreenInit(r)
	r.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return r
}

// runScreenInit runs the active screen's Init command synchronously and folds
// each resulting message back into the active screen, so the golden reflects
// loaded panels, not loading placeholders.
func runScreenInit(r *Root) {
	if r.active == nil {
		return
	}
	cmd := r.active.Init()
	for _, msg := range drainCmd(cmd) {
		next, _ := r.active.Update(msg)
		r.active = next
	}
}

// drainCmd flattens a (possibly batched) tea.Cmd into the messages it emits,
// running each leaf command once. It understands tea.BatchMsg so a tea.Batch of
// data-load commands is fully resolved. It does NOT recurse into commands the
// resulting messages themselves might schedule (none of the dashboard loads
// chain), which keeps it deterministic and terminating.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	msg := cmd()
	switch m := msg.(type) {
	case tea.BatchMsg:
		for _, c := range m {
			out = append(out, drainCmd(c)...)
		}
	case nil:
	default:
		out = append(out, m)
	}
	return out
}

// goldenPath is the committed golden file for a size.
func goldenPath(w, h int) string {
	return filepath.Join("testdata", "golden", shellGoldenName(w, h))
}

func shellGoldenName(w, h int) string {
	return "shell_" + itoa(w) + "x" + itoa(h) + ".txt"
}

// itoa is a tiny base-10 int formatter (avoids pulling strconv just for names).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// TestLayoutGoldenShell renders the Root at the five sizes, asserts the hard
// no-overflow invariant and border expectation, and compares to (or with
// -update, rewrites) the committed golden.
func TestLayoutGoldenShell(t *testing.T) {
	for _, sz := range goldenSizes {
		sz := sz
		t.Run(shellGoldenName(sz.w, sz.h), func(t *testing.T) {
			r := goldenRoot(t, sz.w, sz.h)
			out := r.View()

			assertFitsWithin(t, out, sz.w, sz.h)

			hasBorder := strings.ContainsAny(out, "─│╭╮╰╯")
			if sz.bordered && !hasBorder {
				t.Errorf("%dx%d: expected bordered panels (box glyphs), got none:\n%s", sz.w, sz.h, out)
			}
			if !sz.bordered && hasBorder {
				t.Errorf("%dx%d: expected borderless fallback, found box glyphs:\n%s", sz.w, sz.h, out)
			}

			path := goldenPath(sz.w, sz.h)
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s (run `go test ./tui -run Golden -update`): %v", path, err)
			}
			if string(want) != out {
				t.Errorf("%dx%d: output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s",
					sz.w, sz.h, path, out, string(want))
			}
		})
	}
}

// TestTooSmallFloor asserts that below the 80x24 floor the shell renders ONLY
// the centered resize message (DEGRADED, no box glyphs) and never exceeds the
// terminal rectangle (design §6).
func TestTooSmallFloor(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{79, 24}, {80, 23}} {
		r := goldenRoot(t, sz.w, sz.h)
		out := r.View()
		if !strings.Contains(out, "Terminal too small") {
			t.Errorf("%dx%d: below floor should show only the resize message, got:\n%s", sz.w, sz.h, out)
		}
		if strings.ContainsAny(out, "─│╭╮╰╯") {
			t.Errorf("%dx%d: below floor should be borderless, found box glyphs:\n%s", sz.w, sz.h, out)
		}
		assertFitsWithin(t, out, sz.w, sz.h)
	}
}

// TestLayoutGoldenNavForced renders the navForced-ON 80x24 shell (the Compact
// rail is forced on with `b`), asserts no line exceeds the width (NIT-2: the
// reserved nav rail and the rendered rail agree), and compares to (or with
// -update rewrites) its dedicated golden. Its name contains "Golden" so the
// `go test ./tui -run Golden -update` invocation regenerates it alongside the
// five-size goldens.
func TestLayoutGoldenNavForced(t *testing.T) {
	r := goldenRoot(t, 80, 24)
	r.navForced = true
	out := r.View()
	assertFitsWithin(t, out, 80, 24)

	path := filepath.Join("testdata", "golden", "shell_80x24_navforced.txt")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatalf("write navforced golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read navforced golden %s (run `go test ./tui -run Golden -update`): %v", path, err)
	}
	if string(want) != out {
		t.Errorf("navForced 80x24: output does not match golden %s\n--- got ---\n%s\n--- want ---\n%s",
			path, out, string(want))
	}
}

// assertFitsWithin is the hard no-box-break invariant: every rendered line's
// display width is <= w and the line count is <= h.
func assertFitsWithin(t *testing.T, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > h {
		t.Errorf("line count %d exceeds height %d", len(lines), h)
	}
	for i, ln := range lines {
		if lw := lipgloss.Width(ln); lw > w {
			t.Errorf("line %d width %d exceeds %d: %q", i, lw, w, ln)
		}
	}
}
