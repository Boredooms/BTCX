// Package models holds the frozen, deterministic data contracts used by the
// reporting layer: the report snapshot, the typed section view-models, the
// controlled vocabulary, provenance redaction, the timeline/manifest types and
// the monitor-state classifier. It is NETWORK-FORBIDDEN like its parent.
package models

import (
	"strings"

	"github.com/bctx/bctx/pkg/schema"
)

// Controlled vocabulary. Reports must describe findings using evidence-truthful
// language and must never upgrade an anomaly or pattern match into a claim of
// criminal intent or proven real-world ownership (AGENTS.md §evidence wording).
//
// These constants are the ONLY approved verbs/nouns a renderer should use when
// summarizing an inferred finding.
const (
	// WordObserved describes something present in the local records as-is.
	WordObserved = "observed"
	// WordDetected describes a detector/model firing on local data.
	WordDetected = "detected"
	// WordInferred describes a derived relationship that is not proven fact.
	WordInferred = "inferred"
	// WordPatternMatch describes a pattern detector match.
	WordPatternMatch = "pattern match"
	// WordAnomaly describes a statistical outlier, not a verdict.
	WordAnomaly = "anomaly"
	// WordEvidenceIndicates is the strongest approved framing: it points at
	// source-linked evidence without asserting a conclusion.
	WordEvidenceIndicates = "evidence indicates"
)

// ClusterRelationshipPhrase is the approved phrasing for how an entity cluster
// relationship must be described: inferred, never proven ownership.
const ClusterRelationshipPhrase = "inferred relationship, not proven ownership"

// ForbiddenPhrases is an exported denylist of language a report must never
// contain. Renderers and tests check generated prose against this list. Match
// is case-insensitive substring (see ContainsForbidden).
var ForbiddenPhrases = []string{
	"criminal",
	"illegal",
	"proven owner",
	"proven ownership",
	"guilty",
	"laundering confirmed",
	"confirmed criminal",
	"money launderer",
	"same owner proven",
	"beyond doubt",
	"definitely",
}

// ContainsForbidden reports whether s contains any forbidden phrase
// (case-insensitive). The matched phrase is returned for diagnostics.
func ContainsForbidden(s string) (string, bool) {
	low := strings.ToLower(s)
	for _, p := range ForbiddenPhrases {
		if strings.Contains(low, p) {
			return p, true
		}
	}
	return "", false
}

// AlertStatusText maps an alert review state to the approved human phrasing.
// CONFIRMED is an analyst review state, not an automatic legal determination,
// so it is rendered as "analyst-confirmed (review state)".
func AlertStatusText(s schema.AlertStatus) string {
	switch s {
	case schema.AlertNew:
		return "new"
	case schema.AlertReviewing:
		return "under review"
	case schema.AlertDismissed:
		return "dismissed"
	case schema.AlertConfirmed:
		return "analyst-confirmed (review state)"
	case schema.AlertExported:
		return "exported"
	default:
		return string(s)
	}
}

// PatternTypeText returns the display name for a pattern type. The "-like"
// suffix in the underlying schema constants is intentionally preserved so a
// detector match is never read as a proven classification.
func PatternTypeText(t schema.PatternType) string {
	return string(t)
}
