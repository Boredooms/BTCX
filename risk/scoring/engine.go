// Package scoring implements the deterministic BCTX risk engine.
//
// Risk combines ML signals (anomaly, flow) and deterministic signals (entity
// relationship, structural flow) into an explainable 0..100 score with a
// per-signal breakdown and linked evidence. The engine is pure Go and fully
// deterministic given the same inputs and configured weights. Models feed risk;
// the LLM never calculates risk.
//
// Risk is an investigative prioritization signal, NOT proof of wrongdoing.
package scoring

import (
	"math"
	"sort"
	"time"

	"github.com/bctx/bctx/pkg/schema"
)

// Weights configures signal aggregation. Stored locally / tunable per
// deployment; defaults are conservative and documented.
type Weights struct {
	Anomaly float64
	Flow    float64
	Entity  float64
}

// DefaultWeights are the baseline aggregation weights.
func DefaultWeights() Weights {
	return Weights{Anomaly: 0.5, Flow: 0.35, Entity: 0.15}
}

// Inputs bundles the signals the engine aggregates.
type Inputs struct {
	Subject     string
	SubjectType schema.NodeType

	// AnomalyScore in [0,1] (calibrated).
	AnomalyScore   float64
	AnomalyConf    float64
	AnomalyEvidIDs []string

	// FlowSuspicion in [0,1] = 1 - P(normal) from the flow model.
	FlowSuspicion float64
	FlowTopClass  string
	FlowConf      float64
	FlowEvidIDs   []string

	// EntityStrength in [0,1] = common-input heuristic confidence.
	EntityStrength float64
	EntityMembers  int
	EntityEvidIDs  []string

	// Structural in [0,1] is the strongest DETERMINISTIC structural-detector
	// score (peeling/mixing/fan-out/fan-in/rapid-flow). Unlike the ML anomaly —
	// which saturates for out-of-distribution inputs — the structural detectors
	// are specific and trustworthy, so they corroborate (or fail to corroborate)
	// the anomaly signal. See Score() for how it gates a saturated anomaly.
	Structural float64

	// Previous score, for monitoring deltas (optional).
	Previous *int
}

// Engine computes risk assessments.
type Engine struct {
	W Weights
}

// New builds a risk engine with the given weights.
func New(w Weights) *Engine { return &Engine{W: w} }

