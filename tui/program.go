package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// NewProgram builds the Bubble Tea program for the BCTX TUI over the Root model.
// Callers pass tea.WithContext(ctx) so the program (and every service command
// derived from the Root's ctx) is cancelled on quit — no goroutine outlives the
// program (design §11.3). It is the single entry point the CLI (runHome / the
// `bctx tui` command) uses to launch the interactive UI.
func NewProgram(root *Root, opts ...tea.ProgramOption) *tea.Program {
	return tea.NewProgram(root, opts...)
}
