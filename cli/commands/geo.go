package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/tui/geoip"
)

// newGeoCmd builds `bctx geo`, the offline GeoIP asset lifecycle: status,
// inspect, install, verify. Every subcommand is fully offline — the geoip
// package dials nothing; it only reads a user-installed .mmdb under
// ~/.bctx/geoip/. A missing database yields the documented install message,
// never a crash or fabricated lookup.
func newGeoCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "geo",
		Short: "Offline GeoIP / ASN database (install, verify, inspect)",
		Long: "Manage the local GeoIP / ASN database used to annotate network " +
			"observations. BCTX never downloads it: you supply a free DB-IP .mmdb " +
			"and install it under ~/.bctx/geoip/. All lookups are fully offline.",
	}
	cmd.AddCommand(
		newGeoStatusCmd(gf),
		newGeoInspectCmd(gf),
		newGeoInstallCmd(gf),
		newGeoVerifyCmd(gf),
	)
	return cmd
}

func newGeoStatusCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show installed GeoIP databases and provenance",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := geoip.Dir()
			if err != nil {
				return err
			}
			reg, err := geoip.LoadRegistry(dir)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if gf.jsonOut {
				b, _ := json.MarshalIndent(reg, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			if len(reg.Entries) == 0 {
				fmt.Fprintln(out, geoip.NotInstalledMessage)
				fmt.Fprintf(out, "install path: %s\n", dir)
				return nil
			}
			fmt.Fprintf(out, "GeoIP install dir: %s\n", dir)
			for _, t := range []string{"country", "asn"} {
				e, ok := reg.Entries[t]
				if !ok {
					continue
				}
				fmt.Fprintf(out, "\n[%s] %s\n", e.DBType, e.DBName)
				fmt.Fprintf(out, "  source:      %s\n", e.Source)
				fmt.Fprintf(out, "  source_url:  %s\n", e.SourceURL)
				fmt.Fprintf(out, "  version:     %s\n", e.Version)
				fmt.Fprintf(out, "  license:     %s\n", e.License)
				fmt.Fprintf(out, "  installed:   %s\n", e.InstalledAt)
				fmt.Fprintf(out, "  records:     %d\n", e.RecordCount)
				fmt.Fprintf(out, "  sha256:      %s\n", e.SHA256)
				fmt.Fprintf(out, "  path:        %s\n", e.Path)
			}
			return nil
		},
	}
}

func newGeoInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <ip>",
		Short: "Resolve country and ASN for one IP, fully offline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ip, err := netip.ParseAddr(args[0])
			if err != nil {
				return fmt.Errorf("invalid IP %q: %w", args[0], err)
			}
			dir, err := geoip.Dir()
			if err != nil {
				return err
			}
			d, err := geoip.Open(dir)
			if err != nil {
				if errors.Is(err, geoip.ErrNotInstalled) {
					fmt.Fprintln(cmd.OutOrStdout(), geoip.NotInstalledMessage)
					return nil
				}
				return err
			}
			defer d.Close()

			country, err := d.LookupCountry(ip)
			if err != nil {
				return err
			}
			asn, err := d.LookupASN(ip)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if gf.jsonOut {
				b, _ := json.MarshalIndent(map[string]any{
					"ip":      ip.String(),
					"country": country,
					"asn":     asn,
				}, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "IP:      %s\n", ip)
			if country.ISOCode == "" {
				fmt.Fprintln(out, "Country: (no record)")
			} else {
				fmt.Fprintf(out, "Country: %s (%s)\n", country.Name, country.ISOCode)
			}
			if asn.Number == 0 {
				fmt.Fprintln(out, "ASN:     (no record)")
			} else {
				fmt.Fprintf(out, "ASN:     AS%d %s\n", asn.Number, asn.Org)
			}
			return nil
		},
	}
}

func newGeoInstallCmd(gf *globalFlags) *cobra.Command {
	meta := geoip.RegistryEntry{
		Source:    "DB-IP IP-to-Country Lite (db-ip.com)",
		SourceURL: "https://db-ip.com/db/download/ip-to-country-lite",
		License:   "CC BY 4.0",
	}
	c := &cobra.Command{
		Use:   "install <file.mmdb>",
		Short: "Install a user-supplied .mmdb into ~/.bctx/geoip/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := geoip.Dir()
			if err != nil {
				return err
			}
			entry, err := geoip.Install(dir, args[0], meta)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if gf.jsonOut {
				b, _ := json.MarshalIndent(entry, "", "  ")
				fmt.Fprintln(out, string(b))
				return nil
			}
			fmt.Fprintf(out, "Installed %s (%s)\n", entry.DBName, entry.DBType)
			fmt.Fprintf(out, "  path:   %s\n", entry.Path)
			fmt.Fprintf(out, "  sha256: %s\n", entry.SHA256)
			fmt.Fprintf(out, "  records:%d\n", entry.RecordCount)
			return nil
		},
	}
	c.Flags().StringVar(&meta.DBName, "name", "", "override registry db name (default: file base name)")
	c.Flags().StringVar(&meta.DBType, "type", "", "db type: country|asn (default: auto-detect)")
	c.Flags().StringVar(&meta.Source, "source", meta.Source, "provenance: source description")
	c.Flags().StringVar(&meta.SourceURL, "source-url", meta.SourceURL, "provenance: source URL")
	c.Flags().StringVar(&meta.Version, "version", "", "provenance: dataset version (e.g. 2024.11)")
	c.Flags().StringVar(&meta.License, "license", meta.License, "provenance: license")
	return c
}

func newGeoVerifyCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <file.mmdb>",
		Short: "Recompute a file's sha256 and compare it to the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := geoip.Dir()
			if err != nil {
				return err
			}
			entry, err := geoip.Verify(dir, args[0])
			out := cmd.OutOrStdout()
			if err != nil {
				switch {
				case errors.Is(err, geoip.ErrNotInstalled):
					fmt.Fprintln(out, geoip.NotInstalledMessage)
					return nil
				case errors.Is(err, geoip.ErrChecksumMismatch):
					return fmt.Errorf("VERIFY FAILED: %s does not match any installed database sha256", args[0])
				default:
					return err
				}
			}
			fmt.Fprintf(out, "VERIFY OK: %s matches installed %s (%s)\n", args[0], entry.DBName, entry.DBType)
			fmt.Fprintf(out, "  sha256: %s\n", entry.SHA256)
			return nil
		},
	}
}
