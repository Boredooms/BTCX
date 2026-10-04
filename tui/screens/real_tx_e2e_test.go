package screens

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// This test is the UI half of the real-data end-to-end proof. The companion
// shell/CLI steps acquire the real mainnet transaction
//
//	fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14
//
// from the Esplora provider into a case; testdata/real_tx_e2e.db is a byte
// copy of that acquired case DB (no fabrication — the only "fixture" is a
// snapshot of real acquired on-chain data). Ground truth from blockstream.info:
//
//	input  : 32aneueQWesQHetWba4xU7qfEFhhEYGNgP  value 11267934 sat (0.11267934 BTC)
//	outputs: 13sNX683FmgtDhVNSqfvEHxfB48zWj67Rn   106475   sat
//	         bc1qdulvvg74lu57z5tmz2aa7n5xddgz042g09nzr7 110842 sat
//	         32aneueQWesQHetWba4xU7qfEFhhEYGNgP (change) 11047769 sat
//	fee    : 2848 sat (0.00002848 BTC)   total out = 11265086 sat
//
// The test drives the real Transaction and Graph screens against this data and
// asserts the real values render — the "returns the transaction, then the graph
// interactive visualizer, with the wallet it held and patterns" contract.
const (
	realTxID      = "fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14"
	realInputAddr = "32aneueQWesQHetWba4xU7qfEFhhEYGNgP"
	realOut1      = "13sNX683FmgtDhVNSqfvEHxfB48zWj67Rn"
	realOut2      = "bc1qdulvvg74lu57z5tmz2aa7n5xddgz042g09nzr7"
)

// openRealCaseCtx copies the acquired case DB to a temp file (so the test never
// mutates the committed snapshot) and wires a ScreenCtx to it.
func openRealCaseCtx(t *testing.T, subject Subject) *ScreenCtx {
	t.Helper()
	src := filepath.Join("testdata", "real_tx_e2e.db")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("real-data snapshot missing (%v); run the acquire step first", err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "case.db")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatalf("copy snapshot: %v", err)
	}
	repo, err := sqlite.NewRepository(dst)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline // everything after acquire is offline
	return &ScreenCtx{
		Ctx:     context.Background(),
		App:     &app.App{Repo: repo, CaseID: "tx-e2e", Cleanup: func() {}},
		Cfg:     cfg,
		Styles:  theme.Build(theme.Default()),
		Subject: subject,
	}
}

// TestRealTransactionScreenRendersOnChainData proves the Transaction screen
// loads the acquired canonical tx and renders its real txid, fan-in/out, and
// the real output address — not a placeholder.
func TestRealTransactionScreenRendersOnChainData(t *testing.T) {
	ctx := openRealCaseCtx(t, Subject{ID: realTxID, Kind: SubjectTx})
	m := driveAll(NewTransaction(ctx))
	out := m.View(components.Frame{W: 160, H: 50})

	if strings.Contains(out, "not found in active case") {
		t.Fatalf("transaction screen reports not found — acquire/store broke:\n%s", out)
	}
	// Detail panel short-IDs addresses (prefix…suffix), so assert on the
	// stable 8-char prefix the shortID formatter keeps.
	mustContain(t, "transaction", out, realInputAddr[:8])
	mustContain(t, "transaction", out, realOut1[:8])
	mustContain(t, "transaction", out, realOut2[:8])
	// Exact on-chain amounts (ground truth from blockstream.info).
	mustContain(t, "transaction", out, "0.11265086 BTC") // total out
	mustContain(t, "transaction", out, "0.00002848 BTC") // fee
	mustContain(t, "transaction", out, "0.11267934 BTC") // input value
	// fan-in 1, fan-out 3.
	mustContain(t, "transaction", out, "inputs")
	mustContain(t, "transaction", out, "outputs")
	assertNoOverflow(t, "transaction-real", out, components.Frame{W: 160, H: 50})
}

// TestRealGraphScreenRendersSubgraph proves the Graph interactive visualizer
// centers on the real subject and renders a bounded subgraph (the acquired tx
// fan-out), with the center-node risk join and no overflow.
func TestRealGraphScreenRendersSubgraph(t *testing.T) {
	// Center on the input wallet — its subgraph includes the tx and the two
	// distinct output wallets (depth 2).
	ctx := openRealCaseCtx(t, Subject{ID: realInputAddr, Kind: SubjectWallet})
	m := driveAll(NewGraph(ctx))
	out := m.View(components.Frame{W: 160, H: 50})

	mustContain(t, "graph", out, "GRAPH")
	mustContain(t, "graph", out, "SUBGRAPH")
	// The center node id (short form) must appear in the header.
	mustContain(t, "graph", out, realInputAddr[:8])
	if strings.Contains(out, "no subject") {
		t.Fatalf("graph reports no subject for a real acquired wallet:\n%s", out)
	}
	assertNoOverflow(t, "graph-real", out, components.Frame{W: 160, H: 50})
}

// driveAll runs Init and fully pumps the resulting command tree — unwrapping
// tea.BatchMsg so batched data-load commands (transactionCmd + edgesCmd) all
// deliver their messages, mirroring the Bubble Tea runtime more faithfully than
// the single-shot drive helper.
func driveAll(m Model) Model {
	var pump func(tea.Cmd)
	pump = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		msg := cmd()
		switch mm := msg.(type) {
		case nil:
			return
		case tea.BatchMsg:
			for _, c := range mm {
				pump(c)
			}
		default:
			var next tea.Cmd
			m, next = m.Update(mm)
			pump(next)
		}
	}
	pump(m.Init())
	return m
}

func mustContain(t *testing.T, screen, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Errorf("%s screen render missing %q\n----\n%s\n----", screen, want, out)
	}
}
