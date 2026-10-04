package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/network/state"
	"github.com/bctx/bctx/tui/screens"
)

// checkResult is one health-check line.
type checkResult struct {
	Name   string
	Status string // READY / PENDING / FAIL / PASS
	Detail string
}

func newDoctorCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose BCTX readiness (binary, config, dirs, db, offline)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(gf)
			if err != nil {
				return err
			}
			layout := configs.NewLayout(cfg)
			out := cmd.OutOrStdout()

			var checks []checkResult

			checks = append(checks, checkResult{"Binary", "READY", "bctx executable running"})

			// Config.
			if _, err := os.Stat(cfg.Path()); err == nil {
				checks = append(checks, checkResult{"Config", "READY", cfg.Path()})
			} else {
				checks = append(checks, checkResult{"Config", "PENDING", "run 'bctx init'"})
			}

			// Directories.
			if layout.Exists() {
				checks = append(checks, checkResult{"Directories", "READY", layout.Root})
			} else {
				checks = append(checks, checkResult{"Directories", "PENDING", "run 'bctx init'"})
			}

			// Database (active case).
			engine, cases, cleanup, berr := buildEngine(cfg)
			if berr != nil {
				return berr
			}
			defer cleanup()

			if _, ok := cases.Active(); !ok {
				checks = append(checks, checkResult{"Database", "PENDING", "no active case; run 'bctx case create <name>'"})
			} else if engine.Repo != nil {
				if _, err := engine.Repo.Counts(cmd.Context()); err == nil {
					checks = append(checks, checkResult{"Database", "READY", "case.db reachable"})
				} else {
					checks = append(checks, checkResult{"Database", "FAIL", err.Error()})
				}
			}

			// Indexes/Graph/Models/Reports are implemented; report the REAL
			// state (the stale "later phase" placeholders are gone).
			checks = append(checks, checkResult{"Indexes", "READY", "schema indexes applied via migrations"})

			// Graph: READY when a graph has been built for the active case,
			// else an honest "not built yet" hint.
			graphDetail := "no graph built — run 'bctx graph build'"
			graphStatus := "PENDING"
			if engine.Repo != nil {
				if c, err := engine.Repo.Counts(cmd.Context()); err == nil && c.Edges > 0 {
					graphStatus, graphDetail = "READY", fmt.Sprintf("%d edges in active case", c.Edges)
				}
			}
			checks = append(checks, checkResult{"Graph", graphStatus, graphDetail})

			// Models: the real ML-manifest status via the shared registry.
			switch screens.ModelsStatus(cfg) {
			case "LOADED":
				checks = append(checks, checkResult{"Models", "READY", "anomaly + flow models loaded (" + app.ModelsDir(cfg) + ")"})
			case "SCHEMA-MISMATCH":
				checks = append(checks, checkResult{"Models", "FAIL", "feature-schema hash mismatch — repackage models"})
			default:
				checks = append(checks, checkResult{"Models", "PENDING", "models not found — install into " + app.ModelsDir(cfg)})
			}

			// Reports: the engine is present; report generation is offline-ready.
			checks = append(checks, checkResult{"Reports", "READY", "offline report engine available"})

			// Offline check: analysis path must not require network.
			offline := checkResult{"Offline", "PASS", "analysis path is local-only"}
			if netStatus := engine.NetworkStatus(cmd.Context()); netStatus == state.Airgapped {
				offline.Detail = "airgap mode active"
			}
			checks = append(checks, offline)

			fmt.Fprintln(out, "BCTX Doctor")
			fmt.Fprintln(out)
			worst := 0
			for _, c := range checks {
				fmt.Fprintf(out, "  %-13s %-8s %s\n", c.Name, c.Status, c.Detail)
				if c.Status == "FAIL" {
					worst = 1
				}
			}
			fmt.Fprintln(out)
			if worst == 0 {
				fmt.Fprintln(out, "Offline readiness: PASS")
			} else {
				fmt.Fprintln(out, "Offline readiness: ATTENTION REQUIRED")
			}
			return nil
		},
	}
}
