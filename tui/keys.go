package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Keymaps are declared once here (design §5) with bubbles/key so the
// HelpOverlay and ContextBar can be generated from the same bindings — a single
// source of truth. There is a global keymap plus per-context keymaps for the
// Graph, Map, Table, and Monitor screens.

// GlobalKeyMap holds the navigation/action bindings active on every screen
// unless a text input currently has focus (see Dispatch).
type GlobalKeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Back      key.Binding
	NextPanel key.Binding
	PrevPanel key.Binding
	Search    key.Binding
	Palette   key.Binding
	Help      key.Binding
	Quit      key.Binding
	// Jump holds the single-key screen jumps (registry Key fields). It is a
	// catch-all binding whose Keys() enumerate every registered nav key.
	Jump key.Binding
}

// NewGlobalKeyMap builds the global keymap. Jump keys are populated from the
// registry so help text and dispatch stay consistent with §2's table.
func NewGlobalKeyMap() GlobalKeyMap {
	return GlobalKeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev panel"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Palette: key.NewBinding(
			key.WithKeys(":", "ctrl+p"),
			key.WithHelp(":", "palette"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Jump: key.NewBinding(
			key.WithKeys(NavJumpKeys()...),
			key.WithHelp("1-9/a…", "jump"),
		),
	}
}

// ShortHelp returns the compact binding list shown in the ContextBar.
func (k GlobalKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Search, k.Palette, k.Help, k.Quit}
}

// FullHelp returns the full grid shown in the HelpOverlay.
func (k GlobalKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Back},
		{k.NextPanel, k.PrevPanel, k.Search, k.Palette},
		{k.Jump, k.Help, k.Quit},
	}
}

// GraphKeyMap adds the Graph-screen bindings (design §5).
type GraphKeyMap struct {
	ZoomIn, ZoomOut    key.Binding
	DepthDec, DepthInc key.Binding
	Focus, Filter      key.Binding
	Path               key.Binding
	NextNode, PrevNode key.Binding
	Reset              key.Binding
}

// NewGraphKeyMap builds the Graph keymap.
func NewGraphKeyMap() GraphKeyMap {
	return GraphKeyMap{
		ZoomIn:   key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "zoom in")),
		ZoomOut:  key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "zoom out")),
		DepthDec: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "depth-")),
		DepthInc: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "depth+")),
		Focus:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "focus")),
		Filter:   key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "filter type")),
		Path:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "path mode")),
		NextNode: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next node")),
		PrevNode: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev node")),
		Reset:    key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset")),
	}
}

// MapKeyMap adds the Geo Map bindings (design §5, §D.2). It advertises pan
// (arrows + hjkl), zoom, bin-cycle, the n/N selection move, Enter=open-detail
// and g=go (cross-link), and 0=reset so the HelpOverlay/ContextBar stay a single
// source of truth with MapView.Update. Note: Tab is deliberately NOT bound here
// — it is reserved globally for the Nav↔Body focus cycle (design §A.5) — and `g`
// is Body-local to the map screen only (never promoted to GlobalKeyMap).
type MapKeyMap struct {
	Pan             key.Binding
	ZoomIn, ZoomOut key.Binding
	CycleBin        key.Binding
	NextPoint       key.Binding
	PrevPoint       key.Binding
	OpenDetail      key.Binding
	Go              key.Binding
	Reset           key.Binding
}

// NewMapKeyMap builds the Map keymap.
func NewMapKeyMap() MapKeyMap {
	return MapKeyMap{
		Pan:        key.NewBinding(key.WithKeys("up", "down", "left", "right", "h", "j", "k", "l"), key.WithHelp("hjkl/arrows", "pan")),
		ZoomIn:     key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "zoom in")),
		ZoomOut:    key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "zoom out")),
		CycleBin:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "bin country/ASN")),
		NextPoint:  key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next point")),
		PrevPoint:  key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev point")),
		OpenDetail: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open detail")),
		Go:         key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "go to related tx/IP")),
		Reset:      key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset")),
	}
}

// TableKeyMap adds the Table bindings (design §5).
type TableKeyMap struct {
	Top, Bottom key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	Filter      key.Binding
	Open        key.Binding
}

// NewTableKeyMap builds the Table keymap.
func NewTableKeyMap() TableKeyMap {
	return TableKeyMap{
		Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
		Filter:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		Open:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open row")),
	}
}

// MonitorKeyMap adds the Monitoring bindings (design §5).
type MonitorKeyMap struct {
	PauseFollow key.Binding
	Stop        key.Binding
	Detail      key.Binding
}

// NewMonitorKeyMap builds the Monitor keymap.
func NewMonitorKeyMap() MonitorKeyMap {
	return MonitorKeyMap{
		PauseFollow: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause follow")),
		Stop:        key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop session")),
		Detail:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "event detail")),
	}
}

// Dispatch resolves a key message against the global keymap with focus-aware
// suppression. When inputFocused is true (a SearchBox/CommandPalette textinput
// is active), single-letter nav is suppressed so typing works; only Esc, Enter,
// and the palette toggle survive (design §5). It returns the matched binding's
// first key as the action token, or "" if nothing matched / is suppressed.
//
// Keeping this logic in one function means the AppShell checks focus in exactly
// one place and both the real update loop and tests exercise the same path.
func (k GlobalKeyMap) Dispatch(msg tea.KeyMsg, inputFocused bool) string {
	if inputFocused {
		// Only editing-safe global keys survive while an input is focused.
		switch {
		case key.Matches(msg, k.Back):
			return "esc"
		case key.Matches(msg, k.Enter):
			return "enter"
		default:
			return ""
		}
	}

	switch {
	case key.Matches(msg, k.Up):
		return "up"
	case key.Matches(msg, k.Down):
		return "down"
	case key.Matches(msg, k.Enter):
		return "enter"
	case key.Matches(msg, k.Back):
		return "esc"
	case key.Matches(msg, k.NextPanel):
		return "tab"
	case key.Matches(msg, k.PrevPanel):
		return "shift+tab"
	case key.Matches(msg, k.Search):
		return "search"
	case key.Matches(msg, k.Palette):
		return "palette"
	case key.Matches(msg, k.Help):
		return "help"
	case key.Matches(msg, k.Quit):
		return "quit"
	case key.Matches(msg, k.Jump):
		// Return the literal jump key so the caller can look it up in the registry.
		return "jump:" + msg.String()
	default:
		return ""
	}
}
