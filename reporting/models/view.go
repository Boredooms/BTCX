package models

// ReportView is the fully-derived, render-ready projection of a snapshot. Every
// renderer (json/markdown/html/pdf) consumes this single structure so that all
// formats describe exactly the same facts in the same order. It is produced
// deterministically from the snapshot plus the non-hashed generation metadata
// (generated_at / generated_by) supplied at render time.
//
// The field order here is also the canonical SECTION order used by all
// renderers (design §4.2 / §7.3). It lives in the models package (not the
// parent reporting package) so the format sub-packages can consume it without
// importing reporting, which would create an import cycle.
type ReportView struct {
	Meta ReportMeta `json:"meta"`

	CaseInformation      CaseInformation       `json:"case_information"`
	InvestigationSubject InvestigationSubject  `json:"investigation_subject"`
	AcquisitionSummary   AcquisitionSummary    `json:"acquisition_summary"`
	DatasetProvenance    DatasetProvenance     `json:"dataset_provenance"`
	TransactionSummary   TransactionSummary    `json:"transaction_summary"`
	FlowAnalysis         FlowAnalysis          `json:"flow_analysis"`
	GraphSummary         GraphSummary          `json:"graph_summary"`
	RelatedWallets       RelatedWallets        `json:"related_wallets"`
	EntitySignals        EntitySignals         `json:"entity_signals"`
	NetworkObservations  NetworkObservations   `json:"network_observations"`
	MLSignals            MLSignals             `json:"ml_signals"`
	RiskAssessment       RiskAssessment        `json:"risk_assessment"`
	RiskDeltas           RiskDeltas            `json:"risk_deltas"`
	AlertHistory         AlertHistory          `json:"alert_history"`
	Evidence             Evidence              `json:"evidence"`
	Timeline             Timeline              `json:"timeline"`
	GraphPaths           GraphPaths            `json:"graph_paths"`
	Limitations          Limitations           `json:"limitations"`
	DataQuality          DataQuality           `json:"data_quality"`
	OfflineConnected     OfflineConnectedState `json:"offline_connected_state"`
	GenerationMetadata   GenerationMetadata    `json:"generation_metadata"`
}
