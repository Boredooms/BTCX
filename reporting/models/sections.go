package models

// NotAvailable is the single sentinel string a renderer must emit for data that
// is genuinely absent. A report must never fabricate a value: an empty field is
// rendered as this explicit marker, never as a plausible-looking default.
const NotAvailable = "not available"

// ReportMeta is the identity block of a rendered report. report_id, case_id,
// investigation_id, subject, subject_type, schema_version and report_version
// are reproducible from the snapshot; generated_at / generated_by are the
// non-hashed wall-clock stamps added at render time.
type ReportMeta struct {
	ReportID        string `json:"report_id"`
	CaseID          string `json:"case_id"`
	InvestigationID string `json:"investigation_id"`
	Subject         string `json:"subject"`
	SubjectType     string `json:"subject_type"`
	GeneratedAt     string `json:"generated_at"`
	GeneratedBy     string `json:"generated_by"`
	SchemaVersion   string `json:"schema_version"`
	ReportVersion   string `json:"report_version"`
}

// The 21 typed section view-models (design §4.2). Each is a plain data struct a
// renderer walks; absent fields carry the NotAvailable marker, never a
// fabricated value.

// CaseInformation summarizes the owning case.
type CaseInformation struct {
	CaseID    string `json:"case_id"`
	CreatedAt string `json:"created_at"`
	Provider  string `json:"provider"`
}

// InvestigationSubject identifies what was investigated.
type InvestigationSubject struct {
	Subject     string `json:"subject"`
	SubjectType string `json:"subject_type"`
	Offline     bool   `json:"offline"`
}

// AcquisitionSummary describes how the underlying data was acquired.
type AcquisitionSummary struct {
	Provider        string `json:"provider"`
	ProviderVersion string `json:"provider_version"`
	Status          string `json:"status"`
	Discovered      int    `json:"discovered"`
	Acquired        int    `json:"acquired"`
	Persisted       int    `json:"persisted"`
	Available       bool   `json:"available"`
}

// DatasetProvenance lists the redacted dataset provenance records.
type DatasetProvenance struct {
	Datasets []RedactedDataset `json:"datasets"`
}

// TransactionSummary aggregates the subject's transactions.
type TransactionSummary struct {
	Count       int      `json:"count"`
	TotalInBTC  float64  `json:"total_in_btc"`
	TotalOutBTC float64  `json:"total_out_btc"`
	TotalInSats int64    `json:"total_in_sats"`
	TotalOutSat int64    `json:"total_out_sats"`
	TxIDs       []string `json:"tx_ids"`
}

// FlowAnalysis summarizes value flow direction/shape.
type FlowAnalysis struct {
	TotalFanIn  int `json:"total_fan_in"`
	TotalFanOut int `json:"total_fan_out"`
}

// GraphSummary describes the extracted subgraph size.
type GraphSummary struct {
	Available bool   `json:"available"`
	Center    string `json:"center"`
	Depth     int    `json:"depth"`
	NodeCount int    `json:"node_count"`
	EdgeCount int    `json:"edge_count"`
}

// RelatedWallets reports inferred wallet relationships (clusters).
type RelatedWallets struct {
	Count        int      `json:"count"`
	Relationship string   `json:"relationship"`
	Members      []string `json:"members"`
}

// EntitySignals lists inferred entity clusters.
type EntitySignals struct {
	ClusterCount int `json:"cluster_count"`
}

// NetworkObservations summarizes correlated network telemetry.
type NetworkObservations struct {
	Count int `json:"count"`
}

// MLSignals summarizes local model predictions.
type MLSignals struct {
	Predictions []MLPrediction `json:"predictions"`
}

// MLPrediction is one model output for the subject.
type MLPrediction struct {
	Model         string  `json:"model"`
	ModelVersion  string  `json:"model_version"`
	FeatureSchema string  `json:"feature_schema"`
	Score         float64 `json:"score"`
	Confidence    float64 `json:"confidence"`
}

// RiskAssessment summarizes the explainable risk result.
type RiskAssessment struct {
	Score      int          `json:"score"`
	Confidence float64      `json:"confidence"`
	Signals    []RiskSignal `json:"signals"`
}

// RiskSignal is one risk contributor.
type RiskSignal struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// RiskDeltas lists factual monitoring risk-change records.
type RiskDeltas struct {
	Deltas []RiskDeltaEntry `json:"deltas"`
}

// RiskDeltaEntry is one risk change over time.
type RiskDeltaEntry struct {
	Subject       string `json:"subject"`
	PreviousScore int    `json:"previous_score"`
	CurrentScore  int    `json:"current_score"`
	Delta         int    `json:"delta"`
	Timestamp     string `json:"timestamp"`
}

// AlertHistory lists ranked findings with approved status wording.
type AlertHistory struct {
	Alerts []AlertEntry `json:"alerts"`
}

// AlertEntry is one alert with human-safe status text.
type AlertEntry struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Risk       int     `json:"risk"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	Reason     string  `json:"reason"`
	CreatedAt  string  `json:"created_at"`
}

// Evidence lists source-linked evidence items.
type Evidence struct {
	Items []EvidenceEntry `json:"items"`
}

// EvidenceEntry is one evidence item projection.
type EvidenceEntry struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Severity      string   `json:"severity"`
	Description   string   `json:"description"`
	SourceRecords []string `json:"source_records"`
	Confidence    float64  `json:"confidence"`
}

// Timeline is the ordered sequence of time-stamped events.
type Timeline struct {
	Events []TimelineEvent `json:"events"`
}

// GraphPaths lists risk-propagation paths through the graph.
type GraphPaths struct {
	Paths []GraphPathEntry `json:"paths"`
}

// GraphPathEntry is one propagation path.
type GraphPathEntry struct {
	Seed         string   `json:"seed"`
	Target       string   `json:"target"`
	Distance     int      `json:"distance"`
	Contribution float64  `json:"contribution"`
	Path         []string `json:"path"`
}

// Limitations states what the report does NOT establish.
type Limitations struct {
	Notes []string `json:"notes"`
}

// DataQuality reports completeness/coverage caveats.
type DataQuality struct {
	TransactionCount        int  `json:"transaction_count"`
	NetworkObservationCount int  `json:"network_observation_count"`
	HasGraph                bool `json:"has_graph"`
}

// OfflineConnectedState reports whether the result was produced offline and the
// classified monitor state.
type OfflineConnectedState struct {
	Offline      bool   `json:"offline"`
	MonitorState string `json:"monitor_state"`
	Detail       string `json:"detail"`
}

// GenerationMetadata records how/when the report was generated.
type GenerationMetadata struct {
	GeneratedAt      string            `json:"generated_at"`
	GeneratedBy      string            `json:"generated_by"`
	SchemaVersion    string            `json:"schema_version"`
	GeneratorVersion string            `json:"generator_version"`
	SnapshotSHA256   string            `json:"snapshot_sha256"`
	ModelVersions    map[string]string `json:"model_versions"`
	FeatureSchema    string            `json:"feature_schema"`
}
