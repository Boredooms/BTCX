package schema

import "time"

// -----------------------------------------------------------------------------
// Features
// -----------------------------------------------------------------------------

// FeatureVector is the versioned numeric representation of a subject
// (wallet/transaction) consumed by ML models and the risk engine.
type FeatureVector struct {
	Subject       string             `json:"subject"`
	SubjectType   NodeType           `json:"subject_type"`
	SchemaVersion string             `json:"schema_version"`
	Values        map[string]float64 `json:"values"`
	ComputedAt    time.Time          `json:"computed_at"`
}

// Get returns a feature value and whether it was present.
func (f FeatureVector) Get(name string) (float64, bool) {
	v, ok := f.Values[name]
	return v, ok
}

// -----------------------------------------------------------------------------
// ML predictions
// -----------------------------------------------------------------------------

// Prediction is the output of a single local model inference.
type Prediction struct {
	Model         string    `json:"model"`
	ModelVersion  string    `json:"model_version"`
	FeatureSchema string    `json:"feature_schema"`
	Subject       string    `json:"subject"`
	Score         float64   `json:"score"`      // normalized 0..1
	Confidence    float64   `json:"confidence"` // 0..1
	Timestamp     time.Time `json:"timestamp"`
}

// -----------------------------------------------------------------------------
// Pattern detection
// -----------------------------------------------------------------------------

// PatternType enumerates suspicious-flow detectors. Names are deliberately
// "-like" to avoid asserting proof of criminal activity.
type PatternType string

const (
	PatternPeelingChain PatternType = "peeling_chain_like"
	PatternMixing       PatternType = "mixing_like"
	PatternFanOut       PatternType = "high_fan_out"
	PatternFanIn        PatternType = "high_fan_in"
	PatternRapidFlow    PatternType = "rapid_flow"
)

// PatternResult is the structured output of a detector.
type PatternResult struct {
	Type        PatternType `json:"type"`
	Score       float64     `json:"score"`
	Confidence  float64     `json:"confidence"`
	Subject     string      `json:"subject"`
	SourceTxIDs []string    `json:"source_txids"`
	EvidenceIDs []string    `json:"evidence_ids"`
	Description string      `json:"description"`
}

// -----------------------------------------------------------------------------
// Risk
// -----------------------------------------------------------------------------

// RiskSignal is one contributing component of a risk score.
type RiskSignal struct {
	Name        string   `json:"name"`
	Score       float64  `json:"score"`  // 0..1 contribution strength
	Weight      float64  `json:"weight"` // configured aggregation weight
	Description string   `json:"description"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// RiskAssessment is the explainable risk result for a subject.
type RiskAssessment struct {
	Subject     string       `json:"subject"`
	SubjectType NodeType     `json:"subject_type"`
	Score       int          `json:"score"`      // 0..100
	Band        RiskBand     `json:"band"`       // presentation bucket of Score
	Confidence  float64      `json:"confidence"` // 0..1
	Signals     []RiskSignal `json:"signals"`
	// Previous/Delta support monitoring risk-change reporting.
	Previous  *int      `json:"previous,omitempty"`
	Delta     *int      `json:"delta,omitempty"`
	ModelInfo string    `json:"model_info,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RiskBand is a PRESENTATION bucket for a 0..100 risk score. It adds no new
// forensic meaning — the numeric Score is always authoritative and always
// shown; the band is only a label for triage. Thresholds are documented and
// fixed so the TUI/CLI/report render identically.
type RiskBand string

const (
	BandLow      RiskBand = "LOW"      // 0..29   — no notable signals
	BandMedium   RiskBand = "MEDIUM"   // 30..54  — mild/ambiguous signals
	BandElevated RiskBand = "ELEVATED" // 55..74  — a clear signal present
	BandHigh     RiskBand = "HIGH"     // 75..89  — multiple strong signals
	BandCritical RiskBand = "CRITICAL" // 90..100 — convergent strong signals
)

// BandFor maps a 0..100 score to its presentation band (documented thresholds).
func BandFor(score int) RiskBand {
	switch {
	case score >= 90:
		return BandCritical
	case score >= 75:
		return BandHigh
	case score >= 55:
		return BandElevated
	case score >= 30:
		return BandMedium
	default:
		return BandLow
	}
}

// RiskPropagationStep records how risk propagated from a seed through the graph.
type RiskPropagationStep struct {
	Seed         string   `json:"seed"`
	Target       string   `json:"target"`
	Distance     int      `json:"distance"`
	Contribution float64  `json:"contribution"`
	Path         []string `json:"path"`
}

// -----------------------------------------------------------------------------
// Evidence
// -----------------------------------------------------------------------------

// Severity classifies evidence/alert importance.
type Severity string

const (
	SeverityHigh Severity = "HIGH"
	SeverityMed  Severity = "MED"
	SeverityLow  Severity = "LOW"
)

// EvidenceItem is a structured, source-linked justification for a finding. The
// evidence engine must never produce statements not backed by source records.
type EvidenceItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Severity    Severity `json:"severity"`
	Description string   `json:"description"`
	// SourceRecords references the actual local records (txids, obs ids, etc.).
	SourceRecords []string  `json:"source_records"`
	Feature       string    `json:"feature,omitempty"`
	Value         float64   `json:"value,omitempty"`
	ModelContrib  float64   `json:"model_contribution,omitempty"`
	Confidence    float64   `json:"confidence"`
	CreatedAt     time.Time `json:"created_at"`
}

