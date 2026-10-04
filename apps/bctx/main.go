// Command bctx is the single native entry point for the BCTX
// Bitcoin Forensic Intelligence Platform.
//
// BCTX is terminal-first and offline-first. Only the acquisition layer may
// touch the network; every analysis path operates on local evidence.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bctx/bctx/cli/commands"
)

// Build-time variables, injected via -ldflags.
var (
	version = "0.1.0-dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	root := commands.NewRootCommand(commands.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	})

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "bctx:", err)
		os.Exit(1)
	}
}
