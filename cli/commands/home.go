package commands

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
	"github.com/bctx/bctx/llm/ollama"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui"
	"github.com/bctx/bctx/tui/screens"
)

// runHome launches the interactive investigator TUI. When stdout is not a TTY
// (pipes, CI, scripts) it prints a plain status summary instead — the TTY vs
// non-TTY split is preserved from the original command (design §2 CLI
// integration). Both branches build the engine through the shared app seam so
// the TUI and CLI use exactly one wiring path.
func runHome(cmd *cobra.Command, gf *globalFlags, info BuildInfo) error {
	cfg, err := loadConfig(gf)
	if err != nil {
		return err
	}
	return launchTUI(cmd, gf, cfg, info)
}

// launchTUI builds the shared app seam and either runs the TUI (on a TTY) or
// prints the plain status summary (off-TTY). It is shared by bare `bctx` and
// the explicit `bctx tui` command. gf is threaded so the TUI's acquire seam can
// re-apply the offline gate (--offline/--airgap) on every acquisition.
func launchTUI(cmd *cobra.Command, gf *globalFlags, cfg *configs.Config, info BuildInfo) error {
	a, err := app.Build(cfg)
	if err != nil {
		return err
	}
	defer a.Cleanup()

	ctx := cmd.Context()

	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return printHomePlain(cmd, a, cfg, ctx, info)
	}

	// Composition root for the OPTIONAL local-LLM summary pane (design §E.3): the
	// network-capable ollama adapter is built HERE and nowhere else, so net/http
	// never enters the air-gapped tui/app closure. With cfg.LLM.Enabled=false
	// (the default) ollama.New returns the deterministic summarizer and no socket
	// is ever constructed. This file is the SOLE importer of llm/ollama.
	var sum llm.Summarizer
	if cfg.LLM.Enabled {
		sum = ollama.New(cfg)
	} else {
		sum = llm.NewDeterministic()
	}

	// Composition root for the network-acquisition seam: the explorer provider
	// is constructed HERE (via providerFromConfig inside the service), never in
	// tui/*, so the TUI stays transport-free. The service re-checks the offline
	// gate on every call, so offline/airgap runs offer no network acquisition.
	acq := newTUIAcquireService(gf, cfg)

	root := tui.NewRoot(ctx, a, tui.WithSummarizer(sum), tui.WithAcquireService(acq))
	// Alt-screen gives the TUI its own screen buffer and, crucially, puts the
	// terminal into the mode where bracketed paste is reported — so a
	// right-click / Ctrl+Shift+V paste into a focused input arrives as one
	// KeyMsg (Paste=true) the input inserts whole, instead of being dropped.
	// Bracketed paste is on by default (we never pass WithoutBracketedPaste).
	p := tui.NewProgram(root, tea.WithContext(ctx), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// printHomePlain writes the non-TTY status summary. Every value comes from a
// live service: the network status from engine.NetworkStatus, the corpus counts
// from Repo.Counts, and MODELS from the inference registry via
// screens.ModelsStatus (replacing the hard-coded "PENDING").
func printHomePlain(cmd *cobra.Command, a *app.App, cfg *configs.Config, ctx context.Context, info BuildInfo) error {
	out := cmd.OutOrStdout()

	caseName := a.CaseID
	network := ""
	if a.Engine != nil {
		network = string(a.Engine.NetworkStatus(ctx))
	}
	var counts sdk.Counts
	if a.Repo != nil {
		counts, _ = a.Repo.Counts(ctx)
	}
	models := screens.ModelsStatus(cfg)

	fmt.Fprintln(out, "BCTX — Bitcoin Forensic Intelligence Platform")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Network:      %s\n", network)
	fmt.Fprintf(out, "Case:         %s\n", nonEmptyCLI(caseName))
	fmt.Fprintf(out, "Transactions: %d\n", counts.Transactions)
	fmt.Fprintf(out, "Wallets:      %d\n", counts.Wallets)
	fmt.Fprintf(out, "Network recs: %d\n", counts.NetworkRecs)
	fmt.Fprintf(out, "Edges:        %d\n", counts.Edges)
	fmt.Fprintf(out, "Alerts:       %d\n", counts.Alerts)
	fmt.Fprintf(out, "Models:       %s\n", models)
	fmt.Fprintf(out, "Graph:        %s\n", graphState(counts.Edges))
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Run 'bctx --help' for commands, or launch in a terminal for the TUI.")
	return nil
}

func nonEmptyCLI(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
