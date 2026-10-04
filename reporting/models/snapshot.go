package models

import (
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// CaseMeta is the provenance-safe case metadata carried in a snapshot. It never
// carries provider credentials or secrets — only an identifier, creation time
// and the acquiring provider's advertised name/version.
type CaseMeta struct {
	ID              string `json:"id"`
	CreatedAt       string `json:"created_at"`
	ProviderName    string `json:"provider_name,omitempty"`
	ProviderVersion string `json:"provider_version,omitempty"`
}

// SyncSummary is the provenance-safe projection of acquisition sync state for a
// subject. Transport/credential fields are excluded; only counts and status
// that describe dataset provenance are kept.
type SyncSummary struct {
	SyncID          string `json:"sync_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	ProviderVersion string `json:"provider_version,omitempty"`
	TargetType      string `json:"target_type,omitempty"`
	Target          string `json:"target,omitempty"`
	Status          string `json:"status,omitempty"`
	Discovered      int    `json:"discovered"`
	Acquired        int    `json:"acquired"`
	Persisted       int    `json:"persisted"`
	Duplicates      int    `json:"duplicates"`
	PartialCount    int    `json:"partial_count"`
	Rejected        int    `json:"rejected"`
	StartedAt       string `json:"started_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

// ReportSnapshot is the frozen, deterministic input to every renderer. It is a
// copy of all local state a report depends on, captured at generation time so
// that an existing report never silently changes when the database changes
// later.
//
// There is deliberately NO wall-clock field on the snapshot: generated_at /
// generated_by live on the report meta and are explicitly excluded from the
// snapshot hash (see reporting.SnapshotHash). Every slice is stored in a total
// order so the serialized snapshot — and therefore its hash — is reproducible.
type ReportSnapshot struct {
	SchemaVersion    string `json:"schema_version"`
	GeneratorVersion string `json:"generator_version"`

	Case CaseMeta `json:"case"`

	// Result is the full investigation result the report describes.
	Result schema.InvestigationResult `json:"result"`

	// Provenance.
	Datasets []RedactedDataset `json:"datasets"`
	Sync     *SyncSummary      `json:"sync,omitempty"`

	// Monitoring state (nil session => never monitored).
	MonitorSession *sdk.MonitorSessionRow `json:"monitor_session,omitempty"`
	MonitorEvents  []sdk.MonitorEventRow  `json:"monitor_events"`
	RiskDeltas     []sdk.RiskDeltaRow     `json:"risk_deltas"`
	MonitorAlerts  []sdk.MonitorAlertRow  `json:"monitor_alerts"`

	// Case-level findings.
	Alerts              []schema.Alert              `json:"alerts"`
	NetworkObservations []schema.NetworkObservation `json:"network_observations"`
	Transactions        []schema.Transaction        `json:"transactions"`

	// Model / feature provenance.
	ModelVersions    map[string]string `json:"model_versions"`
	FeatureSchema    string            `json:"feature_schema"`
	FeatureSchemaSHA string            `json:"feature_schema_sha256"`

	// Traceability IDs (total-ordered).
	EvidenceIDs []string `json:"evidence_ids"`
	TxIDs       []string `json:"tx_ids"`
	AlertIDs    []string `json:"alert_ids"`
}
