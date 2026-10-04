package commands

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// newBlockCmd builds `bctx block`: acquire/inspect Bitcoin blocks as first-class
// investigation subjects. A block hash or height resolves to the block header +
// its transactions, persisted through the SAME canonical path as every other
// acquisition. Acquisition is online; inspection of a locally-present block is
// offline. `block <hash>` and `block height <h>` acquire-if-missing (network,
// consent-gated); `block inspect <hash>` is strictly local.
func newBlockCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block",
		Short: "Acquire and inspect Bitcoin blocks (hash or height)",
		Long: "A block is a first-class investigation subject. `bctx block <hash>` " +
			"or `bctx block height <n>` acquires the block + its transactions from " +
			"the configured provider (network, only if not already local), persists " +
			"them canonically, then you investigate offline. `bctx block inspect " +
			"<hash>` reads a locally-stored block without any network.",
	}
	cmd.AddCommand(
		newBlockHashCmd(gf),
		newBlockHeightCmd(gf),
		newBlockInspectCmd(gf),
	)
	return cmd
}

// newBlockHashCmd: `bctx block <hash>` — local-first, acquire if missing.
func newBlockHashCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "<hash>",
		Short: "Acquire (if missing) and show a block by hash",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlockAcquire(cmd, gf, args[0], 0, false)
		},
	}
}

// newBlockHeightCmd: `bctx block height <n>`.
func newBlockHeightCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "height <n>",
		Short: "Acquire (if missing) and show a block by height",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := strconv.Atoi(args[0])
			if err != nil || h < 0 {
				return fmt.Errorf("invalid block height %q", args[0])
			}
			return runBlockAcquire(cmd, gf, "", h, true)
		},
	}
}

// newBlockInspectCmd: `bctx block inspect <hash>` — strictly local, never network.
func newBlockInspectCmd(gf *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <hash>",
		Short: "Show a locally-stored block (no network)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withActiveRepo(gf, func(repo sdk.Repository) error {
				b, err := repo.GetBlock(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if b == nil {
					return fmt.Errorf("block %s not in local case (acquire it first with 'bctx block %s')", args[0], args[0])
				}
				printBlock(cmd, gf, *b)
				return nil
			})
		},
	}
}

// runBlockAcquire is the shared local-first + acquire path. It looks up the
// block locally first; if present it prints it (no network). If missing it
// acquires through the engine — refused up front when offline/airgapped.
func runBlockAcquire(cmd *cobra.Command, gf *globalFlags, hash string, height int, byHeight bool) error {
	cfg, err := loadConfig(gf)
	if err != nil {
		return err
	}
	return withActiveRepo(gf, func(repo sdk.Repository) error {
		// Local-first: already present?
		var local *schema.Block
		if byHeight {
			local, _ = repo.GetBlockByHeight(cmd.Context(), height)
		} else {
			local, _ = repo.GetBlock(cmd.Context(), hash)
		}
		if local != nil {
			printBlock(cmd, gf, *local)
			return nil
		}
		// Not local -> acquisition required. Enforce the offline boundary BEFORE
		// constructing any provider (no silent network).
		if aerr := acquisitionAllowed(cfg, gf); aerr != nil {
			return aerr
		}
		active, _ := caseManagerActive(gf)
		src, perr := providerFromConfig(cfg)
		if perr != nil {
			return perr
		}
		if !src.Capabilities().BlockLookup {
			return fmt.Errorf("provider %q does not support block lookup", src.ProviderName())
		}
		eng := acquisition.NewEngine(src, repo, active, engineConfig(cfg))
		defer eng.Close()
		st, serr := eng.SyncBlock(cmd.Context(), hash, height, byHeight)
		if serr != nil {
			return serr
		}
		printSyncStats(cmd, gf, st)
		// Show the stored block after acquisition.
		if b, _ := repo.GetBlock(cmd.Context(), st.Target); b != nil {
			fmt.Fprintln(cmd.OutOrStdout())
			printBlock(cmd, gf, *b)
		}
		return nil
	})
}

func printBlock(cmd *cobra.Command, gf *globalFlags, b schema.Block) {
	if gf.jsonOut {
		out, _ := json.MarshalIndent(b, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return
	}
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "BCTX BLOCK")
	fmt.Fprintf(out, "  HASH:       %s\n", b.Hash)
	fmt.Fprintf(out, "  HEIGHT:     %d\n", b.Height)
	fmt.Fprintf(out, "  TIME:       %s\n", blockTime(b.Timestamp))
	fmt.Fprintf(out, "  PREV:       %s\n", orUnavailable(b.PrevHash))
	fmt.Fprintf(out, "  TXNS:       %d\n", b.TxCount)
	fmt.Fprintf(out, "  SIZE:       %s\n", orUnavailableInt(b.Size))
	fmt.Fprintf(out, "  WEIGHT:     %s\n", orUnavailableInt(b.Weight))
	fmt.Fprintf(out, "  MERKLE:     %s\n", orUnavailable(b.MerkleRoot))
	fmt.Fprintf(out, "  SOURCE:     %s (%s)\n", b.Provenance.SourceIdentifier, b.Provenance.SourceType)
	fmt.Fprintf(out, "  ACQUIRED:   %s\n", blockTime(b.Provenance.RetrievedAt))
}

func blockTime(t time.Time) string {
	if t.IsZero() {
		return "unavailable"
	}
	return t.UTC().Format(time.RFC3339)
}

func orUnavailable(s string) string {
	if s == "" {
		return "unavailable"
	}
	return s
}

func orUnavailableInt(n int) string {
	if n == 0 {
		return "unavailable"
	}
	return strconv.Itoa(n)
}

// newSyncBlockCmd adds `bctx sync block <hash>` / `sync block height <n>` so
// block acquisition is also reachable under the sync verb (parity with sync
// wallet/tx). It shares runBlockAcquire.
func newSyncBlockCmd(gf *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block <hash>",
		Short: "Acquire a block (and its transactions) from the provider",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "height" {
				if len(args) < 2 {
					return fmt.Errorf("usage: bctx sync block height <n>")
				}
				h, err := strconv.Atoi(args[1])
				if err != nil || h < 0 {
					return fmt.Errorf("invalid block height %q", args[1])
				}
				return runBlockAcquire(cmd, gf, "", h, true)
			}
			return runBlockAcquire(cmd, gf, args[0], 0, false)
		},
	}
	return cmd
}
