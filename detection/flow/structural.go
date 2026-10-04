// Package flow implements deterministic structural flow detectors plus the
// combination of structural evidence with the flow ML score.
//
// Structural detectors are NOT the ML model: they are explicit, auditable rules
// over flow features that mirror ml-lab/ml_lab/training/flow/structural.py. The
// production flow result keeps the ML score and the structural evidence
// separate, then combines them, so an analyst can see both.
//
// Patterns are described as "-like" matches, never as proof of criminal
// activity.
package flow

import "math"

// Features is the structural flow feature set (the 17 inputs the flow model and
// the detectors read). Field order mirrors the model manifest's feature_set.
type Features struct {
	InputCount       float64
	OutputCount      float64
	ParticipantCount float64
	HopCount         float64
	ChainLength      float64
	ValueDecay       float64
	SplitRatio       float64
	MergeRatio       float64
	FanIn            float64
	FanOut           float64
	MedianTimeGap    float64
	Burstiness       float64
	Velocity         float64
	TotalVolumeBTC   float64
	AmountEntropy    float64
	VsizeVB          float64
	FeerateSatVB     float64
}

// ModelFeatureOrder is the exact input order the flow ONNX model expects.
var ModelFeatureOrder = []string{
	"input_count", "output_count", "participant_count", "hop_count",
	"chain_length", "value_decay", "split_ratio", "merge_ratio", "fan_in",
	"fan_out", "median_time_gap", "burstiness", "velocity", "total_volume_btc",
	"amount_entropy", "vsize_vb", "feerate_sat_vb",
}

// Vector returns the features in ModelFeatureOrder for ONNX inference.
func (f Features) Vector() []float64 {
	return []float64{
		f.InputCount, f.OutputCount, f.ParticipantCount, f.HopCount,
		f.ChainLength, f.ValueDecay, f.SplitRatio, f.MergeRatio, f.FanIn,
		f.FanOut, f.MedianTimeGap, f.Burstiness, f.Velocity, f.TotalVolumeBTC,
		f.AmountEntropy, f.VsizeVB, f.FeerateSatVB,
	}
}

// StructuralScore is one deterministic detector's output.
type StructuralScore struct {
	Pattern string  // e.g. "peeling_chain_like"
	Score   float64 // 0..1
	Reason  string
}

// PeelingChain: long chain, high retained value, small splits.
//
// Precondition: an ACTUAL chain must exist (HopCount >= 4). Without a chain the
// retained-value / small-split conditions are not evidence of peeling (they are
// trivially true for a one-off receive or a single spend), so the detector must
// not fire — this is what keeps a benign low-activity wallet out of the
// peeling-chain band. The "-like" score only accrues once the chain gate holds.
func PeelingChain(f Features) StructuralScore {
	if f.HopCount < 4 {
		return StructuralScore{"peeling_chain_like", 0,
			"no multi-hop chain — not peeling-like"}
	}
	s := 0.4 // the chain gate itself
	if f.ValueDecay >= 0.7 {
		s += 0.3
	}
	if f.SplitRatio <= 0.4 {
		s += 0.3
	}
	return StructuralScore{"peeling_chain_like", math.Min(s, 1),
		"long chain with retained value and small peels"}
}

// MixingLike: many inputs AND outputs, high entropy, near-balanced value.
func MixingLike(f Features) StructuralScore {
	s := 0.0
	if f.InputCount >= 6 && f.OutputCount >= 6 {
		s += 0.5
	}
	if f.AmountEntropy >= 0.7 {
		s += 0.3
	}
	if f.ValueDecay >= 0.9 {
		s += 0.2
	}
	return StructuralScore{"mixing_like", math.Min(s, 1),
		"many-in many-out with high entropy"}
}

// FanOut / FanIn.
func FanOut(f Features) StructuralScore {
	s := 0.0
	if f.FanOut >= 10 {
		s += 1.0
	}
	if f.SplitRatio >= 0.7 {
		s += 0.4
	}
	return StructuralScore{"high_fan_out", math.Min(s, 1), "high output fan-out"}
}

func FanIn(f Features) StructuralScore {
	s := 0.0
	if f.FanIn >= 10 {
		s += 1.0
	}
	if f.MergeRatio >= 0.7 {
		s += 0.4
	}
	return StructuralScore{"high_fan_in", math.Min(s, 1), "high input fan-in"}
}

// RapidFlow: short inter-tx gap and bursty.
//
// Precondition: "rapid/bursty" requires MULTIPLE transfers. With a single tx
// (or none) the median time gap is trivially 0 — that is not a burst, so the
// detector must not fire. Burst evidence only accrues once there is real
// repeated activity (participant/hop count indicates more than one transfer).
func RapidFlow(f Features) StructuralScore {
	if f.HopCount < 2 && f.ParticipantCount < 3 {
		return StructuralScore{"rapid_flow", 0,
			"single transfer — not a burst"}
	}
	s := 0.0
	if f.MedianTimeGap <= 10 {
		s += 0.6
	}
	if f.Burstiness >= 0.2 {
		s += 0.4
	}
	return StructuralScore{"rapid_flow", math.Min(s, 1), "rapid, bursty transfers"}
}

// Detect runs all structural detectors.
func Detect(f Features) []StructuralScore {
	return []StructuralScore{
		PeelingChain(f), MixingLike(f), FanOut(f), FanIn(f), RapidFlow(f),
	}
}