// Score aggregates signals into an explainable RiskAssessment.
func (e *Engine) Score(in Inputs) schema.RiskAssessment {
	w := e.W
	// Normalize weights so the combination stays in [0,1].
	wsum := w.Anomaly + w.Flow + w.Entity
	if wsum == 0 {
		wsum = 1
	}

	// Corroboration gate on the ML anomaly. The Isolation-Forest anomaly score
	// saturates to ~1.0 for inputs outside its training distribution, so a lone
	// high anomaly with NO corroborating structural pattern and NO entity
	// relationship is unreliable. We damp an uncorroborated anomaly toward a
	// floor so a structurally-benign wallet reads LOW/MEDIUM, while a wallet
	// whose structure OR entity actually corroborates keeps the full anomaly.
	// This changes no model and no weight — it conditions the anomaly SIGNAL on
	// deterministic, trustworthy evidence.
	corroboration := math.Max(clamp01(in.Structural), clamp01(in.EntityStrength))
	anomalyEffective := clamp01(in.AnomalyScore)
	if anomalyEffective > corroboration {
		// Blend toward the corroboration level: fully trusted only when
		// corroborated, otherwise pulled down (floor keeps some signal). The
		// floor is deliberately low so a FULLY uncorroborated, structurally
		// benign wallet (no structural pattern, no entity relationship) reads
		// LOW — a lone saturated ML anomaly is weak evidence on its own and must
		// not pin an otherwise-clean wallet above the LOW band. Corroboration
		// scales the signal back up along a sqrt curve, so even moderate real
		// corroboration restores most of the signal (a genuinely corroborated
		// wallet still reads HIGH/CRITICAL); only the fully uncorroborated case
		// is held at the floor.
		const uncorroboratedFloor = 0.15
		damp := uncorroboratedFloor + (1-uncorroboratedFloor)*math.Sqrt(corroboration)
		anomalyEffective *= damp
	}

	// Corroboration gate on the ML FLOW signal too. The flow ONNX classifier
	// also saturates for out-of-distribution inputs (it labels almost any tiny
	// synthetic wallet "peeling_chain" at ~1.0). A flow suspicion that neither
	// the deterministic structural detectors NOR an entity relationship
	// corroborate is unreliable, so we damp it toward that same corroboration
	// level (max of structural + entity) exactly like the anomaly. A wallet
	// whose real structure IS peeling/mixing/fan-* — or that sits in a real
	// common-input cluster — keeps the full flow signal; a benign, isolated
	// wallet reads low. This changes no model and no weight — it conditions the
	// flow SIGNAL on trustworthy, rule-based/heuristic evidence.
	flowEffective := clamp01(in.FlowSuspicion)
	if flowEffective > corroboration {
		const uncorroboratedFloor = 0.10
		damp := uncorroboratedFloor + (1-uncorroboratedFloor)*math.Sqrt(corroboration)
		flowEffective *= damp
	}

	signals := []schema.RiskSignal{
		{
			Name:        "transaction_anomaly",
			Score:       anomalyEffective,
			Weight:      w.Anomaly / wsum,
			Description: "Behavior vs learned-normal (Isolation Forest), corroboration-gated.",
			EvidenceIDs: in.AnomalyEvidIDs,
		},
		{
			Name:        "suspicious_flow",
			Score:       flowEffective,
			Weight:      w.Flow / wsum,
			Description: "Flow-pattern match (top: " + in.FlowTopClass + "), structure-gated.",
			EvidenceIDs: in.FlowEvidIDs,
		},
		{
			Name:        "entity_relationship",
			Score:       clamp01(in.EntityStrength),
			Weight:      w.Entity / wsum,
			Description: "Inferred common-input relationship.",
			EvidenceIDs: in.EntityEvidIDs,
		},
	}

	var combined float64
	for _, s := range signals {
		combined += s.Score * s.Weight
	}
	score100 := int(math.Round(clamp01(combined) * 100))

	// Confidence: weighted mean of per-signal confidences, falling back to the
	// signal strength where a model confidence is not provided.
	conf := weightedConfidence(in, signals)

	// Rank signals by contribution for presentation.
	sort.SliceStable(signals, func(i, j int) bool {
		return signals[i].Score*signals[i].Weight > signals[j].Score*signals[j].Weight
	})

	ra := schema.RiskAssessment{
		Subject:     in.Subject,
		SubjectType: in.SubjectType,
		Score:       score100,
		Band:        schema.BandFor(score100),
		Confidence:  conf,
		Signals:     signals,
		ModelInfo:   "anomaly+flow(ONNX via pure-Go tree eval)+entity(common-input)",
		CreatedAt:   time.Now().UTC(),
	}
	if in.Previous != nil {
		prev := *in.Previous
		delta := score100 - prev
		ra.Previous = &prev
		ra.Delta = &delta
	}
	return ra
}

func weightedConfidence(in Inputs, signals []schema.RiskSignal) float64 {
	confs := []struct{ c, w float64 }{
		{orDefault(in.AnomalyConf, in.AnomalyScore), signals[0].Weight},
		{orDefault(in.FlowConf, in.FlowSuspicion), signals[1].Weight},
		{orDefault(0, in.EntityStrength), signals[2].Weight},
	}
	var num, den float64
	for _, c := range confs {
		num += c.c * c.w
		den += c.w
	}
	if den == 0 {
		return 0
	}
	return clamp01(num / den)
}

func orDefault(v, fallback float64) float64 {
	if v > 0 {
		return v
	}
	return fallback
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
