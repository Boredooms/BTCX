"""Verify the exact IsolationForest decision_function reconstruction from
per-tree leaf path lengths, so the Go evaluator can reproduce it precisely.

sklearn IsolationForest:
  score_samples(X) = -2 ** ( -mean_path / c(n_samples) )
  decision_function = score_samples - offset_      (offset_ = -0.5 by default
     for contamination='auto' it is set from the training scores)
We reconstruct mean_path from each tree and compare to the model's own
_compute_chunked_score_samples path, then confirm decision_function matches the
ONNX output recorded in inference_golden.json.
"""
import json
import numpy as np
import joblib
from pathlib import Path

LAB = Path("/home/devara/btcx/ml-lab")
pipe = joblib.load(LAB / "models/anomaly/model.joblib")
scaler = pipe.named_steps["scaler"]
iforest = pipe.named_steps["iforest"]

golden = json.load(open(LAB / "evaluation/inference_golden.json"))
rows = golden["anomaly"]["rows"]
X_raw = np.array([r["input"] for r in rows], dtype=np.float64)
onnx_scores = np.array([r["onnx_score"] for r in rows])

Xs = scaler.transform(X_raw)

# sklearn's own decision_function (ground truth).
dec = iforest.decision_function(Xs)
print("max |sklearn decision_function - onnx_score| =",
      float(np.max(np.abs(dec - onnx_scores))))

# Manual reconstruction of average path length.
from sklearn.ensemble._iforest import _average_path_length

n_samples = Xs.shape[0]
depths = np.zeros(n_samples)
for tree, features in zip(iforest.estimators_, iforest.estimators_features_):
    leaves_index = tree.apply(Xs[:, features])
    node_indicator = tree.decision_path(Xs[:, features])
    n_samples_leaf = tree.tree_.n_node_samples[leaves_index]
    depths += (
        np.ravel(node_indicator.sum(axis=1))
        + _average_path_length(n_samples_leaf)
        - 1.0
    )
denominator = len(iforest.estimators_) * _average_path_length(np.array([iforest.max_samples_]))
scores = 2 ** (-depths / denominator)
# sklearn: score_samples = -scores ; decision_function = score_samples - offset_
recon_dec = -scores - iforest.offset_
print("max |recon decision_function - onnx_score| =",
      float(np.max(np.abs(recon_dec - onnx_scores))))
print("offset_ =", float(iforest.offset_))
print("max_samples_ =", int(iforest.max_samples_))
print("denominator c(n) =", float(denominator))
print("n_estimators =", len(iforest.estimators_))
print("euler via _average_path_length(2) =", float(_average_path_length(np.array([2]))))
