package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// This file declares the tea.Msg vocabulary the TUI uses to move data and
// events between the async service commands (commands.go) and the UI thread
// (design §11). Service results NEVER arrive by blocking a View/Update; they
// arrive as one of these messages.

// NotifyLevel classifies a user-facing notification.
type NotifyLevel string

const (
	// NotifyInfo is neutral information (e.g. "report exported").
	NotifyInfo NotifyLevel = "info"
	// NotifySuccess is a confirmed success (e.g. "sync ok").
	NotifySuccess NotifyLevel = "success"
	// NotifyWarning is a non-fatal warning (e.g. "models missing").
	NotifyWarning NotifyLevel = "warning"
	// NotifyError is a failure surfaced to the operator.
	NotifyError NotifyLevel = "error"
)

// DataLoaded carries a successful service result back to the screen that
// requested it. Payload is the screen-specific result (e.g. *schema.Subgraph,
// *schema.InvestigationResult, []schema.Alert); the receiving screen type
// asserts it. Screen identifies the requesting screen so stale results from a
// superseded/cancelled request can be dropped.
type DataLoaded struct {
	Screen  ScreenID
	Request string // optional correlation key (e.g. the subject queried)
	Payload any
}

// DataError carries a failed service result. Err is surfaced in the ContextBar
// (one line) and expandable in a Modal; it is never silently swallowed
// (design §11.1, AGENTS §19).
type DataError struct {
	Screen  ScreenID
	Request string
	Err     error
}

// NavMsg requests a navigation to another screen. The root pushes the current
// screen onto the nav stack and swaps in the target built from the registry
// (design §4). Subject carries an optional selected subject to set on arrival.
type NavMsg struct {
	To      ScreenID
	Subject Subject
}

// NavBackMsg pops the navigation stack (Esc-back) without a specific target.
type NavBackMsg struct{}

// Notification raises a transient toast into the root notification queue
// (design §11.1). It never steals focus.
type Notification struct {
	Level NotifyLevel
	Text  string
	At    time.Time
}

// TickMsg is the 1s clock tick that refreshes the TopBar time field (§8). It is
// distinct from bubbletea's internal timers so the root can ignore stale ticks.
type TickMsg struct {
	Time time.Time
}

// Subject is the globally selected investigation subject. Kind constrains how
// screens interpret ID.
type Subject struct {
	ID   string
	Kind SubjectKind
}

// SubjectKind enumerates what a selected subject refers to.
type SubjectKind string

const (
	// SubjectNone means no subject is selected.
	SubjectNone SubjectKind = ""
	// SubjectWallet is a wallet/address subject.
	SubjectWallet SubjectKind = "wallet"
	// SubjectTx is a transaction subject.
	SubjectTx SubjectKind = "tx"
	// SubjectIP is a network/IP subject.
	SubjectIP SubjectKind = "ip"
	// SubjectEntity is an entity-cluster subject.
	SubjectEntity SubjectKind = "entity"
	// SubjectBlock is a block subject (hash or height).
	SubjectBlock SubjectKind = "block"
)

// Empty reports whether no subject is selected.
func (s Subject) Empty() bool { return s.Kind == SubjectNone || s.ID == "" }

// tickCmd schedules the next 1-second clock tick.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{Time: t}
	})
}

// notifyCmd raises a notification as a one-shot command.
func notifyCmd(level NotifyLevel, text string) tea.Cmd {
	return func() tea.Msg {
		return Notification{Level: level, Text: text, At: time.Now()}
	}
}
