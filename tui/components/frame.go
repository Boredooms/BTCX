// Package components holds the reusable, self-contained Bubble Tea sub-models
// the BCTX TUI composes (design §3). Each component owns only its own ephemeral
// presentation state, exposes Model/Init/Update/View(Frame), and renders into
// an allocated Frame without overflowing it. Components run no SQL, dial no
// network, and compute no risk — they only render data handed to them
// (AGENTS §12).
package components

import "github.com/charmbracelet/lipgloss"

// Frame is an allocated rectangle in terminal cells. The shell computes Frames
// and passes them down; a component must clamp/truncate to its Frame rather
// than overflow (design §6). It lives here (the leaf package) so tui and every
// component share one type with no import cycle.
type Frame struct {
	X, Y, W, H int
}

// Empty reports whether the frame has no drawable area.
func (f Frame) Empty() bool { return f.W <= 0 || f.H <= 0 }

// lipglossWidth is a thin alias so component files read naturally.
func lipglossWidth(s string) int { return lipgloss.Width(s) }

// clampLine truncates a single line to at most w display cells using lipgloss
// width accounting so wide runes do not overflow the frame.
func clampLine(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Trim rune-by-rune until it fits; cheap and correct for our short lines.
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > w {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

// clampBlock clamps a multi-line block to the frame: each line is truncated to
// the width and the whole block is capped to the height. This is the single
// overflow guard every component's View routes through.
func clampBlock(s string, f Frame) string {
	if f.Empty() {
		return ""
	}
	lines := splitLines(s)
	if len(lines) > f.H {
		lines = lines[:f.H]
	}
	for i := range lines {
		lines[i] = clampLine(lines[i], f.W)
	}
	return joinLines(lines)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
