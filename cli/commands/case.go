package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/cases/manager"
	"github.com/bctx/bctx/configs"
)

func newCaseCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "case",
		Short: "Manage isolated investigation cases",
	}
	cmd.AddCommand(
		newCaseCreateCmd(gf),
		newCaseListCmd(gf),
		newCaseOpenCmd(gf),
		newCaseCloseCmd(gf),
	)
	return cmd
}

func caseManager(gf *globalFlags) (*manager.Manager, error) {
	cfg, err := loadConfig(gf)
	if err != nil {
		return nil, err
	}
	layout := configs.NewLayout(cfg)
	if err := layout.EnsureAll(); err != nil {
		return nil, err
	}
	return manager.New(layout), nil
}

func newCaseCreateCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new isolated case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := caseManager(gf)
			if err != nil {
				return err
			}
			c, err := cm.Create(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created and opened case %q (id: %s)\n", c.Name, c.ID)
			return nil
		},
	}
}

func newCaseListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := caseManager(gf)
			if err != nil {
				return err
			}
			cases, err := cm.List(cmd.Context())
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(cases) == 0 {
				fmt.Fprintln(out, "No cases. Create one with 'bctx case create <name>'.")
				return nil
			}
			active, _ := cm.Active()
			fmt.Fprintf(out, "%-20s %-10s %-8s %s\n", "ID", "STATUS", "ACTIVE", "CREATED")
			for _, c := range cases {
				mark := ""
				if c.ID == active.ID {
					mark = "*"
				}
				fmt.Fprintf(out, "%-20s %-10s %-8s %s\n",
					c.ID, c.Status, mark, c.CreatedAt.Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
}

func newCaseOpenCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "open <name>",
		Short: "Open (activate) an existing case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := caseManager(gf)
			if err != nil {
				return err
			}
			c, err := cm.Open(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Opened case %q (id: %s)\n", c.Name, c.ID)
			return nil
		},
	}
}

func newCaseCloseCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "close <name>",
		Short: "Mark a case closed (data is preserved)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cm, err := caseManager(gf)
			if err != nil {
				return err
			}
			if err := cm.Close(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Closed case %q\n", args[0])
			return nil
		},
	}
}
