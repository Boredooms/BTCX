package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/extensions"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/ingestion"
	"github.com/bctx/bctx/pkg/schema"
)

// extension.go is the CLI face of the offline extension / knowledge manager. It
// mirrors the TUI Extensions tab: list / enable / disable discovered extensions
// and LOAD a demo-example into a fresh case end-to-end (import dataset + geo,
// build graph) — all offline, no network. Knowledge-base and model-pack loads
// are reported honestly (model packs are copied into the models dir for the
// registry to pick up; knowledge bases are referenced in place).
func newExtensionCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "extension",
		Aliases: []string{"ext", "extensions"},
		Short:   "Manage local extensions: knowledge bases, model packs, demo examples (offline)",
	}
	cmd.AddCommand(
		newExtensionListCmd(gf),
		newExtensionEnableCmd(gf, true),
		newExtensionEnableCmd(gf, false),
		newExtensionLoadCmd(gf),
	)
	return cmd
}

// extensionStore builds the offline store rooted at ~/.bctx/extensions.
func extensionStore(gf *globalFlags) (*extensions.Store, *configs.Config, error) {
	cfg, err := loadConfig(gf)
	if err != nil {
		return nil, nil, err
	}
	layout := configs.NewLayout(cfg)
	if err := layout.EnsureAll(); err != nil {
		return nil, nil, err
	}
	return extensions.NewStore(layout.Extensions), cfg, nil
}

func newExtensionListCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed extensions and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := extensionStore(gf)
			if err != nil {
				return err
			}
			exts, err := store.List()
			if err != nil {
				return err
			}
			if gf.jsonOut {
				b, _ := json.MarshalIndent(exts, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}
			out := cmd.OutOrStdout()
			if len(exts) == 0 {
				fmt.Fprintf(out, "No extensions installed.\n")
				fmt.Fprintf(out, "Install one by placing a package under:\n  %s\n", store.Root())
				fmt.Fprintf(out, "Each needs a manifest.json (name, version, kind: knowledge-base|model-pack|demo-example).\n")
				return nil
			}
			fmt.Fprintf(out, "%-16s %-12s %-26s %-9s %-9s %s\n",
				"ID", "KIND", "NAME", "VERSION", "STATUS", "PUBLISHER")
			for _, e := range exts {
				fmt.Fprintf(out, "%-16s %-12s %-26s %-9s %-9s %s\n",
					e.ID, e.Manifest.Kind, truncateExt(e.Manifest.Name, 26),
					e.Manifest.Version, e.Status, e.Manifest.Publisher)
				if e.Err != "" {
					fmt.Fprintf(out, "    ! %s\n", e.Err)
				}
			}
			c := extensions.Summarize(exts)
			fmt.Fprintf(out, "\n%d installed · %d enabled · %d knowledge · %d models · %d demos · %d errored\n",
				c.Total, c.Enabled, c.Knowledge, c.Models, c.Demos, c.Errored)
			return nil
		},
	}
}

func newExtensionEnableCmd(gf *globalFlags, enable bool) *cobra.Command {
	use, short := "enable <id>", "Enable an installed extension"
	if !enable {
		use, short = "disable <id>", "Disable an installed extension"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := extensionStore(gf)
			if err != nil {
				return err
			}
			ext, err := store.SetEnabled(args[0], enable)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s is now %s\n", ext.ID, ext.Status)
			return nil
		},
	}
}

func newExtensionLoadCmd(gf *globalFlags) *cobra.Command {
	var caseName string
	c := &cobra.Command{
		Use:   "load <id>",
		Short: "Load an extension into a case (demo-example: import dataset + build graph), offline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, _, err := extensionStore(gf)
			if err != nil {
				return err
			}
			ext, err := store.Get(args[0])
			if err != nil {
				return err
			}
			if ext.Status == extensions.StatusError {
				return fmt.Errorf("cannot load %q: %s", ext.ID, ext.Err)
			}
			switch ext.Manifest.Kind {
			case extensions.KindDemoExample:
				return loadDemoExample(cmd, gf, ext, caseName)
			case extensions.KindModelPack:
				return fmt.Errorf("model-pack load: copy %q into the models dir and restart; "+
					"automatic model-pack install is not yet wired", ext.Manifest.ModelDir)
			case extensions.KindKnowledgeBase:
				fmt.Fprintf(cmd.OutOrStdout(),
					"knowledge-base %q is referenced in place at %s (enable it to surface in the manager)\n",
					ext.ID, ext.Dir)
				return nil
			default:
				return fmt.Errorf("unknown extension kind %q", ext.Manifest.Kind)
			}
		},
	}
	c.Flags().StringVar(&caseName, "case", "", "case to load into (default: the extension id)")
	return c
}

// loadDemoExample creates/opens a case, imports the example dataset (+ optional
// geo), builds the graph, and reports the result. Fully offline.
func loadDemoExample(cmd *cobra.Command, gf *globalFlags, ext extensions.Extension, caseName string) error {
	if caseName == "" {
		caseName = ext.ID
	}
	dataset, err := ext.DatasetPath()
	if err != nil {
		return err
	}
	geo, err := ext.GeoPath()
	if err != nil {
		return err
	}

	cm, err := caseManager(gf)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	// Create if missing, then open (activate).
	if _, cerr := cm.Create(ctx, caseName); cerr != nil {
		// Create fails if it exists; opening next covers that case.
		_ = cerr
	}
	c, err := cm.Open(ctx, caseName)
	if err != nil {
		return fmt.Errorf("open case %q: %w", caseName, err)
	}
	repo, err := cm.OpenRepository(c.ID)
	if err != nil {
		return err
	}
	defer repo.Close()

	im := ingestion.NewImporter(repo, c.ID)
	st, err := im.Import(ctx, dataset, ingestion.FormatNDJSON, nil)
	if err != nil {
		return fmt.Errorf("import dataset: %w", err)
	}
	_ = repo.AppendAudit(ctx, schema.AuditEvent{
		ID: "audit-ext-" + ext.ID + "-" + st.DatasetID, CaseID: c.ID,
		Action: schema.AuditDatasetImported, Subject: dataset, Result: st.Status,
	})
	obs := 0
	if geo != "" {
		if gst, gerr := im.Import(ctx, geo, ingestion.FormatNDJSON, nil); gerr == nil {
			obs = gst.NetworkObs
		}
	}
	gb := graph.NewBuilder(repo)
	gstats, _ := gb.BuildAll(ctx)

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Loaded demo-example %q into case %q (offline)\n", ext.ID, c.ID)
	fmt.Fprintf(out, "  Transactions: %d\n", st.Transactions)
	fmt.Fprintf(out, "  Network obs:  %d\n", obs)
	fmt.Fprintf(out, "  Graph edges:  %d\n", gstats.Edges)
	if ext.Manifest.Subject != "" {
		fmt.Fprintf(out, "  Subject:      %s\n", ext.Manifest.Subject)
		fmt.Fprintf(out, "  Analyze with: bctx case open %s && bctx analyze wallet %s\n",
			c.ID, ext.Manifest.Subject)
	}
	return nil
}

// truncateExt shortens s to at most n runes with an ellipsis.
func truncateExt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