// -----------------------------------------------------------------------------
// Alerts
// -----------------------------------------------------------------------------

// AlertStatus is the analyst review lifecycle state. "CONFIRMED" is an analyst
// review state, not an automatic legal determination.
type AlertStatus string

const (
	AlertNew       AlertStatus = "NEW"
	AlertReviewing AlertStatus = "REVIEWING"
	AlertDismissed AlertStatus = "DISMISSED"
	AlertConfirmed AlertStatus = "CONFIRMED"
	AlertExported  AlertStatus = "EXPORTED"
)

// Alert is a ranked, explainable finding.
type Alert struct {
	ID          string      `json:"id"`
	CaseID      string      `json:"case_id"`
	Subject     string      `json:"subject"`
	SubjectType NodeType    `json:"subject_type"`
	Type        string      `json:"type"`
	Risk        int         `json:"risk"`
	Confidence  float64     `json:"confidence"`
	Priority    int         `json:"priority"`
	Reason      string      `json:"reason"`
	EvidenceIDs []string    `json:"evidence_ids"`
	Status      AlertStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

// -----------------------------------------------------------------------------
// Investigation result + report
// -----------------------------------------------------------------------------

// InvestigationResult is the single structured object produced by the
// investigation engine and consumed identically by CLI, TUI and reports.
type InvestigationResult struct {
	ID             string                `json:"id"`
	CaseID         string                `json:"case_id"`
	Subject        string                `json:"subject"`
	SubjectType    NodeType              `json:"subject_type"`
	Offline        bool                  `json:"offline"`
	Risk           RiskAssessment        `json:"risk"`
	Predictions    []Prediction          `json:"predictions"`
	Patterns       []PatternResult       `json:"patterns"`
	Clusters       []EntityCluster       `json:"clusters"`
	Propagation    []RiskPropagationStep `json:"propagation"`
	Evidence       []EvidenceItem        `json:"evidence"`
	RelatedWallets int                   `json:"related_wallets"`
	RelevantTxs    int                   `json:"relevant_transactions"`
	Subgraph       *Subgraph             `json:"subgraph,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
}

// ReportFormat enumerates export formats.
type ReportFormat string

const (
	FormatJSON     ReportFormat = "json"
	FormatMarkdown ReportFormat = "md"
	FormatHTML     ReportFormat = "html"
	FormatPDF      ReportFormat = "pdf"
)

// Report wraps an investigation result with report metadata for export. The
// fields are additive over schema-v1: the original columns (ID, CaseID,
// Version, Result, ModelVersions, DatasetSnapshot, GeneratedAt) are preserved,
// and Phase 7 reporting metadata is appended. Legacy rows read back with the
// new fields empty.
type Report struct {
	ID              string              `json:"id"`
	CaseID          string              `json:"case_id"`
	Version         int                 `json:"version"`
	Result          InvestigationResult `json:"result"`
	ModelVersions   map[string]string   `json:"model_versions"`
	DatasetSnapshot string              `json:"dataset_snapshot"`
	GeneratedAt     time.Time           `json:"generated_at"`
	// Phase 7 reporting metadata (additive).
	InvestigationID string   `json:"investigation_id"`
	SubjectType     NodeType `json:"subject_type"`
	SchemaVersion   string   `json:"schema_version"`
	ReportVersion   string   `json:"report_version"`
	GeneratedBy     string   `json:"generated_by"`
	SnapshotSHA256  string   `json:"snapshot_sha256"`
}
