package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/blockchain/acquisition/explorer"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/investigation/orchestrator"
	"github.com/bctx/bctx/monitoring"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/risk/scoring"
	"github.com/bctx/bctx/sdk"
)

// monitorProvider builds the configured acquisition provider for monitoring.
// Supports the real Esplora provider and the deterministic fake.
func monitorProvider(cfg *configs.Config) (acquisition.DataSource, error) {
	switch cfg.Acquisition.Provider {
	case "esplora":
		opts := []explorer.Option{}
		if cfg.Network.ExplorerEndpoint != "" {
			opts = append(opts, explorer.WithEndpoint(cfg.Network.ExplorerEndpoint))
		}
		return explorer.New(opts...), nil
	case "fake", "":
		return acquisition.NewFakeProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Acquisition.Provider)
	}
}

// monitoringConfig maps config into the monitor engine config.
func monitoringConfig(cfg *configs.Config) monitoring.Config {
	mc := monitoring.DefaultConfig()
	m := cfg.Monitoring
	if d, err := time.ParseDuration(m.PollInterval); err == nil {
		mc.PollInterval = d
	}
	if m.MaxEventQueue > 0 {
		mc.MaxEventQueue = m.MaxEventQueue
	}
	if d, err := time.ParseDuration(m.AnalysisDebounce); err == nil {
		mc.AnalysisDebounce = d
	}
	if d, err := time.ParseDuration(m.ReconnectBackoff); err == nil {
		mc.ReconnectBackoff = d
	}
	if m.MaxReconnectAttempts > 0 {
		mc.MaxReconnects = m.MaxReconnectAttempts
	}
	if m.AlertRiskDelta > 0 {
		mc.AlertRiskDelta = m.AlertRiskDelta
	}
	mc.AlertNewSignal = m.AlertNewSignal
	mc.AlertNewPattern = m.AlertNewPattern
	return mc
}

// orchestratorAnalyzer wraps the investigation orchestrator as a
// monitoring.Analyzer (keeps the monitoring package free of orchestrator deps).
func orchestratorAnalyzer(repo sdk.Repository, caseID string) monitoring.Analyzer {
	orch := orchestrator.New(orchestrator.Options{
		Repo: repo, ModelsDir: repoModelsDir(),
		FeatureSchemaSHA: schema.FeatureSchemaSHA256, CaseID: caseID,
		Weights: scoring.DefaultWeights(),
	})
	return func(ctx context.Context, subject string) (monitoring.AnalysisResult, error) {
		res, err := orch.AnalyzeWallet(ctx, subject, true)
		if err != nil {
			return monitoring.AnalysisResult{}, err
		}
		ar := monitoring.AnalysisResult{
			Subject: subject, RiskScore: res.Risk.Score,
			Confidence: res.Risk.Confidence, RelatedWallet: res.RelatedWallets,
		}
		for _, s := range res.Risk.Signals {
			if s.Score > 0 {
				ar.Signals = append(ar.Signals, s.Name)
			}
		}
		for _, p := range res.Patterns {
			ar.Patterns = append(ar.Patterns, string(p.Type))
		}
		for _, e := range res.Evidence {
			ar.EvidenceIDs = append(ar.EvidenceIDs, e.ID)
		}
		return ar, nil
	}
}

func newMonitorCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "monitor", Short: "Live wallet monitoring"}
	cmd.AddCommand(
		newMonitorWalletCmd(gf),
		newMonitorStatusCmd(gf),
		newMonitorListCmd(gf),
		newMonitorLifecycleCmd(gf, "stop", "cancelled"),
		newMonitorLifecycleCmd(gf, "pause", "paused"),
		newMonitorResumeCmd(gf),
	)
	return cmd
}

func newMonitorWalletCmd(gf *globalFlags) *cobra.Command {
	var interval string
	var maxPolls int
	c := &cobra.Command{
		Use:   "wallet <address>",
		Short: "Monitor a wallet for new activity (requires network)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			if err := acquisitionAllowed(cfg, gf); err != nil {
				return err // offline refusal BEFORE provider construction
			}
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				active, _ := caseManagerActive(gf)
				src, err := monitorProvider(cfg)
				if err != nil {
					return err
				}
				if !src.Capabilities().WalletHistory {
					return acquisition.ErrCapabilityUnsupported
				}
				mcfg := monitoringConfig(cfg)
				if interval != "" {
					if d, derr := time.ParseDuration(interval); derr == nil {
						mcfg.PollInterval = d
					}
				}
				mcfg.MaxPolls = maxPolls // 0 = run until cancelled
				svc := monitoring.NewService(repo, src, orchestratorAnalyzer(repo, active), active, mcfg)
				sessID := "mon-" + truncate(args[0], 24)
				sess, rerr := svc.MonitorWallet(cmd.Context(), args[0], sessID)
				if rerr != nil && rerr != context.Canceled {
					// Cancellation is a normal stop; other errors surface.
					if cmd.Context().Err() == nil {
						return rerr
					}
				}
				printSession(cmd, gf, sess)
				return nil
			})
		},
	}
	c.Flags().StringVar(&interval, "interval", "", "poll interval (e.g. 10s)")
	c.Flags().IntVar(&maxPolls, "max-polls", 1, "bound the number of polls (0 = run until Ctrl+C)")
	return c
}

