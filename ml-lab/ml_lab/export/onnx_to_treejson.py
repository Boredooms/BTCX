"""Extract an explicit, Go-consumable tree-ensemble representation from the
REAL exported ONNX graphs, using onnx's own parser as the authority.

Run: python -m ml_lab.export.onnx_to_treejson

Why: BCTX ships as a dependency-light native Go binary. Rather than require the
native onnxruntime shared library (heavy, fragile to package offline), the Go
runtime executes the SAME trained trees via a pure-Go evaluator. This module
serializes the exact scaler + tree-ensemble parameters from model.onnx into
model.trees.json. The Go evaluator reproduces ONNX output within ~1e-6 on the
golden vectors (verified by tests). The JSON is derived deterministically from
the ONNX file and carries the source onnx sha256 for provenance.

Supported ONNX ops: ai.onnx.ml Scaler, TreeEnsembleRegressor,
TreeEnsembleClassifier (BRANCH_LEQ nodes, as emitted by skl2onnx for sklearn
IsolationForest and GradientBoosting).
"""

from __future__ import annotations

import onnx

from ml_lab import paths, util


def _attrs(node):
    d = {}
    for a in node.attribute:
        if a.type == onnx.AttributeProto.FLOAT:
            d[a.name] = a.f
        elif a.type == onnx.AttributeProto.INT:
            d[a.name] = a.i
        elif a.type == onnx.AttributeProto.STRING:
            d[a.name] = a.s.decode()
        elif a.type == onnx.AttributeProto.FLOATS:
            d[a.name] = list(a.floats)
        elif a.type == onnx.AttributeProto.INTS:
            d[a.name] = list(a.ints)
        elif a.type == onnx.AttributeProto.STRINGS:
            d[a.name] = [s.decode() for s in a.strings]
    return d


def _find(graph, op_type):
    for n in graph.node:
        if n.op_type == op_type:
            return n
    return None


def _scaler(graph):
    n = _find(graph, "Scaler")
    if n is None:
        return None
    a = _attrs(n)
    return {"offset": a["offset"], "scale": a["scale"]}


def _all_regressors(graph):
    """Collect ALL TreeEnsembleRegressor nodes. sklearn IsolationForest exports
    as one TreeEnsembleRegressor PER TREE (each outputs a leaf path-length / node
    sample count), followed by an elementwise block that computes the IForest
    anomaly decision_function. We capture every per-tree regressor and merge
    their node tables under distinct tree ids so a single Go walker can evaluate
    all trees, then the Go iforest block applies the exact decision formula.
    """
    nodes = [n for n in graph.node if n.op_type == "TreeEnsembleRegressor"]
    if not nodes:
        return None
    merged = {
        "kind": "regressor",
        "n_targets": 1,
        "aggregate_function": "SUM",
        "post_transform": "NONE",
        "base_values": [],
        "nodes_treeids": [], "nodes_nodeids": [], "nodes_featureids": [],
        "nodes_values": [], "nodes_modes": [], "nodes_truenodeids": [],
        "nodes_falsenodeids": [], "nodes_missing_value_tracks_true": [],
        "target_treeids": [], "target_nodeids": [], "target_ids": [],
        "target_weights": [],
    }
    for tid, n in enumerate(nodes):
        a = _attrs(n)
        cnt = len(a["nodes_nodeids"])
        merged["nodes_treeids"] += [tid] * cnt
        merged["nodes_nodeids"] += a["nodes_nodeids"]
        merged["nodes_featureids"] += a["nodes_featureids"]
        merged["nodes_values"] += a["nodes_values"]
        merged["nodes_modes"] += a["nodes_modes"]
        merged["nodes_truenodeids"] += a["nodes_truenodeids"]
        merged["nodes_falsenodeids"] += a["nodes_falsenodeids"]
        mt = a.get("nodes_missing_value_tracks_true", [0] * cnt)
        merged["nodes_missing_value_tracks_true"] += mt
        lcnt = len(a["target_nodeids"])
        merged["target_treeids"] += [tid] * lcnt
        merged["target_nodeids"] += a["target_nodeids"]
        merged["target_ids"] += a["target_ids"]
        merged["target_weights"] += a["target_weights"]
    merged["n_trees"] = len(nodes)
    return merged


