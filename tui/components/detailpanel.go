package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Row is a labeled key/value pair in a DetailPanel section.
type Row struct {
	Label string
	Value string
	Role  theme.Role // optional; defaults to RoleValue
}

// Section is a titled group of rows (e.g. "signals", "related wallets").
type Section struct {
	Title string
	Rows  []Row
	// Body is free-form text appended under the rows (e.g. a description).
	Body string
}

// DetailPanel renders labeled key/value sections for a subject inside a
// scrollable viewport (design §3). It holds no analysis logic; the parent hands
// it already-formatted sections built from an InvestigationResult/row.
type DetailPanel struct {
	styles   theme.Styles
	vp       viewport.Model
	sections []Section
	state    DataState
	errMsg   string
	ready    bool
}

// NewDetailPanel builds an empty DetailPanel.
func NewDetailPanel(styles theme.Styles) DetailPanel {
	return DetailPanel{styles: styles, state: StateLoading}
}

// SetLoading / SetError move the panel into an explicit state.
func (d *DetailPanel) SetLoading()         { d.state = StateLoading }
func (d *DetailPanel) SetError(msg string) { d.state, d.errMsg = StateError, msg }

// SetSections loads the content. An empty set moves to the empty state.
func (d *DetailPanel) SetSections(secs []Section) {
	d.sections = secs
	if len(secs) == 0 {
		d.state = StateEmpty
		return
	}
	d.state = StateLoaded
}

// State returns the current state.
func (d DetailPanel) State() DataState { return d.state }

// Init implements the sub-model contract.
func (d DetailPanel) Init() tea.Cmd { return nil }

// Update forwards scroll keys to the viewport.
func (d DetailPanel) Update(msg tea.Msg) (DetailPanel, tea.Cmd) {
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return d, cmd
}

// render builds the plain content string for the current sections, wrapping
// every line to the available width so long values and free-form Body text wrap
// cleanly on word boundaries instead of being hard-broken mid-word by the
// viewport (which was clipping "...ownership." to "observati/o/ownership").
func (d DetailPanel) render(width int) string {
	if width < 1 {
		width = 1
	}
	var b strings.Builder
	for i, s := range d.sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(clampLine(d.styles.Role(theme.RoleTitle).Render("── "+s.Title+" ──"), width))
		b.WriteString("\n")
		// Label column is fixed; the value gets the remaining width and wraps,
		// with continuation lines indented under the value column.
		labelW := 18
		if labelW > width-2 {
			labelW = width - 2
		}
		if labelW < 1 {
			labelW = 1
		}
		valueW := width - labelW - 1
		if valueW < 1 {
			valueW = 1
		}
		for _, r := range s.Rows {
			role := r.Role
			if role == "" {
				role = theme.RoleValue
			}
			label := d.styles.Role(theme.RoleLabel).Render(padRight(r.Label, labelW))
			wrapped := wrapText(r.Value, valueW)
			for j, vline := range wrapped {
				if j == 0 {
					b.WriteString(label + " " + d.styles.Role(role).Render(vline) + "\n")
				} else {
					b.WriteString(strings.Repeat(" ", labelW+1) + d.styles.Role(role).Render(vline) + "\n")
				}
			}
		}
		if strings.TrimSpace(s.Body) != "" {
			for _, bl := range wrapText(s.Body, width) {
				b.WriteString(d.styles.Role(theme.RoleValue).Render(bl) + "\n")
			}
		}
	}
	return b.String()
}

// wrapText wraps a plain (unstyled) string to width on word boundaries, hard-
// splitting any single token longer than width. Returns at least one line.
func wrapText(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			// Hard-split a token longer than the whole width.
			for len(w) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, w[:width])
				w = w[width:]
			}
			switch {
			case line == "":
				line = w
			case len(line)+1+len(w) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// View renders the current state within the frame.
func (d DetailPanel) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	switch d.state {
	case StateLoading:
		return clampBlock(d.styles.Role(theme.RoleInfo).Render("querying…"), f)
	case StateEmpty:
		return clampBlock(d.styles.Role(theme.RoleLabel).Render("no detail"), f)
	case StateError:
		return clampBlock(d.styles.Role(theme.RoleCritical).Render("error: "+d.errMsg), f)
	default:
		vp := viewport.New(f.W, f.H)
		vp.SetContent(d.render(f.W))
		return clampBlock(vp.View(), f)
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
