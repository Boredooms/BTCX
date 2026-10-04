package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/acquisition"
)

// newProviderCmd implements `bctx provider list` — local metadata only, never
// requires the network.
func newProviderCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "provider", Short: "Acquisition provider metadata"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List configured/compiled providers and capabilities",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			// Build the configured provider purely to read its local metadata.
			src, err := providerFromConfig(cfg)
			if err != nil {
				return err
			}
			caps := src.Capabilities()
			info := map[string]any{
				"name":         src.ProviderName(),
				"version":      src.ProviderVersion(),
				"configured":   cfg.Acquisition.Provider,
				"capabilities": caps,
			}
			if gf.jsonOut {
				b, _ := json.MarshalIndent(info, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%-10s %-10s %-8s %-8s %-6s %-6s %s\n",
				"NAME", "VERSION", "WALLET", "TX", "PAGE", "RATE", "AUTH")
			fmt.Fprintf(out, "%-10s %-10s %-8v %-8v %-6v %-6v %v\n",
				src.ProviderName(), src.ProviderVersion(),
				caps.WalletHistory, caps.TransactionFetch, caps.Pagination,
				caps.RateLimited, caps.AuthRequired)
			return nil
		},
	})
	return cmd
}

var _ = acquisition.NewFakeProvider
