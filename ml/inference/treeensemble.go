// Package inference executes BCTX's trained models locally with no native
// dependencies and no network access.
//
// The models are tree ensembles (sklearn IsolationForest exported as a
// TreeEnsembleRegressor; GradientBoosting exported as a TreeEnsembleClassifier).
// Rather than depend on the native onnxruntime shared library, the ML lab
// extracts the exact tree parameters from the real model.onnx (via the onnx
// parser, authoritative) into model.trees.json. This package reproduces the
// ONNX ai.onnx.ml TreeEnsemble semantics in pure Go and is verified against the
// Python/ONNX golden vectors within a tight tolerance.
package inference

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Scaler mirrors the ai.onnx.ml Scaler node: output = (input - offset) * scale.
type Scaler struct {
	Offset []float64 `json:"offset"`
	Scale  []float64 `json:"scale"`
}

// Transform applies the scaler to a feature vector in place-safe fashion.
func (s *Scaler) Transform(x []float64) []float64 {
	if s == nil {
		return x
	}
	out := make([]float64, len(x))
	for i := range x {
		out[i] = (x[i] - s.Offset[i]) * s.Scale[i]
	}
	return out
}

// Ensemble holds the flattened node/leaf tables as emitted by ONNX. Node arrays
// are indexed together; a node is identified by (treeid, nodeid).
type Ensemble struct {
	Kind              string    `json:"kind"` // "regressor" | "classifier"
	Classlabels       []string  `json:"classlabels"`
	NTargets          int       `json:"n_targets"`
	AggregateFunction string    `json:"aggregate_function"`
	PostTransform     string    `json:"post_transform"`
	BaseValues        []float64 `json:"base_values"`

	NodesTreeids    []int64   `json:"nodes_treeids"`
	NodesNodeids    []int64   `json:"nodes_nodeids"`
	NodesFeatureids []int64   `json:"nodes_featureids"`
	NodesValues     []float64 `json:"nodes_values"`
	NodesModes      []string  `json:"nodes_modes"`
	NodesTrue       []int64   `json:"nodes_truenodeids"`
	NodesFalse      []int64   `json:"nodes_falsenodeids"`
	NodesMissTrue   []int64   `json:"nodes_missing_value_tracks_true"`

	// Regressor leaf tables.
	TargetTreeids []int64   `json:"target_treeids"`
	TargetNodeids []int64   `json:"target_nodeids"`
	TargetIDs     []int64   `json:"target_ids"`
	TargetWeights []float64 `json:"target_weights"`

	// Classifier leaf tables.
	ClassTreeids []int64   `json:"class_treeids"`
	ClassNodeids []int64   `json:"class_nodeids"`
	ClassIDs     []int64   `json:"class_ids"`
	ClassWeights []float64 `json:"class_weights"`
}

// IForest holds IsolationForest decision-function parameters (present only for
// the anomaly model). See the lab's onnx_to_treejson for the exact formula.
type IForest struct {
	NTrees      int     `json:"n_trees"`
	Euler       float64 `json:"euler"`
	Denominator float64 `json:"denominator"`
	Offset      float64 `json:"offset"`
	MaxSamples  int     `json:"max_samples"`
}

// TreeModel is the deserialized model.trees.json plus a compiled node index.
type TreeModel struct {
	SourceONNX       string   `json:"source_onnx"`
	SourceONNXSHA256 string   `json:"source_onnx_sha256"`
	ScalerNode       *Scaler  `json:"scaler"`
	Ens              Ensemble `json:"ensemble"`
	IForestBlock     *IForest `json:"iforest"`

	// compiled: per-tree node lookup and leaf-weight lookup.
	nodeIndex map[int64]map[int64]int // treeid -> nodeid -> array index
	// leaf weights: treeid -> nodeid -> []classWeight (len = numOutputs)
	leaf      map[int64]map[int64][]float64
	numOutput int
}

