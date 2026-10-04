package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/configs"
)

func newInitCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize local BCTX directories and configuration (offline-safe)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			layout := configs.NewLayout(cfg)
			if err := layout.EnsureAll(); err != nil {
				return err
			}
			// Persist config if it does not yet exist on disk.
			if cfg.Path() == "" {
				cfg = configs.Default()
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Initialized BCTX runtime.")
			fmt.Fprintf(out, "  Config: %s\n", cfg.Path())
			fmt.Fprintf(out, "  Root:   %s\n", layout.Root)
			fmt.Fprintf(out, "  Cases:  %s\n", layout.Cases)
			fmt.Fprintf(out, "  Models: %s\n", layout.Models)
			fmt.Fprintln(out, "Run 'bctx doctor' to verify readiness.")
			return nil
		},
	}
}
