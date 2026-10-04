package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// cockpit.go holds the shared "godmode cockpit" chrome every screen composes
// its dense, bordered, multi-panel layout from: a titled bordered Panel box and
// a compact metric Tile. Both are pure presentation — they render a string the
// caller already built from a live service seam and clamp strictly to the Frame
// they are given (design §3, §6). Colors and borders come only from the active
// theme (styles.Theme().Border / semantic Roles); no component here constructs
// a lipgloss color literal (design §7, acceptance §19.5).
//
// Keeping this chrome in one leaf-package file means the Dashboard and the
// wallet/graph/monitoring/alerts screens all draw the same box, so the whole
// app reads as one cohesive instrument panel rather than ad-hoc text blocks.

// panelChromeH is the border + padding overhead a Panel adds around its body
// (top+bottom border = 2 rows, PanelPadY top+bottom). It is used to size the
// inner body frame so content never overflows the box.
func panelChromeV(styles theme.Styles) int {
	return 2 + 2*theme.Space.PanelPadY // top+bottom border + vertical padding
}

func panelChromeH(styles theme.Styles) int {
	return 2 + 2*theme.Space.PanelPadX // left+right border + horizontal padding
}

// Panel renders a titled, bordered box of exactly f.W x f.H cells containing
// body, truncated to the inner area. The title is drawn in the Title role on
// the first inner row; the body fills the rest. An empty frame renders nothing.
//
// Panel is the single titled-box primitive — every bordered region in the app
// (dashboard tiles row aside, the map/graph/table/detail panels, and the other
// screens' split panes) routes through it so the border glyph, color, padding,
// and title styling stay identical everywhere.
func Panel(styles theme.Styles, title, body string, f Frame) string {
	if f.Empty() {
		return ""
	}
	innerW := f.W - panelChromeH(styles)
	innerH := f.H - panelChromeV(styles)
	if innerW < 1 || innerH < 1 {
		// Too small to draw a bordered box; fall back to a clamped plain body so
		// a tight breakpoint degrades honestly instead of overflowing.
		head := title
		if head != "" && body != "" {
			head += " "
		}
		return clampBlock(styles.Role(theme.RoleTitle).Render(head)+body, f)
	}

	var inner strings.Builder
	bodyH := innerH
	if strings.TrimSpace(title) != "" {
		inner.WriteString(clampLine(styles.Title.Render(title), innerW))
		inner.WriteString("\n")
		bodyH = innerH - 1
	}
	if bodyH < 0 {
		bodyH = 0
	}
	// Clamp the body to the exact inner body height so the assembled inner block
	// is never taller than innerH — otherwise lipgloss MaxHeight would truncate
	// the box's BOTTOM BORDER (dropping the ╰───╯ line), which reads as a
	// chopped-off panel. Clamping here keeps the border intact at every size.
	inner.WriteString(clampBlock(body, Frame{W: innerW, H: bodyH}))

	// NOTE: MaxHeight is deliberately omitted here. If the styled body contains
	// a line wider than innerW, lipgloss re-wraps it and the rendered box grows
	// past innerH; MaxHeight would then silently truncate the BOTTOM BORDER
	// (the ╰───╯ row), which reads as a chopped panel. Instead we render at the
	// natural height and clamp below with a guard that always preserves the
	// final (border) row.
	box := styles.Panel.
		Width(innerW).
		Height(innerH).
		MaxWidth(f.W).
		Render(inner.String())
	// Guard: if the rendered box exceeds the frame height (e.g. a styled body
	// line wrapped), keep the first f.H-1 lines plus the final (bottom-border)
	// line, so the border is never the row that gets dropped.
	lines := splitLines(box)
	if len(lines) > f.H {
		kept := append(lines[:f.H-1:f.H-1], lines[len(lines)-1])
		box = joinLines(kept)
	}
	// Final width guard (a wide rune in the title can't push past the frame).
	return clampBlock(box, f)
}

