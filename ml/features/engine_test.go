package features

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// fakeRepo is a minimal in-memory sdk.Repository for feature tests.
type fakeRepo struct {
	txs map[string][]schema.Transaction // address -> txs
}

func (f *fakeRepo) GetWallet(context.Context, string) (*schema.Wallet, error) { return nil, nil }
func (f *fakeRepo) GetTransaction(context.Context, string) (*schema.Transaction, error) {
	return nil, nil
}
func (f *fakeRepo) WalletTransactions(_ context.Context, a string, _ int) ([]schema.Transaction, error) {
	return f.txs[a], nil
}
func (f *fakeRepo) NetworkObservationsByIP(context.Context, string) ([]schema.NetworkObservation, error) {
	return nil, nil
}
func (f *fakeRepo) SaveTransactions(context.Context, []schema.Transaction) error { return nil }
func (f *fakeRepo) SaveNetworkObservations(context.Context, []schema.NetworkObservation) error {
	return nil
}
func (f *fakeRepo) SaveEdges(context.Context, []schema.GraphEdge) error             { return nil }
func (f *fakeRepo) SaveEvidence(context.Context, []schema.EvidenceItem) error       { return nil }
func (f *fakeRepo) SaveRiskAssessment(context.Context, schema.RiskAssessment) error { return nil }
func (f *fakeRepo) SaveAlert(context.Context, schema.Alert) error                   { return nil }
func (f *fakeRepo) ListAlerts(context.Context, int) ([]schema.Alert, error)         { return nil, nil }
func (f *fakeRepo) AppendAudit(context.Context, schema.AuditEvent) error            { return nil }
func (f *fakeRepo) Counts(context.Context) (sdk.Counts, error)                      { return sdk.Counts{}, nil }
func (f *fakeRepo) Close() error                                                    { return nil }
func (f *fakeRepo) EdgesFrom(context.Context, string) ([]schema.GraphEdge, error)   { return nil, nil }
func (f *fakeRepo) EdgesTo(context.Context, string) ([]schema.GraphEdge, error)     { return nil, nil }
func (f *fakeRepo) AllEdges(context.Context, int) ([]schema.GraphEdge, error)       { return nil, nil }
func (f *fakeRepo) DeleteEdges(context.Context) error                               { return nil }
func (f *fakeRepo) AllTransactions(context.Context, int) ([]schema.Transaction, error) {
	return nil, nil
}
func (f *fakeRepo) RecentTransactions(context.Context, int) ([]schema.Transaction, error) {
	return nil, nil
}
func (f *fakeRepo) SaveBlock(context.Context, schema.Block) error { return nil }
func (f *fakeRepo) GetBlock(context.Context, string) (*schema.Block, error) {
	return nil, nil
}
func (f *fakeRepo) GetBlockByHeight(context.Context, int) (*schema.Block, error) {
	return nil, nil
}
func (f *fakeRepo) ListBlocks(context.Context, int) ([]schema.Block, error) {
	return nil, nil
}
func (f *fakeRepo) AllNetworkObservations(context.Context, int) ([]schema.NetworkObservation, error) {
	return nil, nil
}
func (f *fakeRepo) TxExists(context.Context, string) (bool, error)         { return false, nil }
func (f *fakeRepo) ObsExists(context.Context, string) (bool, error)        { return false, nil }
func (f *fakeRepo) SaveDataset(context.Context, schema.Dataset) error      { return nil }
func (f *fakeRepo) ListDatasets(context.Context) ([]schema.Dataset, error) { return nil, nil }
func (f *fakeRepo) GetDataset(context.Context, string) (*schema.Dataset, error) {
	return nil, nil
}
func (f *fakeRepo) SaveCheckpoint(context.Context, sdk.ImportCheckpoint) error { return nil }
func (f *fakeRepo) GetCheckpoint(context.Context, string) (*sdk.ImportCheckpoint, error) {
	return nil, nil
}
func (f *fakeRepo) SaveImportErrors(context.Context, string, []sdk.ImportError) error {
	return nil
}
func (f *fakeRepo) SaveSyncCheckpoint(context.Context, sdk.SyncCheckpoint) error { return nil }
func (f *fakeRepo) GetSyncCheckpoint(context.Context, string) (*sdk.SyncCheckpoint, error) {
	return nil, nil
}
func (f *fakeRepo) ListSyncCheckpoints(context.Context) ([]sdk.SyncCheckpoint, error) {
	return nil, nil
}
func (f *fakeRepo) SaveMonitorSession(context.Context, sdk.MonitorSessionRow) error { return nil }
func (f *fakeRepo) GetMonitorSession(context.Context, string) (*sdk.MonitorSessionRow, error) {
	return nil, nil
}
func (f *fakeRepo) ListMonitorSessions(context.Context) ([]sdk.MonitorSessionRow, error) {
	return nil, nil
}
func (f *fakeRepo) SaveMonitorEvent(context.Context, sdk.MonitorEventRow) error { return nil }
func (f *fakeRepo) MonitorEventExists(context.Context, string) (bool, error)    { return false, nil }
func (f *fakeRepo) SaveRiskDelta(context.Context, sdk.RiskDeltaRow) error       { return nil }
func (f *fakeRepo) SaveMonitorAlert(context.Context, sdk.MonitorAlertRow) (bool, error) {
	return true, nil
}
func (f *fakeRepo) ListMonitorAlerts(context.Context, string) ([]sdk.MonitorAlertRow, error) {
	return nil, nil
}
func (f *fakeRepo) ListMonitorEvents(context.Context, string, int) ([]sdk.MonitorEventRow, error) {
	return nil, nil
}
func (f *fakeRepo) ListRiskDeltas(context.Context, string) ([]sdk.RiskDeltaRow, error) {
	return nil, nil
}
func (f *fakeRepo) SaveReport(context.Context, sdk.ReportRow) error { return nil }
func (f *fakeRepo) GetReport(context.Context, string) (*sdk.ReportRow, error) {
	return nil, nil
}
func (f *fakeRepo) ListReports(context.Context) ([]sdk.ReportRow, error) { return nil, nil }
func (f *fakeRepo) SaveReportExport(context.Context, sdk.ReportExportRow) error {
	return nil
}

