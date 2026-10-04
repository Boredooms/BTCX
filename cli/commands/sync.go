package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/sdk"
)

// providerFromConfig builds the configured DataSource. Only the deterministic
// fake provider is implemented in this phase; a real explorer adapter plugs in
// here behind the same interface without changing storage/graph/ML/risk.
func providerFromConfig(cfg *configs.Config) (acquisition.DataSource, error) {
	// Delegate to the shared factory (supports esplora + fake).
	return monitorProvider(cfg)
}

func engineConfig(cfg *configs.Config) acquisition.Config {
	ec := acquisition.DefaultConfig()
	a := cfg.Acquisition
	if a.MaxWorkers > 0 {
		ec.MaxWorkers = a.MaxWorkers
	}
	if a.PageSize > 0 {
		ec.PageSize = a.PageSize
	}
	if a.BatchSize > 0 {
		ec.BatchSize = a.BatchSize
	}
	if d, err := time.ParseDuration(a.RequestTimeout); err == nil {
		ec.RequestTimeout = d
	}
	if a.MaxRetries > 0 {
		ec.Retry.MaxAttempts = a.MaxRetries
	}
	if d, err := time.ParseDuration(a.InitialBackoff); err == nil {
		ec.Retry.InitialBackoff = d
	}
	if d, err := time.ParseDuration(a.MaxBackoff); err == nil {
		ec.Retry.MaxBackoff = d
	}
	ec.RequestsPerSecond = a.RequestsPerSecond
	return ec
}

func newSyncCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sync", Short: "Acquire wallet/transaction data from a provider"}
	cmd.AddCommand(
		newSyncWalletCmd(gf),
		newSyncTxCmd(gf),
		newSyncBlockCmd(gf),
		newSyncStatusCmd(gf),
		newSyncListCmd(gf),
	)
	return cmd
}

// acquisitionAllowed enforces the offline/airgap boundary before any network use.
func acquisitionAllowed(cfg *configs.Config, gf *globalFlags) error {
	if gf.offline || gf.airgap || !cfg.AcquisitionAllowed() {
		return acquisition.ErrOfflineAcquisition
	}
	return nil
}

func printSyncStats(cmd *cobra.Command, gf *globalFlags, st acquisition.Stats) {
	if gf.jsonOut {
		b, _ := json.MarshalIndent(map[string]any{
			"sync_id": st.SyncID, "provider": st.Provider, "target": st.Target,
			"target_type": st.TargetType, "status": st.Status, "pages": st.Pages,
			"discovered": st.Discovered, "fetched": st.Fetched, "new": st.New,
			"duplicates": st.Duplicates, "partial": st.Partial,
			"rejected": st.Rejected, "retries": st.Retries,
			"elapsed_ms": st.Elapsed.Milliseconds(),
		}, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "PROVIDER:   %s\n", st.Provider)
	fmt.Fprintf(out, "TARGET:     %s (%s)\n", st.Target, st.TargetType)
	fmt.Fprintf(out, "STATUS:     %s\n", st.Status)
	fmt.Fprintf(out, "PAGES:      %d\n", st.Pages)
	fmt.Fprintf(out, "DISCOVERED: %d\n", st.Discovered)
	fmt.Fprintf(out, "FETCHED:    %d\n", st.Fetched)
	fmt.Fprintf(out, "NEW:        %d\n", st.New)
	fmt.Fprintf(out, "DUPLICATES: %d\n", st.Duplicates)
	fmt.Fprintf(out, "PARTIAL:    %d\n", st.Partial)
	fmt.Fprintf(out, "REJECTED:   %d\n", st.Rejected)
	fmt.Fprintf(out, "RETRIES:    %d\n", st.Retries)
	fmt.Fprintf(out, "ELAPSED:    %s\n", st.Elapsed.Round(time.Millisecond))
}

func newSyncWalletCmd(gf *globalFlags) *cobra.Command {
	var resume bool
	c := &cobra.Command{
		Use:   "wallet <address>",
		Short: "Acquire a wallet's history from the configured provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			if err := acquisitionAllowed(cfg, gf); err != nil {
				return err
			}
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				active, _ := caseManagerActive(gf)
				src, err := providerFromConfig(cfg)
				if err != nil {
					return err
				}
				eng := acquisition.NewEngine(src, repo, active, engineConfig(cfg))
				defer eng.Close()

				cursor := acquisition.PaginationState{}
				if resume {
					cp, _ := repo.GetSyncCheckpoint(cmd.Context(), "sync-wallet-"+truncate(args[0], 16))
					if cp != nil {
						if cp.Target != args[0] || cp.Provider != src.ProviderName() {
							return acquisition.ErrSyncCheckpointConflict
						}
						cursor = acquisition.PaginationState{Cursor: cp.Cursor, Page: cp.CursorPage}
					}
				}
				st, err := eng.SyncWallet(cmd.Context(), args[0], cursor)
				if err != nil {
					return err
				}
				printSyncStats(cmd, gf, st)
				return nil
			})
		},
	}
	c.Flags().BoolVar(&resume, "resume", false, "resume from the last checkpoint")
	return c
}

func newSyncTxCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tx <txid>",
		Short: "Acquire a single transaction from the configured provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			if err := acquisitionAllowed(cfg, gf); err != nil {
				return err
			}
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				active, _ := caseManagerActive(gf)
				src, err := providerFromConfig(cfg)
				if err != nil {
					return err
				}
				eng := acquisition.NewEngine(src, repo, active, engineConfig(cfg))
				defer eng.Close()
				st, err := eng.SyncTransaction(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				printSyncStats(cmd, gf, st)
				return nil
			})
		},
	}
}

func newSyncStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <sync-id>",
		Short: "Show a sync checkpoint status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				cp, err := repo.GetSyncCheckpoint(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if cp == nil {
					return fmt.Errorf("no sync %q", args[0])
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(cp, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Sync:       %s\n", cp.SyncID)
				fmt.Fprintf(out, "Provider:   %s %s\n", cp.Provider, cp.ProviderVersion)
				fmt.Fprintf(out, "Target:     %s (%s)\n", cp.Target, cp.TargetType)
				fmt.Fprintf(out, "Status:     %s\n", cp.Status)
				fmt.Fprintf(out, "Pages:      %d\n", cp.PagesDone)
				fmt.Fprintf(out, "Discovered: %d\n", cp.Discovered)
				fmt.Fprintf(out, "Persisted:  %d\n", cp.Persisted)
				fmt.Fprintf(out, "Duplicates: %d\n", cp.Duplicates)
				fmt.Fprintf(out, "Retries:    %d\n", cp.Retries)
				if cp.Error != "" {
					fmt.Fprintf(out, "Error:      %s\n", cp.Error)
				}
				return nil
			})
		},
	}
}

func newSyncListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List sync checkpoints",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				cps, err := repo.ListSyncCheckpoints(cmd.Context())
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if len(cps) == 0 {
					fmt.Fprintln(out, "No syncs.")
					return nil
				}
				fmt.Fprintf(out, "%-28s %-8s %-10s %-10s %s\n", "SYNC", "PROVIDER", "STATUS", "PERSISTED", "TARGET")
				for _, cp := range cps {
					fmt.Fprintf(out, "%-28s %-8s %-10s %-10d %s\n",
						cp.SyncID, cp.Provider, cp.Status, cp.Persisted, cp.Target)
				}
				return nil
			})
		},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
