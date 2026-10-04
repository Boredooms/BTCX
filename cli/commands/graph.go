package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/sdk"
)

// withActiveRepo opens the active case repository and runs fn.
func withActiveRepo(gf *globalFlags, fn func(repo sdk.Repository) error) error {
	cfg, err := loadConfig(gf)
	if err != nil {
		return err
	}
	_, cases, cleanup, err := buildEngine(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	active, ok := cases.Active()
	if !ok {
		return fmt.Errorf("no active case; run 'bctx case create <name>' first")
	}
	repo, err := cases.OpenRepository(active.ID)
	if err != nil {
		return err
	}
	defer repo.Close()
	return fn(repo)
}

func newGraphCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "graph", Short: "Graph build and queries"}
	cmd.AddCommand(
		newGraphWalletCmd(gf),
		newGraphBuildCmd(gf, false),
		newGraphBuildCmd(gf, true),
		newGraphStatsCmd(gf),
	)
	return cmd
}

func newGraphWalletCmd(gf *globalFlags) *cobra.Command {
	var depth int
	c := &cobra.Command{
		Use:   "wallet <address>",
		Short: "Extract a bounded subgraph around a wallet",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				svc := graph.NewService(repo)
				sg, err := svc.Subgraph(cmd.Context(), args[0], depth)
				if err != nil {
					return err
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(sg, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Subgraph of %s (depth %d)\n", sg.Center, sg.Depth)
				fmt.Fprintf(out, "  nodes: %d  edges: %d\n", len(sg.Nodes), len(sg.Edges))
				for _, e := range sg.Edges {
					fmt.Fprintf(out, "  %s --%s--> %s\n", e.From, e.Type, e.To)
				}
				return nil
			})
		},
	}
	c.Flags().IntVar(&depth, "depth", 2, "traversal depth")
	return c
}

func newGraphBuildCmd(gf *globalFlags, rebuild bool) *cobra.Command {
	use, short := "build", "Build the graph from local data (idempotent)"
	if rebuild {
		use, short = "rebuild", "Clear and rebuild the graph from canonical data"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				b := graph.NewBuilder(repo)
				var st graph.Stats
				var err error
				if rebuild {
					st, err = b.Rebuild(cmd.Context())
				} else {
					st, err = b.BuildAll(cmd.Context())
				}
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Graph %s complete.\n", use)
				fmt.Fprintf(out, "  Transactions: %d\n", st.Transactions)
				fmt.Fprintf(out, "  Observations: %d\n", st.Observations)
				fmt.Fprintf(out, "  Edges:        %d\n", st.Edges)
				for t, n := range st.ByType {
					fmt.Fprintf(out, "    %-14s %d\n", t, n)
				}
				return nil
			})
		},
	}
}

func newGraphStatsCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show persisted graph statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				st, err := graph.NewBuilder(repo).GraphStats(cmd.Context())
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Edges: %d\n", st.Edges)
				for t, n := range st.ByType {
					fmt.Fprintf(out, "  %-14s %d\n", t, n)
				}
				return nil
			})
		},
	}
}

func newNeighborsCmd(gf *globalFlags) *cobra.Command {
	var depth int
	c := &cobra.Command{
		Use:   "neighbors <id>",
		Short: "List neighbors within a bounded depth",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				svc := graph.NewService(repo)
				nodes, err := svc.NeighborsDepth(cmd.Context(), args[0], depth)
				if err != nil {
					return err
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(nodes, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Neighbors of %s (depth %d): %d\n", args[0], depth, len(nodes))
				for _, n := range nodes {
					fmt.Fprintf(out, "  %-10s %s\n", n.Type, n.ID)
				}
				return nil
			})
		},
	}
	c.Flags().IntVar(&depth, "depth", 1, "traversal depth")
	return c
}

func newPathCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "path <source> <destination>",
		Short: "Find a shortest path between two nodes",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				svc := graph.NewService(repo)
				p, err := svc.Path(cmd.Context(), args[0], args[1])
				if err != nil {
					return err
				}
				if gf.jsonOut {
					b, _ := json.MarshalIndent(map[string]any{
						"source": args[0], "destination": args[1],
						"path": p, "hops": hops(p),
					}, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(b))
					return nil
				}
				out := cmd.OutOrStdout()
				if len(p) == 0 {
					fmt.Fprintf(out, "No path from %s to %s.\n", args[0], args[1])
					return nil
				}
				fmt.Fprintf(out, "Path (%d hops):\n", hops(p))
				for i, n := range p {
					if i > 0 {
						fmt.Fprint(out, "  -> ")
					} else {
						fmt.Fprint(out, "  ")
					}
					fmt.Fprintln(out, n)
				}
				return nil
			})
		},
	}
}

func hops(path []string) int {
	if len(path) == 0 {
		return 0
	}
	return len(path) - 1
}