// LoadTreeModel reads and compiles a model.trees.json file.
func LoadTreeModel(path string) (*TreeModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read tree model: %w", err)
	}
	var m TreeModel
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse tree model: %w", err)
	}
	if err := m.compile(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *TreeModel) compile() error {
	e := &m.Ens
	// Node index.
	m.nodeIndex = make(map[int64]map[int64]int)
	for i := range e.NodesNodeids {
		t := e.NodesTreeids[i]
		n := e.NodesNodeids[i]
		if m.nodeIndex[t] == nil {
			m.nodeIndex[t] = make(map[int64]int)
		}
		m.nodeIndex[t][n] = i
	}

	// Number of outputs.
	switch e.Kind {
	case "classifier":
		m.numOutput = len(e.Classlabels)
	case "regressor":
		if e.NTargets > 0 {
			m.numOutput = e.NTargets
		} else {
			m.numOutput = 1
		}
	default:
		return fmt.Errorf("unknown ensemble kind %q", e.Kind)
	}

	// Leaf weight table.
	m.leaf = make(map[int64]map[int64][]float64)
	addLeaf := func(tree, node, id int64, w float64) {
		if m.leaf[tree] == nil {
			m.leaf[tree] = make(map[int64][]float64)
		}
		if m.leaf[tree][node] == nil {
			m.leaf[tree][node] = make([]float64, m.numOutput)
		}
		m.leaf[tree][node][id] += w
	}
	if e.Kind == "classifier" {
		for i := range e.ClassNodeids {
			addLeaf(e.ClassTreeids[i], e.ClassNodeids[i], e.ClassIDs[i], e.ClassWeights[i])
		}
	} else {
		for i := range e.TargetNodeids {
			addLeaf(e.TargetTreeids[i], e.TargetNodeids[i], e.TargetIDs[i], e.TargetWeights[i])
		}
	}
	return nil
}

// NumOutputs returns the number of raw outputs (1 for the regressor, n_classes
// for the classifier).
func (m *TreeModel) NumOutputs() int { return m.numOutput }

// Classes returns the classifier class labels (nil for a regressor).
func (m *TreeModel) Classes() []string { return m.Ens.Classlabels }

// walkTree follows BRANCH_LEQ decisions from the root (nodeid 0) to a leaf and
// returns the leaf's node id.
func (m *TreeModel) walkTree(tree int64, x []float64) int64 {
	e := &m.Ens
	node := int64(0)
	for {
		idx := m.nodeIndex[tree][node]
		mode := e.NodesModes[idx]
		if mode == "LEAF" {
			return node
		}
		// BRANCH_LEQ: feature <= threshold -> true branch, else false branch.
		feat := e.NodesFeatureids[idx]
		thr := e.NodesValues[idx]
		fv := x[feat]
		goTrue := fv <= thr
		if math.IsNaN(fv) && len(e.NodesMissTrue) > idx {
			goTrue = e.NodesMissTrue[idx] == 1
		}
		if goTrue {
			node = e.NodesTrue[idx]
		} else {
			node = e.NodesFalse[idx]
		}
	}
}

// RawOutputs runs the ensemble on a (already feature-ordered) raw input vector
// and returns the aggregated raw outputs BEFORE any post-transform, with
// base_values added. For the regressor this is a length-1 slice equal to the
// ONNX "scores" output; for the classifier it is the pre-softmax class sums.
func (m *TreeModel) RawOutputs(rawInput []float64) []float64 {
	x := m.ScalerNode.Transform(rawInput)
	sums := make([]float64, m.numOutput)
	copy(sums, padBase(m.Ens.BaseValues, m.numOutput))

	// Each distinct tree contributes its leaf weights.
	for tree := range m.leaf {
		leafNode := m.walkTree(tree, x)
		w := m.leaf[tree][leafNode]
		for c := 0; c < m.numOutput; c++ {
			sums[c] += w[c]
		}
	}
	return sums
}

// Probabilities runs the classifier and applies the post-transform (SOFTMAX).
func (m *TreeModel) Probabilities(rawInput []float64) []float64 {
	raw := m.RawOutputs(rawInput)
	switch m.Ens.PostTransform {
	case "SOFTMAX":
		return softmax(raw)
	case "NONE", "":
		return raw
	default:
		return softmax(raw)
	}
}

func padBase(b []float64, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n && i < len(b); i++ {
		out[i] = b[i]
	}
	return out
}

func softmax(v []float64) []float64 {
	maxv := math.Inf(-1)
	for _, x := range v {
		if x > maxv {
			maxv = x
		}
	}
	var sum float64
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = math.Exp(x - maxv)
		sum += out[i]
	}
	if sum == 0 {
		return out
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}
