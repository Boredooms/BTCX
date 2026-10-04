// Package sdk defines the stable core contracts and the Engine facade that
// every presentation layer (CLI, TUI, future local API) consumes. The engine
// owns no presentation logic; it orchestrates the domain services.
//
// All interfaces take a context.Context for cancellation. Analysis-side
// services must never require network access.
package sdk

import (
	"context"

	"github.com/bctx/bctx/pkg/schema"
)

// Repository is the local persistence boundary. All analysis reads flow
// through here; implementations are backed by SQLite per case.
type Repository interface {
	// Wallet / transaction / network lookups (indexed).
	GetWallet(ctx context.Context, address string) (*schema.Wallet, error)
	GetTransaction(ctx context.Context, txid string) (*schema.Transaction, error)
	WalletTransactions(ctx context.Context, address string, limit int) ([]schema.Transaction, error)
	NetworkObservationsByIP(ctx context.Context, ip string) ([]schema.NetworkObservation, error)
	// AllTransactions streams every transaction (used by the full graph build).
	AllTransactions(ctx context.Context, limit int) ([]schema.Transaction, error)
	// RecentTransactions returns the newest transactions by timestamp (newest
	// first), up to limit — powers the dashboard live-activity feed.
	RecentTransactions(ctx context.Context, limit int) ([]schema.Transaction, error)

	// SaveBlock persists a canonical block header (idempotent by hash).
	SaveBlock(ctx context.Context, b schema.Block) error
	// GetBlock returns a block by hash, or nil if not present locally.
	GetBlock(ctx context.Context, hash string) (*schema.Block, error)
	// GetBlockByHeight returns a block by height, or nil if not present.
	GetBlockByHeight(ctx context.Context, height int) (*schema.Block, error)
	// ListBlocks returns stored blocks newest-first (by height), up to limit.
	ListBlocks(ctx context.Context, limit int) ([]schema.Block, error)
	// AllNetworkObservations returns all observations (used by graph build).
	AllNetworkObservations(ctx context.Context, limit int) ([]schema.NetworkObservation, error)

	// Persistence (used by ingestion/acquisition after normalization).
	SaveTransactions(ctx context.Context, txs []schema.Transaction) error
	SaveNetworkObservations(ctx context.Context, obs []schema.NetworkObservation) error
	SaveEdges(ctx context.Context, edges []schema.GraphEdge) error
	// Graph adjacency reads (used by the graph engine for bounded traversal).
	EdgesFrom(ctx context.Context, nodeID string) ([]schema.GraphEdge, error)
	EdgesTo(ctx context.Context, nodeID string) ([]schema.GraphEdge, error)
	AllEdges(ctx context.Context, limit int) ([]schema.GraphEdge, error)
	DeleteEdges(ctx context.Context) error

	// Evidence / risk / alerts persistence.
	SaveEvidence(ctx context.Context, items []schema.EvidenceItem) error
	SaveRiskAssessment(ctx context.Context, r schema.RiskAssessment) error
	SaveAlert(ctx context.Context, a schema.Alert) error
	ListAlerts(ctx context.Context, limit int) ([]schema.Alert, error)

	// Ingestion support.
	TxExists(ctx context.Context, txid string) (bool, error)
	ObsExists(ctx context.Context, id string) (bool, error)
	SaveDataset(ctx context.Context, d schema.Dataset) error
	ListDatasets(ctx context.Context) ([]schema.Dataset, error)
	GetDataset(ctx context.Context, id string) (*schema.Dataset, error)
	SaveCheckpoint(ctx context.Context, c ImportCheckpoint) error
	GetCheckpoint(ctx context.Context, datasetID string) (*ImportCheckpoint, error)
	SaveImportErrors(ctx context.Context, datasetID string, errs []ImportError) error

	// Acquisition sync checkpoints (Phase 5).
	SaveSyncCheckpoint(ctx context.Context, c SyncCheckpoint) error
	GetSyncCheckpoint(ctx context.Context, syncID string) (*SyncCheckpoint, error)
	ListSyncCheckpoints(ctx context.Context) ([]SyncCheckpoint, error)

	// Monitoring (Phase 6).
	SaveMonitorSession(ctx context.Context, s MonitorSessionRow) error
	GetMonitorSession(ctx context.Context, sessionID string) (*MonitorSessionRow, error)
	ListMonitorSessions(ctx context.Context) ([]MonitorSessionRow, error)
	SaveMonitorEvent(ctx context.Context, e MonitorEventRow) error
	MonitorEventExists(ctx context.Context, eventID string) (bool, error)
	SaveRiskDelta(ctx context.Context, d RiskDeltaRow) error
	SaveMonitorAlert(ctx context.Context, a MonitorAlertRow) (bool, error)
	ListMonitorAlerts(ctx context.Context, sessionID string) ([]MonitorAlertRow, error)
	// Monitoring history reads (Phase 7 reporting).
	ListMonitorEvents(ctx context.Context, sessionID string, limit int) ([]MonitorEventRow, error)
	ListRiskDeltas(ctx context.Context, sessionID string) ([]RiskDeltaRow, error)

	// Reporting (Phase 7).
	SaveReport(ctx context.Context, r ReportRow) error
	GetReport(ctx context.Context, id string) (*ReportRow, error)
	ListReports(ctx context.Context) ([]ReportRow, error)
	SaveReportExport(ctx context.Context, e ReportExportRow) error

	// Audit.
	AppendAudit(ctx context.Context, e schema.AuditEvent) error

	// Counts for status/doctor/home.
	Counts(ctx context.Context) (Counts, error)

	Close() error
}