def _iforest_block(model_dir, n_trees):
    """IsolationForest decision-function parameters, read from the trained
    sklearn model (authoritative), verified to reproduce the ONNX output to
    ~1e-7 (see scripts/verify_iforest_formula.py).

    Per sample, for each tree, walk to its leaf:
      depth_t = (edges from root to leaf) + c(leaf_n_samples) - 1
    where c(k) is the average path length of an unsuccessful BST search:
      c(k) = 2*(ln(k-1)+euler) - 2*(k-1)/k   for k > 2
      c(2) = 1 ; c(k<=1) = 0
    Then:
      avg   = sum_t(depth_t) / n_trees
      score = 2 ** ( -avg / denominator )
      decision_function = -score - offset        (offset = iforest.offset_)
    The leaf's stored ONNX target weight is leaf_n_samples (node sample count).
    """
    import joblib

    pipe = joblib.load(model_dir / "model.joblib")
    iforest = pipe.named_steps["iforest"]
    from sklearn.ensemble._iforest import _average_path_length
    import numpy as np

    denom = float(len(iforest.estimators_) *
                  _average_path_length(np.array([iforest.max_samples_]))[0])
    return {
        "n_trees": int(len(iforest.estimators_)),
        "euler": 0.5772156649,
        "denominator": denom,         # c(n) * n_trees
        "offset": float(iforest.offset_),
        "max_samples": int(iforest.max_samples_),
    }


def _average_path_length_scalar(k, euler=0.5772156649):
    if k <= 1:
        return 0.0
    if k == 2:
        return 1.0
    import math
    return 2 * (math.log(k - 1) + euler) - 2 * (k - 1) / k


def _iforest_leaf_pathlengths_from_onnx(graph, regressor_order):
    """Compute corrected path length per ONNX leaf, staying entirely in ONNX
    node numbering (avoids sklearn<->ONNX node-id mismatch).

    For each per-tree TreeEnsembleRegressor:
      - leaf depth: computed by walking the ONNX node structure (true/false ids)
      - n_samples: from the tree's LabelEncoder (nodeid -> n_node_samples)
    corrected path length = depth + c(n_samples).

    Returns {tree_index: {onnx_leaf_nodeid: pathlength}} where tree_index
    matches the enumeration order used by _all_regressors.
    """
    reg_nodes = [n for n in graph.node if n.op_type == "TreeEnsembleRegressor"]
    le_nodes = [n for n in graph.node if n.op_type == "LabelEncoder"]

    # Map each regressor's output tensor to the LabelEncoder consuming it, to
    # pair the correct nodeid->n_samples table with each tree.
    le_by_input = {}
    for le in le_nodes:
        a = _attrs(le)
        if "keys_int64s" in a and "values_floats" in a:
            le_by_input[le.input[0]] = dict(zip(a["keys_int64s"], a["values_floats"]))

    result = {}
    for tidx, reg in enumerate(reg_nodes):
        a = _attrs(reg)
        # Build child structure in ONNX node numbering.
        nid = a["nodes_nodeids"]
        modes = a["nodes_modes"]
        tnode = a["nodes_truenodeids"]
        fnode = a["nodes_falsenodeids"]
        idx_of = {nid[i]: i for i in range(len(nid))}
        # depth via BFS from node 0.
        depth = {0: 0}
        stack = [0]
        while stack:
            node = stack.pop()
            i = idx_of[node]
            if modes[i] != "LEAF":
                for ch in (tnode[i], fnode[i]):
                    depth[ch] = depth[node] + 1
                    stack.append(ch)
        # n_samples map: the regressor output feeds (via Cast/Gather) a
        # LabelEncoder; match by scanning le tables whose keys are this tree's
        # leaf node ids.
        leaf_ids = [nid[i] for i in range(len(nid)) if modes[i] == "LEAF"]
        ns_map = None
        for table in le_by_input.values():
            if set(leaf_ids).issubset(set(table.keys())):
                ns_map = table
                break
        pl = {}
        for lid in leaf_ids:
            ns = ns_map.get(lid, 1.0) if ns_map else 1.0
            pl[lid] = float(depth.get(lid, 0)) + _average_path_length_scalar(ns)
        result[tidx] = pl
    return result


