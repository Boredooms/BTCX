// Package commands implements the BCTX Cobra command tree. Commands are a thin
// presentation layer: they parse flags, build the engine, call core services,
// and render output. Business logic lives in the domain packages, never here.
package commands

import (
	"github.com/spf13/cobra"

	"github.com/bctx/bctx/configs"
)

// BuildInfo carries build-time metadata injected from main.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// global flags shared across commands.
type globalFlags struct {
	configPath string
	offline    bool
	airgap     bool
	jsonOut    bool
}

// NewRootCommand builds the root `bctx` command.
func NewRootCommand(info BuildInfo) *cobra.Command {
	gf := &globalFlags{}

	root := &cobra.Command{
		Use:   "bctx",
		Short: "BCTX — Bitcoin Forensic Intelligence Platform",
		Long: "BCTX is a terminal-first, offline-first Bitcoin forensic " +
			"intelligence workstation. Acquire while connected, own the data " +
			"locally, analyze locally, disconnect, and keep investigating.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// With no subcommand, launch the interactive TUI home.
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHome(cmd, gf, info)
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&gf.configPath, "config", configs.DefaultPath(), "path to config.toml")
	pf.BoolVar(&gf.offline, "offline", false, "disable acquisition for this run (analysis still works)")
	pf.BoolVar(&gf.airgap, "airgap", false, "hard-disable all network acquisition")
	pf.BoolVar(&gf.jsonOut, "json", false, "emit machine-readable JSON where supported")

	root.AddCommand(
		newVersionCmd(info),
		newInitCmd(gf),
		newStatusCmd(gf),
		newDoctorCmd(gf),
		newCaseCmd(gf),
		newDatasetCmd(gf),
		newAnalyzeCmd(gf),
		newGraphCmd(gf),
		newNeighborsCmd(gf),
		newPathCmd(gf),
		newSyncCmd(gf),
		newProviderCmd(gf),
		newMonitorCmd(gf),
		newReportCmd(gf),
		newGeoCmd(gf),
		newMapCmd(gf),
		newBlockCmd(gf),
		newExtensionCmd(gf),
		newTuiCmd(gf, info),
	)
	return root
}

// loadConfig loads config and applies global flag overrides (offline/airgap).
func loadConfig(gf *globalFlags) (*configs.Config, error) {
	cfg, err := configs.Load(gf.configPath)
	if err != nil {
		return nil, err
	}
	if gf.airgap {
		cfg.Network.Mode = configs.ModeAirgap
		cfg.Network.AcquisitionEnabled = false
	} else if gf.offline {
		cfg.Network.Mode = configs.ModeOffline
	}
	return cfg, nil
}