// ImportCheckpoint persists resumable import progress.
type ImportCheckpoint struct {
	DatasetID     string
	SourcePath    string
	SourceSHA256  string
	Format        string
	ParserVersion string
	RecordsDone   int
	LastBatch     int
	Status        string
	UpdatedAt     string
}

// ImportError is a bounded per-record diagnostic.
type ImportError struct {
	RecordNo int
	Field    string
	Reason   string
	Fragment string
}

// SyncCheckpoint persists resumable provider-acquisition state. Transport/sync
// state only; canonical records live in the normal tables.
type SyncCheckpoint struct {
	SyncID          string
	CaseID          string
	Provider        string
	ProviderVersion string
	TargetType      string // "wallet" | "tx"
	Target          string
	Cursor          string
	CursorPage      int
	PagesDone       int
	Discovered      int
	Acquired        int
	Persisted       int
	Duplicates      int
	PartialCount    int
	Rejected        int
	Retries         int
	Status          string
	Error           string
	StartedAt       string
	UpdatedAt       string
}

// MonitorSessionRow persists a monitoring session (Phase 6).
type MonitorSessionRow struct {
	SessionID       string
	CaseID          string
	Target          string
	TargetType      string
	Provider        string
	ProviderVersion string
	Mode            string
	Status          string
	Health          string
	PollIntervalMS  int
	LastCursor      string
	StartedAt       string
	UpdatedAt       string
	LastEventAt     string
	LastPollOKAt    string
	EventsSeen      int
	EventsNew       int
	EventsDuplicate int
	TxAcquired      int
	AlertsGenerated int
	Reconnects      int
	Gaps            int
	LastRiskScore   int
	Error           string
}

// MonitorEventRow is one bounded operational event record.
type MonitorEventRow struct {
	EventID     string
	SessionID   string
	Type        string
	TxID        string
	Confirmed   bool
	BlockHeight int
	FirstSeen   string
	Timestamp   string
}

// RiskDeltaRow is a factual risk-change record.
type RiskDeltaRow struct {
	ID             string
	SessionID      string
	Subject        string
	PreviousScore  int
	CurrentScore   int
	Delta          int
	PreviousConf   float64
	CurrentConf    float64
	ChangedSignals []string
	NewPatterns    []string
	NewEvidenceIDs []string
	TriggerEvent   string
	Timestamp      string
}

// MonitorAlertRow is a deduplicated monitor alert.
type MonitorAlertRow struct {
	AlertID     string
	DedupKey    string
	SessionID   string
	CaseID      string
	Subject     string
	Trigger     string
	Severity    string
	RiskBefore  int
	RiskAfter   int
	Delta       int
	EvidenceIDs []string
	TxIDs       []string
	Reason      string
	Timestamp   string
}

