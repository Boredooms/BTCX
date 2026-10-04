package commands

import (
	"github.com/spf13/cobra"
)

// newTuiCmd adds the explicit `bctx tui` command. It launches the same
// investigator TUI that bare `bctx` launches on a TTY, through the one shared
// app-seam wiring path (launchTUI). Off a TTY it prints the same plain status
// summary, so scripting `bctx tui | cat` stays well-behaved. It never changes
// the behavior or output of any other subcommand.
func newTuiCmd(gf *globalFlags, info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive investigator terminal UI",
		Long: "Launch the BCTX investigator TUI: a keyboard-first, offline-first " +
			"forensic workstation over the local case. On a non-TTY it prints a " +
			"plain status summary instead.",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			return launchTUI(cmd, gf, cfg, info)
		},
	}
}
