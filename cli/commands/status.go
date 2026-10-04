package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/screens"
)

func newStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show network, case, data, model and graph status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			engine, cases, cleanup, err := buildEngine(cfg)
			if err != nil {
				return err
			}
			defer cleanup()

			ctx := cmd.Context()
			netStatus := engine.NetworkStatus(ctx)

			// Real ML-manifest status (LOADED / SCHEMA-MISMATCH / MISSING),
			// resolved through the same registry the TUI uses — never the stale
			// hard-coded "PENDING" the CLI used to print.
			modelsStatus := screens.ModelsStatus(cfg)

			caseName := "(none)"
			if active, ok := cases.Active(); ok {
				caseName = active.ID
			}

			var counts sdk.Counts
			if engine.Repo != nil {
				counts, _ = engine.Repo.Counts(ctx)
			}

			if gf.jsonOut {
				b, _ := json.MarshalIndent(map[string]any{
					"network":      string(netStatus),
					"case":         caseName,
					"transactions": counts.Transactions,
					"wallets":      counts.Wallets,
					"network_recs": counts.NetworkRecs,
					"edges":        counts.Edges,
					"alerts":       counts.Alerts,
					"models":       modelsStatus,
					"graph":        graphState(counts.Edges),
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "BCTX — Bitcoin Forensic Intelligence Platform")
			fmt.Fprintln(out)
			fmt.Fprintf(out, "Network:       %s\n", netStatus)
			fmt.Fprintf(out, "Case:          %s\n", caseName)
			fmt.Fprintf(out, "Transactions:  %d\n", counts.Transactions)
			fmt.Fprintf(out, "Wallets:       %d\n", counts.Wallets)
			fmt.Fprintf(out, "Network recs:  %d\n", counts.NetworkRecs)
			fmt.Fprintf(out, "Graph edges:   %d\n", counts.Edges)
			fmt.Fprintf(out, "Alerts:        %d\n", counts.Alerts)
			fmt.Fprintf(out, "Models:        %s\n", modelsStatus)
			fmt.Fprintf(out, "Graph:         %s\n", graphState(counts.Edges))
			return nil
		},
	}
}

func graphState(edges int) string {
	if edges > 0 {
		return "READY"
	}
	return "EMPTY"
}