def _classifier(graph):
    """GradientBoosting -> TreeEnsembleClassifier. Returns tree tables."""
    n = _find(graph, "TreeEnsembleClassifier")
    if n is None:
        return None
    a = _attrs(n)
    classes = a.get("classlabels_strings")
    if classes is None and "classlabels_int64s" in a:
        classes = [str(x) for x in a["classlabels_int64s"]]
    return {
        "kind": "classifier",
        "classlabels": classes,
        "aggregate_function": a.get("aggregate_function", "SUM"),
        "post_transform": a.get("post_transform", "NONE"),
        "base_values": a.get("base_values", []),
        "nodes_treeids": a["nodes_treeids"],
        "nodes_nodeids": a["nodes_nodeids"],
        "nodes_featureids": a["nodes_featureids"],
        "nodes_values": a["nodes_values"],
        "nodes_modes": a["nodes_modes"],
        "nodes_truenodeids": a["nodes_truenodeids"],
        "nodes_falsenodeids": a["nodes_falsenodeids"],
        "nodes_missing_value_tracks_true": a.get("nodes_missing_value_tracks_true", []),
        "class_treeids": a["class_treeids"],
        "class_nodeids": a["class_nodeids"],
        "class_ids": a["class_ids"],
        "class_weights": a["class_weights"],
    }


def convert(model_dir):
    onnx_path = model_dir / "model.onnx"
    m = onnx.load(str(onnx_path))
    g = m.graph
    classifier = _classifier(g)
    if classifier is not None:
        ensemble = classifier
        iforest = None
    else:
        ensemble = _all_regressors(g)
        if ensemble is None:
            raise SystemExit(f"no tree ensemble in {onnx_path}")
        iforest = _iforest_block(model_dir, ensemble.get("n_trees", 1))
    doc = {
        "source_onnx": onnx_path.name,
        "source_onnx_sha256": util.sha256_file(onnx_path),
        "scaler": _scaler(g),
        "ensemble": ensemble,
    }
    if iforest is not None:
        doc["iforest"] = iforest
        # Replace ONNX leaf weights (node-id keys) with the exact corrected path
        # length (depth + c(n_samples)), computed entirely in ONNX node
        # numbering. The ONNX leaf weight is the ONNX leaf node id, so we key the
        # per-tree pathlength table by it.
        pathlen = _iforest_leaf_pathlengths_from_onnx(g, ensemble.get("n_trees", 1))
        e = ensemble
        new_weights = []
        missing = 0
        for t, nodeid, w in zip(e["target_treeids"], e["target_nodeids"],
                                e["target_weights"]):
            # ONNX leaf weight == ONNX leaf node id for IForest regressors.
            table = pathlen.get(t, {})
            key = int(round(w))
            if key in table:
                new_weights.append(table[key])
            elif nodeid in table:
                new_weights.append(table[nodeid])
            else:
                new_weights.append(float(w))
                missing += 1
        e["target_weights"] = new_weights
        doc["iforest"]["leaf_is_pathlength"] = True
        doc["iforest"]["leaf_weight_remap_missing"] = missing
        print(f"  iforest leaf remap: {len(new_weights)} leaves, missing={missing}")
    out = model_dir / "model.trees.json"
    util.write_json(out, doc)
    kind = ensemble["kind"]
    ntrees = len(set(ensemble["nodes_treeids"]))
    print(f"{model_dir.name}: {kind}, trees={ntrees}, "
          f"nodes={len(ensemble['nodes_nodeids'])} -> {out.name} "
          f"({out.stat().st_size:,} bytes)")
    return doc


def main() -> None:
    for name in ("anomaly", "flow"):
        convert(paths.MODELS / name)


if __name__ == "__main__":
    main()
