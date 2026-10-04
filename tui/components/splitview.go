package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SplitView composes two already-rendered child panes side by side (or stacked)
// with a configurable ratio. In compact mode it collapses to a single pane with
// a toggle (design §3, §6). It is purely geometric: callers render each child
// into the Frame SplitView computes for it.
type SplitView struct {
	// Ratio is the left pane's share of the width, 0..1 (default 0.5).
	Ratio float64
	// Compact collapses to a single pane; Showing selects which pane is shown.
	Compact bool
	Showing int // 0 = left/first, 1 = right/second
}

// Frames returns the left/right child frames for a given outer frame. In
// compact mode one frame is the full area and the other is empty.
func (s SplitView) Frames(f Frame) (left, right Frame) {
	if f.Empty() {
		return Frame{}, Frame{}
	}
	if s.Compact {
		if s.Showing == 1 {
			return Frame{}, f
		}
		return f, Frame{}
	}
	ratio := s.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	leftW := int(float64(f.W) * ratio)
	if leftW < 1 {
		leftW = 1
	}
	rightW := f.W - leftW
	if rightW < 0 {
		rightW = 0
	}
	left = Frame{X: f.X, Y: f.Y, W: leftW, H: f.H}
	right = Frame{X: f.X + leftW, Y: f.Y, W: rightW, H: f.H}
	return left, right
}

// Join renders the two child strings into the outer frame using the computed
// frames, so the result never overflows.
func (s SplitView) Join(f Frame, leftContent, rightContent string) string {
	if f.Empty() {
		return ""
	}
	lf, rf := s.Frames(f)
	if s.Compact {
		if s.Showing == 1 {
			return clampBlock(rightContent, rf)
		}
		return clampBlock(leftContent, lf)
	}
	l := padHeight(clampBlock(leftContent, lf), f.H, lf.W)
	r := padHeight(clampBlock(rightContent, rf), f.H, rf.W)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, l, r)
	return clampBlock(joined, f)
}

// toggleHint is a tiny helper kept for symmetry with the compact toggle.
func toggleHint() string { return strings.TrimSpace("tab: toggle pane") }
