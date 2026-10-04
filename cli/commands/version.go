package commands

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				b, _ := json.MarshalIndent(map[string]string{
					"version": info.Version,
					"commit":  info.Commit,
					"build":   info.Date,
					"os":      runtime.GOOS,
					"arch":    runtime.GOARCH,
					"go":      runtime.Version(),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "BCTX — Bitcoin Forensic Intelligence Platform")
			fmt.Fprintf(out, "Version: %s\n", info.Version)
			fmt.Fprintf(out, "Commit:  %s\n", info.Commit)
			fmt.Fprintf(out, "Build:   %s\n", info.Date)
			fmt.Fprintf(out, "OS:      %s\n", runtime.GOOS)
			fmt.Fprintf(out, "Arch:    %s\n", runtime.GOARCH)
			fmt.Fprintf(out, "Go:      %s\n", runtime.Version())
			return nil
		},
	}
}