// Tile is one bordered metric cell: a dim label over a bright value in the
// given semantic role. It is the dashboard's top-row building block. Width is
// fixed by the caller (TileRow splits the row evenly); height is 4 rows
// (border + label + value + border).
func Tile(styles theme.Styles, label, value string, role theme.Role, f Frame) string {
	if f.Empty() {
		return ""
	}
	innerW := f.W - panelChromeH(styles)
	if innerW < 1 {
		return clampBlock(styles.Role(role).Render(value), f)
	}
	// Truncate the PLAIN label/value to the inner width BEFORE styling so the
	// lipgloss panel Width() never has to word-wrap them onto a second line
	// (which previously split "TRANSACTIONS" -> "TRANSACTION"/"S" and broke the
	// tile's border alignment at narrow widths).
	lbl := styles.Label.Render(truncateCells(strings.ToUpper(label), innerW))
	val := lipgloss.NewStyle().Bold(true).
		Foreground(styles.Theme().Color(role)).Render(truncateCells(value, innerW))
	body := lbl + "\n" + val
	box := styles.Panel.
		Width(innerW).
		Height(f.H - panelChromeV(styles)).
		MaxWidth(f.W).
		MaxHeight(f.H).
		Render(body)
	return clampBlock(box, f)
}

// truncateCells cuts an UNSTYLED string to at most w display cells, appending an
// ellipsis when it must cut. Operates on runes (no ANSI) so it is exact for the
// plain label/value text tiles and rows use before styling.
func truncateCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

// TileSpec is one metric tile's content for TileRow.
type TileSpec struct {
	Label string
	Value string
	Role  theme.Role
}

// TileRow lays a set of metric tiles across the frame width. The width is split
// by SplitH (one shared budget rule), so tiles that cannot clear SplitH's floor
// are dropped from the right rather than overflowing, and a gap-wide blank
// spacer is rendered between tiles so the on-screen width is
// sum(cols)+(n-1)*gap == f.W exactly. The result is clamped to the frame; tile
// height defaults to the frame height.
// minTileW is the minimum inner width a tile needs so its (abbreviated) label
// and value read cleanly. TileRow fits as many tiles per row as the width
// allows at this minimum, wrapping the rest onto additional rows, so a tile is
// never squeezed narrow enough to wrap its label.
const minTileW = 16

// TilesPerRow reports how many tiles fit on one row at a given frame width, and
// TileRowsNeeded reports how many stacked rows n tiles will occupy — so a caller
// can budget the tile region's height (each row is 4 cells tall).
func TilesPerRow(width int) int {
	gap := theme.Space.Gap
	perRow := (width + gap) / (minTileW + gap)
	if perRow < 1 {
		perRow = 1
	}
	return perRow
}

// TileRowsNeeded returns the stacked-row count for n tiles at the given width.
func TileRowsNeeded(width, n int) int {
	if n <= 0 {
		return 0
	}
	per := TilesPerRow(width)
	if per > n {
		per = n
	}
	return (n + per - 1) / per
}

func TileRow(styles theme.Styles, tiles []TileSpec, f Frame) string {
	if f.Empty() || len(tiles) == 0 {
		return ""
	}
	gap := theme.Space.Gap
	// How many tiles fit on one row at the minimum tile width?
	perRow := (f.W + gap) / (minTileW + gap)
	if perRow < 1 {
		perRow = 1
	}
	if perRow > len(tiles) {
		perRow = len(tiles)
	}
	// Each visual row is tileH tall; render tiles in chunks of perRow and stack.
	var rows []string
	for start := 0; start < len(tiles); start += perRow {
		end := start + perRow
		if end > len(tiles) {
			end = len(tiles)
		}
		chunk := tiles[start:end]
		cols := SplitH(f.W, len(chunk), gap)
		if len(cols) == 0 {
			continue
		}
		spacer := gapSpacer(gap, f.H)
		parts := make([]string, 0, len(cols)*2)
		for i, w := range cols {
			if i > 0 && spacer != "" {
				parts = append(parts, spacer)
			}
			parts = append(parts, Tile(styles, chunk[i].Label, chunk[i].Value, chunk[i].Role,
				Frame{W: w, H: f.H}))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, parts...))
	}
	return clampBlock(strings.Join(rows, "\n"), f)
}

