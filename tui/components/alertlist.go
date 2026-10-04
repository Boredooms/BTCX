package components

import (
	"fmt"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// AlertList renders schema.Alert rows with severity glyphs/colors (design §3).
// Severity here is the presentation band derived from the alert's risk score
// (design §10) — it never asserts a crime (AGENTS §18). Rows are supplied by a
// bounded ListAlerts read; the component fetches nothing.
type AlertList struct {
	styles   theme.Styles
	alerts   []schema.Alert
	selected int
	state    DataState
	errMsg   string
}

// NewAlertList builds an empty AlertList.
func NewAlertList(styles theme.Styles) AlertList {
	return AlertList{styles: styles, state: StateLoading}
}

// SetLoading / SetError move the list into an explicit state.
func (a *AlertList) SetLoading()         { a.state = StateLoading }
func (a *AlertList) SetError(msg string) { a.state, a.errMsg = StateError, msg }

// SetAlerts loads alert rows. An empty set moves to the empty state.
func (a *AlertList) SetAlerts(alerts []schema.Alert) {
	a.alerts = alerts
	a.selected = 0
	if len(alerts) == 0 {
		a.state = StateEmpty
		return
	}
	a.state = StateLoaded
}

// State returns the current data state.
func (a AlertList) State() DataState { return a.state }

// SelectedAlert returns the currently highlighted alert and true, or a zero
// Alert and false when the list is empty / not loaded. The caller (the
// Dashboard) uses it to open the alert's subject on Enter.
func (a AlertList) SelectedAlert() (schema.Alert, bool) {
	if a.state != StateLoaded || a.selected < 0 || a.selected >= len(a.alerts) {
		return schema.Alert{}, false
	}
	return a.alerts[a.selected], true
}

func (a AlertList) severityGlyph(sev string) string {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return "!"
	case "HIGH":
		return "▲"
	default:
		return "·"
	}
}

// Init implements the sub-model contract.
func (a AlertList) Init() tea.Cmd { return nil }

// Update handles up/down selection.
func (a AlertList) Update(msg tea.Msg) (AlertList, tea.Cmd) {
	if a.state != StateLoaded {
		return a, nil
	}
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if a.selected > 0 {
				a.selected--
			}
		case "down", "j":
			if a.selected < len(a.alerts)-1 {
				a.selected++
			}
		}
	}
	return a, nil
}

// View renders the alert rows within the frame.
func (a AlertList) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	switch a.state {
	case StateLoading:
		return clampBlock(a.styles.Role(theme.RoleInfo).Render("querying…"), f)
	case StateEmpty:
		return clampBlock(a.styles.Role(theme.RoleLabel).Render("no alerts in this case"), f)
	case StateError:
		return clampBlock(a.styles.Role(theme.RoleCritical).Render("error: "+a.errMsg), f)
	}
	var lines []string
	for i, al := range a.alerts {
		sev := severityFromRisk(al)
		marker := "  "
		if i == a.selected {
			marker = "▸ "
		}
		// One compact row per alert, columns adapted to the frame width so it
		// never wraps onto a second line. Wide: severity + subject + type +
		// status + risk; narrow: drop the type and status words. Final truncate
		// to the frame width guards any residual overflow.
		var row string
		if f.W >= 56 {
			row = fmt.Sprintf("%s%s %-9s %-12s %-16s %-5s r%3d",
				marker, a.severityGlyph(sev), sev, shortID(al.Subject), al.Type, string(al.Status), al.Risk)
		} else {
			row = fmt.Sprintf("%s%s %-9s %-12s r%3d",
				marker, a.severityGlyph(sev), sev, shortID(al.Subject), al.Risk)
		}
		row = truncatePlain(row, f.W)
		lines = append(lines, a.styles.SeverityChip(sev, row))
	}
	if len(lines) > f.H {
		lines = lines[:f.H]
	}
	return clampBlock(strings.Join(lines, "\n"), f)
}

// truncatePlain cuts an UNSTYLED string to at most w display cells, appending an
// ellipsis when it had to cut. It operates on runes (no ANSI), so callers must
// pass plain text before applying any lipgloss style.
func truncatePlain(s string, w int) string {
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
