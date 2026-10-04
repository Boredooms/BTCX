package components

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/tui/theme"
)

// BarSample is one labeled value in a BarChart.
type BarSample struct {
	Label string  // short x-axis label (e.g. a time or index)
	Value float64 // magnitude
	Role  theme.Role
}

// BarChart renders a compact horizontal bar chart for long-horizon movement
// readouts (risk over time, event counts per poll). It is PRESENTATION ONLY:
// the caller supplies already-computed samples from real history (risk deltas /
// event counts); the chart invents no data and scales to the max sample.
type BarChart struct {
	styles theme.Styles
	title  string
	unit   string
}

// NewBarChart builds a bar chart with a title and a value unit suffix.
func NewBarChart(styles theme.Styles, title, unit string) BarChart {
	return BarChart{styles: styles, title: title, unit: unit}
}

// Sparkline renders a single-line unicode sparkline from values, scaled to the
// max. Useful as a dense inline trend in a header. Returns "" for no data.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	ramp := []rune("▁▂▃▄▅▆▇█")
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	for _, v := range values {
		idx := 0
		if max > 0 {
			idx = int(v / max * float64(len(ramp)-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= len(ramp) {
				idx = len(ramp) - 1
			}
		}
		b.WriteRune(ramp[idx])
	}
	return b.String()
}

// View renders the bar chart within the frame: a title row, then one row per
// sample (newest first is the caller's choice) as "label | ████ value", scaled
// to the frame width. Rows beyond the frame height are dropped honestly.
func (c BarChart) View(samples []BarSample, f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	if c.title != "" {
		b.WriteString(c.styles.Role(theme.RoleTitle).Render(c.title))
		b.WriteString("\n")
	}
	rows := f.H
	if c.title != "" {
		rows--
	}
	if len(samples) == 0 {
		b.WriteString(c.styles.Role(theme.RoleLabel).Render("no movement recorded yet"))
		return clampBlock(b.String(), f)
	}

	// Column widths: label (fixed), value (fixed), bar (remaining).
	labelW := 0
	for _, s := range samples {
		if len(s.Label) > labelW {
			labelW = len(s.Label)
		}
	}
	if labelW > 14 {
		labelW = 14
	}
	// Value column: the formatted value+unit, fixed to the widest sample so the
	// bar width is uniform and the line never wraps.
	valStrs := make([]string, len(samples))
	valW := 0
	for i, s := range samples {
		valStrs[i] = fmt.Sprintf("%.0f%s", s.Value, c.unit)
		if len(valStrs[i]) > valW {
			valW = len(valStrs[i])
		}
	}
	// label + space + bar + space + value, with a 1-cell safety margin so the
	// row never reaches the exact frame edge (which can wrap the trailing value).
	barW := f.W - labelW - valW - 4
	if barW < 1 {
		barW = 1
	}
	max := 0.0
	for _, s := range samples {
		if s.Value > max {
			max = s.Value
		}
	}
	shown := 0
	for i, s := range samples {
		if shown >= rows {
			break
		}
		filled := 0
		if max > 0 {
			filled = int(s.Value / max * float64(barW))
		}
		if filled < 0 {
			filled = 0
		}
		if filled > barW {
			filled = barW
		}
		role := s.Role
		if role == "" {
			role = theme.RoleInfo
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("·", barW-filled)
		line := fmt.Sprintf("%-*s %s %*s",
			labelW, truncateCells(s.Label, labelW),
			c.styles.Role(role).Render(bar),
			valW, valStrs[i])
		b.WriteString(clampLine(line, f.W))
		b.WriteString("\n")
		shown++
	}
	return clampBlock(b.String(), f)
}
