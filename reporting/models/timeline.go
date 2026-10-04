package models

// TimelineEventKind enumerates the sources a timeline event can originate from.
type TimelineEventKind string

const (
	// KindTx is a transaction timestamp.
	KindTx TimelineEventKind = "tx"
	// KindMonitorEvent is a captured monitoring event.
	KindMonitorEvent TimelineEventKind = "monitor_event"
	// KindRiskDelta is a factual risk-change record.
	KindRiskDelta TimelineEventKind = "risk_delta"
	// KindAlert is a ranked finding.
	KindAlert TimelineEventKind = "alert"
)

// TimelineEvent is one time-ordered entry assembled from the snapshot. Timestamp
// is an RFC3339 UTC string so that lexical ordering is chronological, matching
// the storage layer's ordering guarantee.
type TimelineEvent struct {
	Kind      TimelineEventKind `json:"kind"`
	Timestamp string            `json:"timestamp"`
	ID        string            `json:"id"`
	Detail    string            `json:"detail"`
}