// Row2 joins two already-rendered panel strings side by side. The two column
// widths come from SplitH(f.W, 2, Gap) — the single width budget — and an
// explicit gap-wide blank spacer is rendered BETWEEN the columns, so the
// rendered width is col0 + gap + col1 == f.W exactly and no column double-counts
// the gap (design §C.2). The ratio argument biases which column keeps the
// remainder: a ratio > 0.5 widens the left column within the SplitH budget. If
// the frame cannot fit two floored columns, the left block is rendered
// full-width (the caller's content stacks). Columns are height-padded to align,
// then the whole row is clamped to the frame.
// Row2Widths returns the two column widths Row2 renders into for a body width
// and ratio, plus ok=false when the width cannot fit two floored columns (so
// the caller stacks instead). It is the single source for the split so a caller
// that sizes each panel BEFORE Row2 joins them (e.g. the Dashboard) passes
// exactly the width Row2 will clamp to — the sub-widths come from one SplitH
// budget, never a local f.W/2 literal (design §C.2). The two widths sum to
// SplitH(w,2,gap)[0]+[1], i.e. w-gap, so col0+gap+col1 == w.
func Row2Widths(w int, ratio float64) (left, right int, ok bool) {
	gap := theme.Space.Gap
	cols := SplitH(w, 2, gap)
	if len(cols) < 2 {
		return 0, 0, false
	}
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	total := cols[0] + cols[1]
	left = int(float64(total) * ratio)
	if left < minCol {
		left = minCol
	}
	if left > total-minCol {
		left = total - minCol
	}
	right = total - left
	return left, right, true
}

func Row2(f Frame, ratio float64, left, right string) string {
	if f.Empty() {
		return ""
	}
	leftW, rightW, ok := Row2Widths(f.W, ratio)
	if !ok {
		// No room for two floored columns: render the left content full-width.
		return clampBlock(padHeight(clampBlock(left, Frame{W: f.W, H: f.H}), f.H, f.W), f)
	}
	gap := theme.Space.Gap
	l := padHeight(clampBlock(left, Frame{W: leftW, H: f.H}), f.H, leftW)
	r := padHeight(clampBlock(right, Frame{W: rightW, H: f.H}), f.H, rightW)
	spacer := gapSpacer(gap, f.H)
	return clampBlock(lipgloss.JoinHorizontal(lipgloss.Top, l, spacer, r), f)
}

// gapSpacer renders a gap-wide, height-tall blank column so JoinHorizontal
// places a visible, counted gap between columns (JoinHorizontal inserts no
// spacing of its own). The spacer is plain spaces — a run of blanks carries no
// color, so it stays theme-neutral against any panel background. A zero gap or
// height renders nothing.
func gapSpacer(gap, h int) string {
	if gap <= 0 || h <= 0 {
		return ""
	}
	line := strings.Repeat(" ", gap)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = line
	}
	return joinLines(rows)
}

// StackV stacks already-rendered blocks vertically, allocating each a share of
// the height, and clamps the whole stack to the frame. Heights is the per-block
// row budget; a nil/short Heights splits evenly. Use it for compact breakpoints
// where panels stack instead of tiling.
func StackV(f Frame, blocks []string, heights []int) string {
	if f.Empty() || len(blocks) == 0 {
		return ""
	}
	if len(heights) != len(blocks) {
		each := f.H / len(blocks)
		heights = make([]int, len(blocks))
		for i := range heights {
			heights[i] = each
		}
		heights[len(heights)-1] = f.H - each*(len(blocks)-1)
	}
	var b strings.Builder
	for i, blk := range blocks {
		h := heights[i]
		if h < 1 {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(clampBlock(blk, Frame{W: f.W, H: h}))
	}
	return clampBlock(b.String(), f)
}
