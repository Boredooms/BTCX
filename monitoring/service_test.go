package monitoring

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
)

func newRepo(t *testing.T) *sqlite.Repository {
	t.Helper()
	r, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func page(txs ...acquisition.AcquiredTransaction) acquisition.AcquiredWalletPage {
	p := acquisition.AcquiredWalletPage{Address: "A", Done: true}
	for _, t := range txs {
		p.TxIDs = append(p.TxIDs, t.TxID)
		p.Transactions = append(p.Transactions, t)
	}
	return p
}

func mkTx(id, from, to string, amt float64, confirmed bool) acquisition.AcquiredTransaction {
	return acquisition.AcquiredTransaction{
		TxID: id, Timestamp: "2026-01-01T10:00:00Z", FeeBTC: 0.0001, HasFee: true,
		ScriptType: "p2wpkh", BaseSize: 110, TotalSize: 140, Weight: 470, VSize: 118,
		Confirmed: confirmed, BlockHeight: boolHeight(confirmed),
		Inputs:  []acquisition.AcquiredIO{{Address: from, AmountBTC: amt}},
		Outputs: []acquisition.AcquiredIO{{Address: to, AmountBTC: amt - 0.0001}},
	}
}

func boolHeight(c bool) int {
	if c {
		return 800000
	}
	return 0
}

func testCfg() Config {
	c := DefaultConfig()
	c.PollInterval = time.Millisecond
	c.ReconnectBackoff = time.Millisecond
	c.MaxPolls = 1
	return c
}

// stepAnalyzer returns scripted risk per call to drive delta/alert tests.
func stepAnalyzer(scores []int, patterns [][]string) Analyzer {
	i := 0
	return func(ctx context.Context, subject string) (AnalysisResult, error) {
		idx := i
		if idx >= len(scores) {
			idx = len(scores) - 1
		}
		var pats []string
		if idx < len(patterns) {
			pats = patterns[idx]
		}
		i++
		return AnalysisResult{
			Subject: subject, RiskScore: scores[idx], Confidence: 0.9,
			Signals: []string{"transaction_anomaly"}, Patterns: pats,
			EvidenceIDs: []string{"E1"},
		}, nil
	}
}

func TestSessionCreationAndPersistence(t *testing.T) {
	r := newRepo(t)
	sp := newScripted([]acquisition.AcquiredWalletPage{page(mkTx("T1", "A", "B", 1.0, true))})
	svc := NewService(r, sp, stepAnalyzer([]int{40}, nil), "c", testCfg())
	ctx := context.Background()
	sess, err := svc.MonitorWallet(ctx, "A", "mon-A")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "completed" {
		t.Fatalf("status=%q, want completed", sess.Status)
	}
	got, _ := r.GetMonitorSession(ctx, "mon-A")
	if got == nil || got.TxAcquired != 1 {
		t.Fatalf("session not persisted correctly: %+v", got)
	}
	// Canonical tx persisted.
	tx, _ := r.GetTransaction(ctx, "T1")
	if tx == nil || tx.Size.VSize != 118 {
		t.Fatalf("canonical tx not persisted with size: %+v", tx)
	}
}

func TestEventAndTxDedup(t *testing.T) {
	r := newRepo(t)
	// Same tx returned on two polls.
	p := page(mkTx("T1", "A", "B", 1.0, true))
	sp := newScripted([]acquisition.AcquiredWalletPage{p, p})
	cfg := testCfg()
	cfg.MaxPolls = 2
	svc := NewService(r, sp, stepAnalyzer([]int{40, 40}, nil), "c", cfg)
	sess, err := svc.MonitorWallet(context.Background(), "A", "mon-dedup")
	if err != nil {
		t.Fatal(err)
	}
	if sess.EventsNew != 1 {
		t.Fatalf("events_new=%d, want 1 (deduped)", sess.EventsNew)
	}
	if sess.EventsDuplicate < 1 {
		t.Fatalf("events_duplicate=%d, want >=1", sess.EventsDuplicate)
	}
	if sess.TxAcquired != 1 {
		t.Fatalf("tx_acquired=%d, want 1 (canonical dedup)", sess.TxAcquired)
	}
}

func TestMempoolToConfirmed(t *testing.T) {
	r := newRepo(t)
	// Poll 1: mempool tx; Poll 2: same tx confirmed -> distinct events.
	p1 := page(mkTx("T1", "A", "B", 1.0, false))
	p2 := page(mkTx("T1", "A", "B", 1.0, true))
	sp := newScripted([]acquisition.AcquiredWalletPage{p1, p2})
	cfg := testCfg()
	cfg.MaxPolls = 2
	svc := NewService(r, sp, stepAnalyzer([]int{40, 40}, nil), "c", cfg)
	sess, err := svc.MonitorWallet(context.Background(), "A", "mon-conf")
	if err != nil {
		t.Fatal(err)
	}
	// Two distinct events (mempool + confirmed) for the same txid.
	if sess.EventsNew != 2 {
		t.Fatalf("events_new=%d, want 2 (mempool + confirmed)", sess.EventsNew)
	}
}

func TestRiskDeltaAndAlert(t *testing.T) {
	r := newRepo(t)
	// Poll 1: risk 40; Poll 2: risk 71 (+31 >= threshold 10) -> alert.
	p := page(mkTx("T1", "A", "B", 1.0, false))
	p2 := page(mkTx("T2", "A", "C", 0.5, false))
	sp := newScripted([]acquisition.AcquiredWalletPage{p, p2})
	cfg := testCfg()
	cfg.MaxPolls = 2
	cfg.AlertRiskDelta = 10
	svc := NewService(r, sp, stepAnalyzer([]int{40, 71}, nil), "c", cfg)
	ctx := context.Background()
	sess, err := svc.MonitorWallet(ctx, "A", "mon-delta")
	if err != nil {
		t.Fatal(err)
	}
	if sess.LastRiskScore != 71 {
		t.Fatalf("last risk=%d, want 71", sess.LastRiskScore)
	}
	alerts, _ := r.ListMonitorAlerts(ctx, "mon-delta")
	found := false
	for _, a := range alerts {
		if a.Trigger == "risk_score_increase" && a.Delta == 31 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected risk_score_increase alert +31, got %+v", alerts)
	}
}

func TestAlertDeduplication(t *testing.T) {
	r := newRepo(t)
	// Same risk jump re-observed must not create duplicate alerts (same trigger
	// event). Use identical pages/scores across 3 polls.
	p := page(mkTx("T1", "A", "B", 1.0, true))
	sp := newScripted([]acquisition.AcquiredWalletPage{p, p, p})
	cfg := testCfg()
	cfg.MaxPolls = 3
	svc := NewService(r, sp, stepAnalyzer([]int{80, 80, 80}, nil), "c", cfg)
	ctx := context.Background()
	if _, err := svc.MonitorWallet(ctx, "A", "mon-adedup"); err != nil {
		t.Fatal(err)
	}
	alerts, _ := r.ListMonitorAlerts(ctx, "mon-adedup")
	// First poll: prev=-1 so no risk-increase alert; subsequent polls: no change.
	// There must be no runaway duplication.
	if len(alerts) > 1 {
		t.Fatalf("expected <=1 alert, got %d (dedup failed)", len(alerts))
	}
}

func TestProviderOutageThenReconnect(t *testing.T) {
	r := newRepo(t)
	sp := newScripted([]acquisition.AcquiredWalletPage{
		page(mkTx("T1", "A", "B", 1.0, true)),
		{}, // placeholder; poll 1 will fail via failAt
		page(mkTx("T2", "A", "C", 0.5, true)),
	})
	sp.failAt[1] = acquisition.ErrProviderUnavailable // transient on 2nd poll
	cfg := testCfg()
	cfg.MaxPolls = 3
	cfg.MaxReconnects = 5
	svc := NewService(r, sp, stepAnalyzer([]int{40, 40, 45}, nil), "c", cfg)
	sess, err := svc.MonitorWallet(context.Background(), "A", "mon-recon")
	if err != nil {
		t.Fatalf("should recover from transient outage: %v", err)
	}
	if sess.Reconnects < 1 {
		t.Fatalf("expected >=1 reconnect, got %d", sess.Reconnects)
	}
	if sess.Status != "completed" {
		t.Fatalf("status=%q, want completed after reconnect", sess.Status)
	}
}

func TestCancellationSafe(t *testing.T) {
	r := newRepo(t)
	sp := newScripted([]acquisition.AcquiredWalletPage{page(mkTx("T1", "A", "B", 1.0, true))})
	cfg := testCfg()
	cfg.MaxPolls = 0 // run until cancelled
	svc := NewService(r, sp, stepAnalyzer([]int{40}, nil), "c", cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sess, err := svc.MonitorWallet(ctx, "A", "mon-cancel")
	if err == nil {
		t.Fatal("expected cancellation")
	}
	if sess.Status != "paused" {
		t.Fatalf("status=%q, want paused on cancel", sess.Status)
	}
}

func TestCapabilityUnsupported(t *testing.T) {
	r := newRepo(t)
	sp := newScripted(nil)
	sp.caps.WalletHistory = false
	svc := NewService(r, sp, stepAnalyzer([]int{0}, nil), "c", testCfg())
	_, err := svc.MonitorWallet(context.Background(), "A", "mon-cap")
	if err != acquisition.ErrCapabilityUnsupported {
		t.Fatalf("err=%v, want ErrCapabilityUnsupported", err)
	}
}

func TestCrashRecoveryIdempotent(t *testing.T) {
	r := newRepo(t)
	p := page(mkTx("T1", "A", "B", 1.0, true), mkTx("T2", "A", "C", 0.5, true))
	// First "run" (simulated crash = just stop after one poll).
	sp1 := newScripted([]acquisition.AcquiredWalletPage{p})
	svc1 := NewService(r, sp1, stepAnalyzer([]int{40}, nil), "c", testCfg())
	ctx := context.Background()
	if _, err := svc1.MonitorWallet(ctx, "A", "mon-crash"); err != nil {
		t.Fatal(err)
	}
	// Restart: same session id, same data. Must not duplicate canonical txs.
	sp2 := newScripted([]acquisition.AcquiredWalletPage{p})
	svc2 := NewService(r, sp2, stepAnalyzer([]int{40}, nil), "c", testCfg())
	sess, err := svc2.MonitorWallet(ctx, "A", "mon-crash")
	if err != nil {
		t.Fatal(err)
	}
	if sess.TxAcquired != 0 {
		t.Fatalf("restart acquired %d new txs, want 0 (idempotent)", sess.TxAcquired)
	}
	// Count canonical txs: exactly 2.
	c, _ := r.Counts(ctx)
	if c.Transactions != 2 {
		t.Fatalf("transactions=%d, want 2 after crash+restart", c.Transactions)
	}
}

func TestCaseIsolation(t *testing.T) {
	rA := newRepo(t)
	rB := newRepo(t)
	ctx := context.Background()
	spA := newScripted([]acquisition.AcquiredWalletPage{page(mkTx("TA", "A", "B", 1.0, true))})
	spB := newScripted([]acquisition.AcquiredWalletPage{page(mkTx("TB", "X", "Y", 2.0, true))})
	svcA := NewService(rA, spA, stepAnalyzer([]int{40}, nil), "caseA", testCfg())
	svcB := NewService(rB, spB, stepAnalyzer([]int{40}, nil), "caseB", testCfg())
	if _, err := svcA.MonitorWallet(ctx, "A", "mon-iA"); err != nil {
		t.Fatal(err)
	}
	if _, err := svcB.MonitorWallet(ctx, "X", "mon-iB"); err != nil {
		t.Fatal(err)
	}
	// Case A must not see case B's tx.
	if tx, _ := rA.GetTransaction(ctx, "TB"); tx != nil {
		t.Fatal("case A leaked case B transaction")
	}
	if tx, _ := rB.GetTransaction(ctx, "TA"); tx != nil {
		t.Fatal("case B leaked case A transaction")
	}
}

func TestConfirmationStateRepresented(t *testing.T) {
	r := newRepo(t)
	sp := newScripted([]acquisition.AcquiredWalletPage{page(mkTx("T1", "A", "B", 1.0, false))})
	svc := NewService(r, sp, stepAnalyzer([]int{40}, nil), "c", testCfg())
	ctx := context.Background()
	if _, err := svc.MonitorWallet(ctx, "A", "mon-mempool"); err != nil {
		t.Fatal(err)
	}
	// The monitor event for an unconfirmed tx must be a discovery, not confirmed.
	exists, _ := r.MonitorEventExists(ctx, eventID("T1", false))
	if !exists {
		t.Fatal("expected mempool (unconfirmed) event")
	}
	confirmedExists, _ := r.MonitorEventExists(ctx, eventID("T1", true))
	if confirmedExists {
		t.Fatal("must not fabricate a confirmed event for a mempool tx")
	}
	_ = schema.CompleteValid
}