// ReportRow persists a forensic report (Phase 7). The first four fields map to
// the schema-v1 (0001) NOT NULL columns; the remaining fields map to the
// additive 0007 columns and read back empty for legacy rows.
type ReportRow struct {
	// 0001 NOT NULL columns.
	ID          string
	CaseID      string
	Version     int
	Subject     string
	ResultJSON  string
	GeneratedAt string
	// 0001 nullable columns.
	ModelVersions   string
	DatasetSnapshot string
	// 0007 additive columns (nullable).
	ReportSchemaVersion string
	GeneratorVersion    string
	InvestigationID     string
	SubjectType         string
	SnapshotSHA256      string
	SnapshotJSON        string
	GeneratedBy         string
}

// ReportExportRow persists a bundled report export (Phase 7).
type ReportExportRow struct {
	ExportID       string
	ReportID       string
	CaseID         string
	BundlePath     string
	Formats        string
	ManifestSHA256 string
	CreatedAt      string
}

// Counts summarizes local dataset size for status displays.
type Counts struct {
	Transactions int
	Wallets      int
	NetworkRecs  int
	Edges        int
	Alerts       int
}

// DataSource is an acquisition adapter. It is the ONLY contract permitted to
// use the network. Its job ends once data is normalized and ready to persist.
type DataSource interface {
	Name() string
	GetAddress(ctx context.Context, address string) (*schema.Wallet, error)
	GetAddressTransactions(ctx context.Context, address, cursor string) ([]schema.Transaction, string, error)
	GetTransaction(ctx context.Context, txid string) (*schema.Transaction, error)
}

// GraphService builds and queries bounded subgraphs over local data.
type GraphService interface {
	Neighbors(ctx context.Context, id string) ([]schema.GraphNode, error)
	Subgraph(ctx context.Context, center string, depth int) (*schema.Subgraph, error)
	Path(ctx context.Context, src, dst string) ([]string, error)
}

// FeatureService computes versioned feature vectors from local data.
type FeatureService interface {
	WalletFeatures(ctx context.Context, address string) (schema.FeatureVector, error)
	TransactionFeatures(ctx context.Context, txid string) (schema.FeatureVector, error)
}

// MLService runs local model inference. Implementations load local ONNX models
// and must never download anything at runtime.
type MLService interface {
	Predict(ctx context.Context, model string, fv schema.FeatureVector) (schema.Prediction, error)
	ModelInfo(model string) (ModelInfo, error)
	Models() []ModelInfo
}

// ModelInfo describes a loaded/declared local model.
type ModelInfo struct {
	Name          string
	Version       string
	FeatureSchema string
	Checksum      string
	Loaded        bool
	Path          string
}

// DetectionService runs suspicious-flow detectors and clustering.
type DetectionService interface {
	DetectPatterns(ctx context.Context, subject string) ([]schema.PatternResult, error)
	Cluster(ctx context.Context, subject string) ([]schema.EntityCluster, error)
}

// RiskService aggregates signals into an explainable risk assessment.
type RiskService interface {
	Score(ctx context.Context, in RiskInput) (schema.RiskAssessment, error)
	Propagate(ctx context.Context, seed string, depth int) ([]schema.RiskPropagationStep, error)
}

// RiskInput bundles the signals the risk engine aggregates.
type RiskInput struct {
	Subject     string
	SubjectType schema.NodeType
	Predictions []schema.Prediction
	Patterns    []schema.PatternResult
	Clusters    []schema.EntityCluster
	Propagation []schema.RiskPropagationStep
}

// EvidenceService assembles source-linked evidence for a result.
type EvidenceService interface {
	Build(ctx context.Context, result *schema.InvestigationResult) ([]schema.EvidenceItem, error)
}

// ReportService renders an investigation report to local files.
type ReportService interface {
	Build(ctx context.Context, result schema.InvestigationResult) (schema.Report, error)
	Export(ctx context.Context, r schema.Report, format schema.ReportFormat, outPath string) error
}

// CaseService manages isolated investigation cases.
type CaseService interface {
	Create(ctx context.Context, name string) (schema.Case, error)
	Open(ctx context.Context, name string) (schema.Case, error)
	List(ctx context.Context) ([]schema.Case, error)
	Close(ctx context.Context, name string) error
	Active() (schema.Case, bool)
}
