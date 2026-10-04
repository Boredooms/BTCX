package components

import (
	"strings"

	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// StreamLine is one append-only entry in the monitor stream.
type StreamLine struct {
	Timestamp string
	Kind      string // event / risk_delta / alert
	Text      string
	// Severity is set only for alert lines (presentation band).
	Severity string
}

// MonitorStream is an append-only, auto-scrolling log of a live or historical
// monitor session (design §3, §11.3). It honors cancellation: when the parent's
// session context is cancelled it stops appending (the parent simply stops
// feeding lines). The LIVE marker is the caller's responsibility and must only
// be shown while a MonitorWallet loop is actually running (AGENTS §16).
type MonitorStream struct {
	styles theme.Styles
	lines  []StreamLine
	follow bool // auto-scroll to newest
	live   bool
	cap    int
}

// NewMonitorStream builds an empty stream with a bounded line buffer.
func NewMonitorStream(styles theme.Styles) MonitorStream {
	return MonitorStream{styles: styles, follow: true, cap: 1000}
}

// SetLive marks the session live/ended. Only set true while MonitorWallet runs.
func (m *MonitorStream) SetLive(live bool) { m.live = live }

// Append adds a line, trimming to the bounded buffer (oldest dropped).
func (m *MonitorStream) Append(l StreamLine) {
	m.lines = append(m.lines, l)
	if len(m.lines) > m.cap {
		m.lines = m.lines[len(m.lines)-m.cap:]
	}
}

// ToggleFollow flips auto-scroll (space key).
func (m *MonitorStream) ToggleFollow() { m.follow = !m.follow }

// Lines returns the current buffered lines (for tests).
func (m MonitorStream) Lines() []StreamLine { return m.lines }

// Init implements the sub-model contract.
func (m MonitorStream) Init() tea.Cmd { return nil }

// Update handles the follow toggle.
func (m MonitorStream) Update(msg tea.Msg) (MonitorStream, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == " " {
		m.follow = !m.follow
	}
	return m, nil
}

func (m MonitorStream) marker() string {
	if m.live {
		return m.styles.Role(theme.RoleHealthy).Render("●LIVE")
	}
	return m.styles.Role(theme.RoleLabel).Render("●ENDED")
}

// View renders the newest lines that fit, with the LIVE/ENDED marker header.
func (m MonitorStream) View(f Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.marker())
	b.WriteString("\n")

	rows := f.H - 1
	if rows < 1 {
		rows = 1
	}
	start := 0
	if len(m.lines) > rows {
		start = len(m.lines) - rows // auto-scroll to newest
	}
	for _, l := range m.lines[start:] {
		line := m.styles.Role(theme.RoleLabel).Render(l.Timestamp) + " " +
			m.styles.Role(theme.RoleInfo).Render(l.Kind) + " " +
			m.styles.Role(theme.RoleValue).Render(l.Text)
		if l.Kind == "alert" && l.Severity != "" {
			line += " " + m.styles.SeverityChip(l.Severity, "["+l.Severity+"]")
		}
		b.WriteString(clampLine(line, f.W) + "\n")
	}
	return clampBlock(b.String(), f)
}
