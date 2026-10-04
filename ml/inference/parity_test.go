package inference

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// goldenInference mirrors ml-lab/evaluation/inference_golden.json.
type goldenInference struct {
	Anomaly struct {
		Rows []struct {
			Input        []float64 `json:"input"`
			ONNXScore    float64   `json:"onnx_score"`
			AnomalyScore float64   `json:"anomaly_score"`
		} `json:"rows"`
	} `json:"anomaly"`
	Flow struct {
		Classes []string `json:"classes"`
		Rows    []struct {
			Input       []float64 `json:"input"`
			Proba       []float64 `json:"proba"`
			ArgmaxClass string    `json:"argmax_class"`
		} `json:"rows"`
	} `json:"flow"`
}

func repoFile(parts ...string) string {
	all := append([]string{"..", ".."}, parts...)
	return filepath.Join(all...)
}

func loadGoldenInference(t *testing.T) *goldenInference {
	t.Helper()
	path := repoFile("ml-lab", "evaluation", "inference_golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("golden inference not present: %v", err)
	}
	var g goldenInference
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return &g
}

const parityTol = 1e-5

// TestAnomalyParity verifies the Go anomaly evaluator reproduces the ONNX raw
// score and the calibrated anomaly_score for every golden row.
func TestAnomalyParity(t *testing.T) {
	g := loadGoldenInference(t)
	lm, err := LoadTreeModelDir(t, "anomaly")
	if err != nil {
		t.Fatalf("load anomaly: %v", err)
	}
	maxRaw, maxCal := 0.0, 0.0
	for i, row := range g.Anomaly.Rows {
		res, err := lm.PredictAnomaly(row.Input)
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		dRaw := math.Abs(res.RawScore - row.ONNXScore)
		dCal := math.Abs(res.AnomalyScore - row.AnomalyScore)
		if dRaw > maxRaw {
			maxRaw = dRaw
		}
		if dCal > maxCal {
			maxCal = dCal
		}
	}
	t.Logf("anomaly parity: max raw diff=%.3e, max calibrated diff=%.3e (n=%d)",
		maxRaw, maxCal, len(g.Anomaly.Rows))
	if maxRaw > parityTol || maxCal > parityTol {
		t.Fatalf("anomaly parity exceeded tol %g: raw=%.3e cal=%.3e",
			parityTol, maxRaw, maxCal)
	}
}

// TestFlowParity verifies the Go flow evaluator reproduces ONNX class
// probabilities and 100%% argmax agreement.
func TestFlowParity(t *testing.T) {
	g := loadGoldenInference(t)
	lm, err := LoadTreeModelDir(t, "flow")
	if err != nil {
		t.Fatalf("load flow: %v", err)
	}
	maxDiff := 0.0
	argmaxAgree := 0
	for i, row := range g.Flow.Rows {
		res, err := lm.PredictFlow(row.Input)
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		for j, c := range g.Flow.Classes {
			d := math.Abs(res.Probabilities[c] - row.Proba[j])
			if d > maxDiff {
				maxDiff = d
			}
		}
		if res.TopClass == row.ArgmaxClass {
			argmaxAgree++
		}
	}
	agree := float64(argmaxAgree) / float64(len(g.Flow.Rows))
	t.Logf("flow parity: max prob diff=%.3e, argmax agreement=%.4f (n=%d)",
		maxDiff, agree, len(g.Flow.Rows))
	if maxDiff > parityTol {
		t.Fatalf("flow prob parity exceeded tol %g: %.3e", parityTol, maxDiff)
	}
	if agree != 1.0 {
		t.Fatalf("flow argmax agreement = %.4f, want 1.0", agree)
	}
}

// LoadTreeModelDir loads a model from the repo models/ dir for tests.
func LoadTreeModelDir(t *testing.T, name string) (*LoadedModel, error) {
	t.Helper()
	reg := NewRegistry(repoFile("models"), "")
	return reg.Get(name)
}
