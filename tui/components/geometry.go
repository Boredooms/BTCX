package components

import "github.com/bctx/bctx/tui/theme"

// geometry.go is the single width-budget authority for the cockpit layout
// (design §C). Every row of panels is divided by SplitH so border/padding/gap
// width is counted exactly once; PanelInner reports the content area a Panel
// exposes for a frame. Keeping both in the leaf components package lets
// cockpit.go call them while tui/layout.go delegates to the same math, so the
// reserved width and the rendered width can never disagree.

// minCol is the floor width a column must have before SplitH will emit it. A
// narrower request returns fewer columns so the caller can stack instead of
// drawing a box too small to close its border (design §C.2 rule 4).
const minCol = 12

// SplitH divides total into n columns separated by (n-1) gaps of gap cells
// each, returning the absolute width of every column. The widths sum to
// total-(n-1)*gap exactly; any integer remainder is handed to the last column
// so sum(cols)+(n-1)*gap == total. If total cannot give every column at least
// minCol after reserving the gaps, SplitH returns FEWER columns (as many as fit
// at the floor, remainder to the last), so a row never overflows and the caller
// stacks what did not fit. n<=0 returns nil; n==1 returns the whole width.
func SplitH(total, n, gap int) []int {
	if n <= 0 || total <= 0 {
		return nil
	}
	if gap < 0 {
		gap = 0
	}
	if n == 1 {
		return []int{total}
	}
	// Reduce the column count until each column clears the floor once the gaps
	// between the kept columns are reserved.
	for n > 1 {
		avail := total - (n-1)*gap
		if avail >= n*minCol {
			break
		}
		n--
	}
	if n == 1 {
		return []int{total}
	}
	avail := total - (n-1)*gap
	if avail < 0 {
		avail = 0
	}
	base := avail / n
	cols := make([]int, n)
	for i := range cols {
		cols[i] = base
	}
	// Remainder to the last column so sum(cols) == avail exactly.
	cols[n-1] = avail - base*(n-1)
	return cols
}

// PanelInner returns the inner content width and height a Panel exposes for an
// outer frame: the frame minus the panel's border + padding chrome. It mirrors
// cockpit.panelChromeH/V (2 border + 2*PanelPadX horizontal, 2 border +
// 2*PanelPadY vertical) by recomputing from the theme spacing tokens, so a
// caller can size body content without duplicating the chrome math. Values are
// clamped to 0 so a tight frame reports no negative inner area.
func PanelInner(f Frame) (w, h int) {
	w = f.W - (2 + 2*theme.Space.PanelPadX)
	h = f.H - (2 + 2*theme.Space.PanelPadY)
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w, h
}