func checkFinite(t *testing.T, fv schema.FeatureVector) {
	t.Helper()
	if len(fv.Values) != len(Order) {
		t.Fatalf("expected %d features, got %d", len(Order), len(fv.Values))
	}
	for _, name := range Order {
		v, ok := fv.Values[name]
		if !ok {
			t.Fatalf("missing feature %q", name)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("feature %q is NaN/Inf: %v", name, v)
		}
	}
	if fv.SchemaVersion != FeatureSchemaVersion {
		t.Fatalf("schema version = %q", fv.SchemaVersion)
	}
}

func TestZeroTransactions(t *testing.T) {
	e := NewEngine(&fakeRepo{txs: map[string][]schema.Transaction{}}, nil)
	fv, err := e.WalletFeatures(context.Background(), "W_absent")
	if err != nil {
		t.Fatal(err)
	}
	checkFinite(t, fv)
	if fv.Values["tx_count"] != 0 {
		t.Fatalf("tx_count = %v, want 0", fv.Values["tx_count"])
	}
}

func TestSingleTransaction(t *testing.T) {
	ts := time.Now().UTC()
	txs := []schema.Transaction{{
		TxID: "TX1", Timestamp: ts, FeeBTC: 0.0001,
		Inputs:  []schema.TransactionInput{{Address: "WX", AmountBTC: 2.0}},
		Outputs: []schema.TransactionOutput{{Address: "WT", AmountBTC: 1.9}},
	}}
	e := NewEngine(&fakeRepo{txs: map[string][]schema.Transaction{"WT": txs}}, nil)
	fv, err := e.WalletFeatures(context.Background(), "WT")
	if err != nil {
		t.Fatal(err)
	}
	checkFinite(t, fv)
	if fv.Values["tx_count"] != 1 {
		t.Fatalf("tx_count = %v", fv.Values["tx_count"])
	}
	if fv.Values["incoming_count"] != 1 {
		t.Fatalf("incoming_count = %v", fv.Values["incoming_count"])
	}
	// Single tx -> zero time span -> tx_per_hour uses eps denom, must be finite.
}

func TestRepeatedTimestampsZeroSpan(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var txs []schema.Transaction
	for i := 0; i < 5; i++ {
		txs = append(txs, schema.Transaction{
			TxID: "TX", Timestamp: ts, // identical timestamps
			Inputs:  []schema.TransactionInput{{Address: "WT", AmountBTC: 1}},
			Outputs: []schema.TransactionOutput{{Address: "WO", AmountBTC: 0.9}},
		})
	}
	e := NewEngine(&fakeRepo{txs: map[string][]schema.Transaction{"WT": txs}}, nil)
	fv, err := e.WalletFeatures(context.Background(), "WT")
	if err != nil {
		t.Fatal(err)
	}
	checkFinite(t, fv) // zero span must not produce Inf in tx_per_hour/velocity
}

func TestExtremeValues(t *testing.T) {
	ts := time.Now().UTC()
	txs := []schema.Transaction{{
		TxID: "TXB", Timestamp: ts,
		Inputs:  []schema.TransactionInput{{Address: "WT", AmountBTC: 1e12}},
		Outputs: []schema.TransactionOutput{{Address: "WO", AmountBTC: 1e-12}},
	}}
	e := NewEngine(&fakeRepo{txs: map[string][]schema.Transaction{"WT": txs}}, nil)
	fv, err := e.WalletFeatures(context.Background(), "WT")
	if err != nil {
		t.Fatal(err)
	}
	checkFinite(t, fv)
}

func TestNoNetworkEvidence(t *testing.T) {
	ts := time.Now().UTC()
	txs := []schema.Transaction{{
		TxID: "TX", Timestamp: ts,
		Outputs: []schema.TransactionOutput{{Address: "WT", AmountBTC: 1}},
	}}
	e := NewEngine(&fakeRepo{txs: map[string][]schema.Transaction{"WT": txs}}, nil)
	fv, _ := e.WalletFeatures(context.Background(), "WT")
	if fv.Values["obs_count"] != 0 || fv.Values["unique_ip_count"] != 0 {
		t.Fatal("network features must be zero when no observations exist")
	}
}
