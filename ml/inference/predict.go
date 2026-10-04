package inference

import (
	"fmt"
	"math"
)

// AnomalyResult is the typed anomaly prediction.
type AnomalyResult struct {
	RawScore     float64 // ONNX "scores" output (== sklearn decision_function)
	AnomalyScore float64 // calibrated, clamped to [0,1]
	ModelVersion string
}

// FlowResult is the typed flow prediction.
type FlowResult struct {
	Probabilities map[string]float64 // class -> probability
	TopClass      string
	TopScore      float64
	ModelVersion  string
	Classes       []string // class order as in the model
}

// PredictAnomaly runs the anomaly regressor and applies the frozen calibration.
//
// Contract (from the ML lab): ONNX "scores" == sklearn decision_function; the
// raw anomaly magnitude is -scores; the calibrated score is
// clamp((raw - raw_lo)/(raw_hi - raw_lo), 0, 1).
func (lm *LoadedModel) PredictAnomaly(features []float64) (AnomalyResult, error) {
	if lm.Trees.Ens.Kind != "regressor" {
		return AnomalyResult{}, fmt.Errorf("anomaly model is not a regressor")
	}
	if lm.Calibration == nil {
		return AnomalyResult{}, fmt.Errorf("anomaly model missing calibration")
	}
	if lm.Trees.IForestBlock == nil {
		return AnomalyResult{}, fmt.Errorf("anomaly model missing iforest params")
	}
	// decision_function == ONNX "scores"; raw anomaly magnitude = -scores.
	score := lm.Trees.IForestDecision(features)
	raw := -score
	lo, hi := lm.Calibration.RawLo, lm.Calibration.RawHi
	den := hi - lo
	if den == 0 {
		den = 1e-9
	}
	cal := (raw - lo) / den
	cal = math.Max(0, math.Min(1, cal))
	return AnomalyResult{
		RawScore:     score,
		AnomalyScore: cal,
		ModelVersion: lm.Manifest.ModelVersion,
	}, nil
}

// PredictFlow runs the flow classifier and returns per-class probabilities in
// the model's class order (not alphabetical).
func (lm *LoadedModel) PredictFlow(features []float64) (FlowResult, error) {
	if lm.Trees.Ens.Kind != "classifier" {
		return FlowResult{}, fmt.Errorf("flow model is not a classifier")
	}
	probs := lm.Trees.Probabilities(features)
	classes := lm.Trees.Classes()
	if len(probs) != len(classes) {
		return FlowResult{}, fmt.Errorf("prob/class length mismatch: %d vs %d",
			len(probs), len(classes))
	}
	m := make(map[string]float64, len(classes))
	topIdx := 0
	for i, c := range classes {
		m[c] = probs[i]
		if probs[i] > probs[topIdx] {
			topIdx = i
		}
	}
	return FlowResult{
		Probabilities: m,
		TopClass:      classes[topIdx],
		TopScore:      probs[topIdx],
		ModelVersion:  lm.Manifest.ModelVersion,
		Classes:       classes,
	}, nil
}
