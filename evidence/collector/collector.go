// Package collector assembles structured, source-linked evidence from ML and
// detector results. Every evidence item references the actual local records
// (txids, cluster members, feature values) that justify it. The collector
// never produces a statement that is not backed by a provided result.
package collector

import (
	"fmt"
	"time"

	"github.com/bctx/bctx/pkg/schema"
)

// AnomalyInput carries what the anomaly result contributes to evidence.
type AnomalyInput struct {
	Score        float64
	ModelVersion string
	TopFeatures  map[string]float64 // optional feature context
}

// FlowInput carries flow contribution.
type FlowInput struct {
	TopClass     string
	Suspicion    float64 // 1 - P(normal)
	ModelVersion string
	SourceTxIDs  []string
	Structural   []StructuralEvidence
}

// StructuralEvidence is a deterministic detector finding.
type StructuralEvidence struct {
	Pattern string
	Score   float64
	Reason  string
}

// EntityInput carries entity contribution.
type EntityInput struct {
	ClusterID   string
	Members     []string
	Confidence  float64
	SourceTxIDs []string
}

// Builder creates evidence items with stable ids.
type Builder struct {
	subject string
	seq     int
	now     time.Time
}

// NewBuilder returns an evidence builder for a subject.
func NewBuilder(subject string) *Builder {
	return &Builder{subject: subject, now: time.Now().UTC()}
}

func (b *Builder) id() string {
	b.seq++
	return fmt.Sprintf("E%d", b.seq)
}

func sev(score float64) schema.Severity {
	switch {
	case score >= 0.85:
		return schema.SeverityHigh
	case score >= 0.6:
		return schema.SeverityMed
	default:
		return schema.SeverityLow
	}
}

// Anomaly builds an evidence item from the anomaly result.
func (b *Builder) Anomaly(in AnomalyInput) schema.EvidenceItem {
	return schema.EvidenceItem{
		ID:            b.id(),
		Type:          "transaction_anomaly",
		Severity:      sev(in.Score),
		Description:   fmt.Sprintf("Behavior deviates from learned baseline (anomaly=%.2f).", in.Score),
		SourceRecords: []string{b.subject},
		Feature:       "anomaly_score",
		Value:         in.Score,
		ModelContrib:  in.Score,
		Confidence:    in.Score,
		CreatedAt:     b.now,
	}
}

// Flow builds the flow ML evidence item plus one item per fired structural
// detector (kept separate from the ML score).
func (b *Builder) Flow(in FlowInput) []schema.EvidenceItem {
	items := []schema.EvidenceItem{{
		ID:            b.id(),
		Type:          "suspicious_flow",
		Severity:      sev(in.Suspicion),
		Description:   fmt.Sprintf("Flow-pattern match: %s-like (suspicion=%.2f).", in.TopClass, in.Suspicion),
		SourceRecords: in.SourceTxIDs,
		Feature:       "flow_suspicion",
		Value:         in.Suspicion,
		ModelContrib:  in.Suspicion,
		Confidence:    in.Suspicion,
		CreatedAt:     b.now,
	}}
	for _, s := range in.Structural {
		if s.Score <= 0 {
			continue
		}
		items = append(items, schema.EvidenceItem{
			ID:            b.id(),
			Type:          "structural_" + s.Pattern,
			Severity:      sev(s.Score),
			Description:   fmt.Sprintf("Structural detector: %s (%s).", s.Pattern, s.Reason),
			SourceRecords: in.SourceTxIDs,
			Feature:       s.Pattern,
			Value:         s.Score,
			Confidence:    s.Score,
			CreatedAt:     b.now,
		})
	}
	return items
}

// Entity builds an evidence item from the entity relationship.
func (b *Builder) Entity(in EntityInput) *schema.EvidenceItem {
	if in.Confidence <= 0 || len(in.Members) <= 1 {
		return nil // no inferred relationship -> no evidence (truthful)
	}
	return &schema.EvidenceItem{
		ID:       b.id(),
		Type:     "entity_relationship",
		Severity: sev(in.Confidence),
		Description: fmt.Sprintf(
			"Inferred common-input relationship: %s (%d members). "+
				"Inferred relationship, not proof of ownership.",
			in.ClusterID, len(in.Members)),
		SourceRecords: in.SourceTxIDs,
		Feature:       "entity_confidence",
		Value:         in.Confidence,
		Confidence:    in.Confidence,
		CreatedAt:     b.now,
	}
}
