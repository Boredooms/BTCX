package schema

import "time"

// CaseStatus is the lifecycle state of an investigation case.
type CaseStatus string

const (
	CaseActive   CaseStatus = "active"
	CaseClosed   CaseStatus = "closed"
	CaseArchived CaseStatus = "archived"
)

// Case is an isolated investigation workspace. Each case owns its own
// database file and evidence/report directories under ~/.bctx/cases/<id>/.
type Case struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        CaseStatus        `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Datasets      []string          `json:"datasets"`
	ModelVersions map[string]string `json:"model_versions,omitempty"`
}

// Dataset describes an imported or acquired data batch with its provenance.
type Dataset struct {
	ID            string    `json:"id"`
	CaseID        string    `json:"case_id"`
	SourceFile    string    `json:"source_file"`
	SHA256        string    `json:"sha256"`
	SchemaVersion string    `json:"schema_version"`
	RecordsRead   int       `json:"records_read"`
	RecordsValid  int       `json:"records_valid"`
	RecordsReject int       `json:"records_rejected"`
	Transactions  int       `json:"transactions"`
	Wallets       int       `json:"wallets"`
	NetworkRecs   int       `json:"network_records"`
	ImportedAt    time.Time `json:"imported_at"`
	ToolVersion   string    `json:"tool_version"`
	// Phase 4 ingestion provenance.
	Format          string `json:"format,omitempty"`
	ParserVersion   string `json:"parser_version,omitempty"`
	ImporterVersion string `json:"importer_version,omitempty"`
	Status          string `json:"status,omitempty"`
	Duplicates      int    `json:"duplicates,omitempty"`
	PartialCount    int    `json:"partial_count,omitempty"`
}

// MonitorStatus is the state of a monitoring session.
type MonitorStatus string

const (
	MonitorRunning MonitorStatus = "RUNNING"
	MonitorPaused  MonitorStatus = "PAUSED"
	MonitorStopped MonitorStatus = "STOPPED"
)

// MonitorSession persists the state of a wallet monitor so analysis can resume
// from the last captured local event after a disconnect/reconnect.
type MonitorSession struct {
	ID          string        `json:"id"`
	CaseID      string        `json:"case_id"`
	Subject     string        `json:"subject"`
	Status      MonitorStatus `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	LastSyncAt  time.Time     `json:"last_sync_at"`
	LastSeenTx  string        `json:"last_seen_tx"`
	TxCaptured  int           `json:"tx_captured"`
	RiskBefore  int           `json:"risk_before"`
	RiskAfter   int           `json:"risk_after"`
	AlertsCount int           `json:"alerts_count"`
}

// MonitorEvent is a single captured monitoring event.
type MonitorEvent struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	TxID      string    `json:"txid"`
	AmountBTC float64   `json:"amount_btc"`
	Timestamp time.Time `json:"timestamp"`
	// Simulated marks events produced by a demo/fixture source rather than a
	// live network. Never label simulated events as LIVE.
	Simulated bool `json:"simulated"`
}

// AuditEvent is a local, non-telemetry record of an operator action.
type AuditEvent struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	Action    string    `json:"action"`
	Subject   string    `json:"subject"`
	Session   string    `json:"session,omitempty"`
	Result    string    `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

// Audit action constants.
const (
	AuditDatasetImported     = "DATASET_IMPORTED"
	AuditWalletAnalyzed      = "WALLET_ANALYZED"
	AuditTransactionAnalyzed = "TRANSACTION_ANALYZED"
	AuditGraphExpanded       = "GRAPH_EXPANDED"
	AuditModelExecuted       = "MODEL_EXECUTED"
	AuditRiskUpdated         = "RISK_UPDATED"
	AuditAlertCreated        = "ALERT_CREATED"
	AuditReportGenerated     = "REPORT_GENERATED"
	AuditReportExported      = "REPORT_EXPORTED"
	AuditMonitorStarted      = "MONITOR_STARTED"
	AuditMonitorStopped      = "MONITOR_STOPPED"
	AuditCaseCreated         = "CASE_CREATED"
)
