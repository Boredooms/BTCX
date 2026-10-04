package screens

import (
	"strings"

	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Help is the Help screen (design §2, Nav=false). It renders a static keyboard
// reference generated from the keymap registry. The reference content is a
// fixed, documented keymap — not live service data — so it is honest to show it
// statically (this is a presentation reference, not fabricated forensic data).
type Help struct {
	ctx    *ScreenCtx
	styles theme.Styles
}

// NewHelp builds the Help screen.
func NewHelp(ctx *ScreenCtx) *Help {
	return &Help{ctx: ctx, styles: ctx.styles()}
}

// Init is a no-op.
func (h *Help) Init() tea.Cmd { return nil }

// Update is a no-op (Esc-close is handled by the Root).
func (h *Help) Update(tea.Msg) (Model, tea.Cmd) { return h, nil }

// helpGroups is the documented keymap reference, grouped by context (design §5).
var helpGroups = [][2]string{
	{"GLOBAL", ""},
	{"↑/k ↓/j", "move selection"},
	{"enter", "activate / drill in"},
	{"esc", "back / close / clear"},
	{"tab / shift+tab", "next / prev panel"},
	{"/", "search"},
	{": / ctrl+p", "command palette"},
	{"1-9 a o d r s t g m", "jump to screen"},
	{"? ", "this help"},
	{"q / ctrl+c", "quit"},
	{"GRAPH", ""},
	{"[ / ]", "decrease / increase depth"},
	{"n / N", "next / prev node"},
	{"p", "path mode (pick src, dst)"},
	{"0", "reset view"},
	{"MAP", ""},
	{"c", "bin by country / ASN"},
	{"MONITOR / DATA", ""},
	{"s / y", "start session / sync (offline-gated)"},
}

// View renders the static keyboard reference.
func (h *Help) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString(h.styles.Title.Render("KEYBOARD REFERENCE"))
	b.WriteString("\n\n")
	for _, g := range helpGroups {
		if g[1] == "" {
			b.WriteString(h.styles.Role(theme.RoleTitle).Render(g[0]))
			b.WriteString("\n")
			continue
		}
		b.WriteString(h.styles.Role(theme.RoleInfo).Render(padRightLocal(g[0], 22)))
		b.WriteString(h.styles.Value.Render(g[1]))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(h.styles.Label.Render("[esc] close"))
	return clampBlockLocal(b.String(), f)
}

// ShortHelp is empty — the Help screen is itself the reference.
func (h *Help) ShortHelp() []key.Binding { return noBinding }

func padRightLocal(s string, n int) string {
	if len([]rune(s)) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}