func printSession(cmd *cobra.Command, gf *globalFlags, s sdk.MonitorSessionRow) {
	if gf.jsonOut {
		b, _ := json.MarshalIndent(map[string]any{
			"session_id": s.SessionID, "case_id": s.CaseID, "target": s.Target,
			"provider": s.Provider, "status": s.Status, "health": s.Health,
			"events_seen": s.EventsSeen, "new_transactions": s.TxAcquired,
			"events_new": s.EventsNew, "events_duplicate": s.EventsDuplicate,
			"alerts": s.AlertsGenerated, "last_event_at": s.LastEventAt,
			"last_risk_score": s.LastRiskScore, "reconnects": s.Reconnects,
			"gaps": s.Gaps,
		}, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "BCTX MONITOR")
	fmt.Fprintf(out, "  SESSION:   %s\n", s.SessionID)
	fmt.Fprintf(out, "  TARGET:    %s\n", s.Target)
	fmt.Fprintf(out, "  PROVIDER:  %s %s\n", s.Provider, s.ProviderVersion)
	fmt.Fprintf(out, "  STATUS:    %s\n", s.Status)
	fmt.Fprintf(out, "  HEALTH:    %s\n", s.Health)
	fmt.Fprintf(out, "  EVENTS:    seen=%d new=%d dup=%d\n", s.EventsSeen, s.EventsNew, s.EventsDuplicate)
	fmt.Fprintf(out, "  TX NEW:    %d\n", s.TxAcquired)
	fmt.Fprintf(out, "  RISK:      %d\n", s.LastRiskScore)
	fmt.Fprintf(out, "  ALERTS:    %d\n", s.AlertsGenerated)
	if s.Reconnects > 0 || s.Gaps > 0 {
		fmt.Fprintf(out, "  HEALTH:    reconnects=%d gaps=%d\n", s.Reconnects, s.Gaps)
	}
	if s.Error != "" {
		fmt.Fprintf(out, "  ERROR:     %s\n", s.Error)
	}
}

func newMonitorStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status <session-id>",
		Short: "Show a monitoring session status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				s, err := repo.GetMonitorSession(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if s == nil {
					return fmt.Errorf("no session %q", args[0])
				}
				printSession(cmd, gf, *s)
				return nil
			})
		},
	}
}

func newMonitorListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List monitoring sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				ss, err := repo.ListMonitorSessions(cmd.Context())
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if len(ss) == 0 {
					fmt.Fprintln(out, "No monitoring sessions.")
					return nil
				}
				fmt.Fprintf(out, "%-28s %-9s %-10s %-6s %s\n", "SESSION", "PROVIDER", "STATUS", "RISK", "TARGET")
				for _, s := range ss {
					fmt.Fprintf(out, "%-28s %-9s %-10s %-6d %s\n",
						s.SessionID, s.Provider, s.Status, s.LastRiskScore, s.Target)
				}
				return nil
			})
		},
	}
}

// newMonitorLifecycleCmd implements stop/pause by setting a target status.
func newMonitorLifecycleCmd(gf *globalFlags, use, status string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <session-id>",
		Short: use + " a monitoring session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				s, err := repo.GetMonitorSession(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if s == nil {
					return fmt.Errorf("no session %q", args[0])
				}
				s.Status = status
				if err := repo.SaveMonitorSession(cmd.Context(), *s); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Session %s -> %s\n", s.SessionID, status)
				return nil
			})
		},
	}
}

func newMonitorResumeCmd(gf *globalFlags) *cobra.Command {
	var maxPolls int
	c := &cobra.Command{
		Use:   "resume <session-id>",
		Short: "Resume a paused/stopped monitoring session",
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
				s, err := repo.GetMonitorSession(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if s == nil {
					return fmt.Errorf("no session %q", args[0])
				}
				active, _ := caseManagerActive(gf)
				src, perr := monitorProvider(cfg)
				if perr != nil {
					return perr
				}
				if s.Provider != src.ProviderName() {
					return acquisition.ErrSyncCheckpointConflict
				}
				mcfg := monitoringConfig(cfg)
				mcfg.MaxPolls = maxPolls
				svc := monitoring.NewService(repo, src, orchestratorAnalyzer(repo, active), active, mcfg)
				sess, rerr := svc.MonitorWallet(cmd.Context(), s.Target, s.SessionID)
				if rerr != nil && cmd.Context().Err() == nil {
					return rerr
				}
				printSession(cmd, gf, sess)
				return nil
			})
		},
	}
	c.Flags().IntVar(&maxPolls, "max-polls", 1, "bound the number of polls (0 = until Ctrl+C)")
	return c
}
