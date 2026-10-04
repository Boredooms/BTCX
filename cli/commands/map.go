package commands

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/tui/mapdata"
)

// newMapCmd builds `bctx map`, the offline world-geometry asset lifecycle:
// status, verify, install. Symmetric with `bctx geo`. Fully offline — the
// mapdata package dials nothing; it only reads an installed world-110m.asset
// under ~/.bctx/mapdata/. A missing or corrupt asset yields the documented
// install message, never a crash or a fabricated coastline.
func newMapCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Offline world geometry asset (install, verify, status)",
		Long: "Manage the local world geometry asset (coastlines + country " +
			"centroids) used by the Geo Map screen. BCTX never downloads it: the " +
			"asset is built once at release time by scripts/build_mapdata.sh and " +
			"installed under ~/.bctx/mapdata/. All rendering is fully offline.",
	}
	cmd.AddCommand(
		newMapStatusCmd(gf),
		newMapVerifyCmd(gf),
		newMapInstallCmd(gf),
	)
	return cmd
}

func newMapStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the installed world geometry asset and provenance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := mapdata.Dir()
			if err != nil {
				return err
			}
			reg, err := mapdata.LoadRegistry(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if gf.jsonOut {
				b, _ := json.MarshalIndent(reg, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			if reg.Entry.Name == "" {
				fmt.Fprintln(out, mapdata.NotInstalledMessage)
				fmt.Fprintf(out, "install path: %s\n", dir)
				return nil
			}
			e := reg.Entry
			fmt.Fprintf(out, "World geometry asset: %s\n", e.Name)
			fmt.Fprintf(out, "  source:     %s\n", e.Source)
			fmt.Fprintf(out, "  source_url: %s\n", e.SourceURL)
			fmt.Fprintf(out, "  version:    %s\n", e.Version)
			fmt.Fprintf(out, "  license:    %s\n", e.License)
			fmt.Fprintf(out, "  installed:  %s\n", e.InstalledAt)
			fmt.Fprintf(out, "  centroids:  %d\n", e.RecordCount)
			fmt.Fprintf(out, "  sha256:     %s\n", e.SHA256)
			fmt.Fprintf(out, "  path:       %s\n", e.Path)
			return nil
		},
	}
}

func newMapVerifyCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify [file.asset]",
		Short: "Recompute the asset sha256 and compare it to the registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := mapdata.Dir()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			var entry mapdata.RegistryEntry
			var verifyErr error
			var target string
			if len(args) == 1 {
				target = args[0]
				entry, verifyErr = mapdata.VerifyFile(dir, args[0])
			} else {
				target = "installed asset"
				entry, verifyErr = mapdata.VerifyInstalled(dir)
			}

			if verifyErr != nil {
				switch {
				case errors.Is(verifyErr, mapdata.ErrNotInstalled):
					fmt.Fprintln(out, mapdata.NotInstalledMessage)
					return nil
				case errors.Is(verifyErr, mapdata.ErrChecksumMismatch):
					return fmt.Errorf("VERIFY FAILED: %s sha256 does not match registry", target)
				default:
					return verifyErr
				}
			}
			fmt.Fprintf(out, "VERIFY OK: %s matches registry (%s)\n", target, entry.Name)
			fmt.Fprintf(out, "  sha256: %s\n", entry.SHA256)
			return nil
		},
	}
}

func newMapInstallCmd(gf *globalFlags) *cobra.Command {
	meta := mapdata.RegistryEntry{
		Name:      "world-110m",
		Source:    "Natural Earth 1:110m admin-0 / coastline",
		SourceURL: "https://www.naturalearthdata.com/downloads/110m-cultural-vectors/",
		Version:   "natural-earth-110m",
		License:   "public domain",
	}
	c := &cobra.Command{
		Use:   "install <file.asset>",
		Short: "Install a world-110m.asset into ~/.bctx/mapdata/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := mapdata.Dir()
			if err != nil {
				return err
			}
			entry, err := mapdata.Install(dir, args[0], meta)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if gf.jsonOut {
				b, _ := json.MarshalIndent(entry, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "Installed %s\n", entry.Name)
			fmt.Fprintf(out, "  path:      %s\n", entry.Path)
			fmt.Fprintf(out, "  sha256:    %s\n", entry.SHA256)
			fmt.Fprintf(out, "  centroids: %d\n", entry.RecordCount)
			return nil
		},
	}
	c.Flags().StringVar(&meta.Version, "version", meta.Version, "provenance: asset version")
	c.Flags().StringVar(&meta.Source, "source", meta.Source, "provenance: source description")
	c.Flags().StringVar(&meta.SourceURL, "source-url", meta.SourceURL, "provenance: source URL")
	c.Flags().StringVar(&meta.License, "license", meta.License, "provenance: license")
	return c
}
