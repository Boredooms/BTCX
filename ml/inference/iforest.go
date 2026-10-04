package inference

import "math"

// walkTreeDepth follows BRANCH_LEQ decisions and returns both the leaf node id
// and the number of edges traversed from the root (the leaf's depth).
func (m *TreeModel) walkTreeDepth(tree int64, x []float64) (leaf int64, depth int) {
	e := &m.Ens
	node := int64(0)
	for {
		idx := m.nodeIndex[tree][node]
		if e.NodesModes[idx] == "LEAF" {
			return node, depth
		}
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
		depth++
	}
}

// averagePathLength is sklearn's c(k): the average path length of an
// unsuccessful search in a binary search tree. Matches
// sklearn.ensemble._iforest._average_path_length exactly.
func averagePathLength(k float64, euler float64) float64 {
	if k <= 1 {
		return 0
	}
	if k == 2 {
		return 1
	}
	return 2*(math.Log(k-1)+euler) - 2*(k-1)/k
}

// IForestDecision reproduces sklearn IsolationForest.decision_function (which
// equals the ONNX "scores" output) for a single raw input vector.
//
//	depth_t = edges_to_leaf + c(leaf_n_samples) - 1
//	avg     = sum_t(depth_t) / denominator            (denominator = c(n)*n_trees)
//	score   = 2 ** ( -avg )
//	decision_function = -score - offset
func (m *TreeModel) IForestDecision(rawInput []float64) float64 {
	x := m.ScalerNode.Transform(rawInput)
	fp := m.IForestBlock

	var totalDepth float64
	for tree := range m.leaf {
		leafNode := m.walkTree(tree, x)
		// Leaf weight has been remapped by the lab to the exact corrected path
		// length (depth + c(n_samples)); just sum it.
		totalDepth += m.leaf[tree][leafNode][0]
	}
	avg := totalDepth / fp.Denominator
	score := math.Pow(2, -avg)
	return -score - fp.Offset
}
