"""Export authoritative golden INFERENCE vectors for Go parity.

Run: python -m ml_lab.export.golden_inference

Produces evaluation/inference_golden.json containing, for a fixed set of real
input rows:
  - anomaly: the 25-feature input, the ONNX raw 'scores' output, and the
    calibrated anomaly_score in [0,1]
  - flow: the 17 flow-feature input and the ONNX per-class probability vector
    (in the model's class order)

The Go inference layer must reproduce these within tolerance. This is the
cross-language contract: Python+ONNX here == Go there.
"""

from __future__ import annotations

import joblib
import numpy as np
import onnxruntime as ort
import pandas as pd

from ml_lab import data, paths, schema, util
from ml_lab.training.flow.train import FLOW_FEATURES

N_ANOMALY = 40
N_FLOW = 40


def _anomaly_golden():
    calib = util.read_json(paths.MODELS / "anomaly" / "calibration.json")
    df = data.load_features(paths.ANOMALY_CSV, extra_cols=["split", "subject_id"])
    test = df[df["split"] == "test"].reset_index(drop=True).iloc[:N_ANOMALY]
    X = test[list(schema.FEATURES)].to_numpy(np.float32)

    sess = ort.InferenceSession(str(paths.MODELS / "anomaly" / "model.onnx"),
                                providers=["CPUExecutionProvider"])
    outs = sess.run(None, {"input": X})
    scores = None
    for o in outs:
        a = np.asarray(o).reshape(len(X), -1)
        if a.shape[1] == 1:
            scores = a.ravel()
    lo, hi = calib["raw_lo"], calib["raw_hi"]
    calibrated = np.clip((-scores - lo) / (hi - lo), 0.0, 1.0)

    rows = []
    for i in range(len(X)):
        rows.append({
            "subject_id": str(test.iloc[i]["subject_id"]),
            "input": [float(v) for v in X[i]],
            "onnx_score": float(scores[i]),
            "anomaly_score": float(calibrated[i]),
        })
    return {
        "feature_order": list(schema.FEATURES),
        "calibration": calib,
        "onnx_output_semantics": "ONNX 'scores' == sklearn decision_function; "
                                 "raw = -scores; anomaly_score = clamp((raw-raw_lo)/(raw_hi-raw_lo),0,1)",
        "rows": rows,
    }


def _flow_golden():
    pipe = joblib.load(paths.MODELS / "flow" / "model.joblib")
    classes = list(pipe.classes_)
    df = pd.read_csv(paths.FLOW_CSV, usecols=FLOW_FEATURES + ["split", "flow_id",
                                                              "pattern_label"],
                     low_memory=False)
    test = df[df["split"] == "test"].reset_index(drop=True).iloc[:N_FLOW]
    X = test[FLOW_FEATURES].to_numpy(np.float32)

    sess = ort.InferenceSession(str(paths.MODELS / "flow" / "model.onnx"),
                                providers=["CPUExecutionProvider"])
    outs = sess.run(None, {"input": X})
    proba = None
    for o in outs:
        a = np.asarray(o)
        if a.ndim == 2 and a.shape[1] == len(classes):
            proba = a
    rows = []
    for i in range(len(X)):
        rows.append({
            "flow_id": str(test.iloc[i]["flow_id"]),
            "input": [float(v) for v in X[i]],
            "proba": [float(p) for p in proba[i]],
            "argmax_class": classes[int(proba[i].argmax())],
            "true_label": str(test.iloc[i]["pattern_label"]),
        })
    return {
        "feature_order": FLOW_FEATURES,
        "classes": classes,
        "rows": rows,
    }


def main() -> None:
    paths.ensure_dirs()
    golden = {
        "schema_sha256": schema.schema_hash(),
        "anomaly": _anomaly_golden(),
        "flow": _flow_golden(),
        "env": util.env_block(),
    }
    out = paths.EVALUATION / "inference_golden.json"
    util.write_json(out, golden)
    print(f"wrote {out}")
    print(f"anomaly rows: {len(golden['anomaly']['rows'])}, "
          f"flow rows: {len(golden['flow']['rows'])}")
    print(f"flow classes: {golden['flow']['classes']}")


if __name__ == "__main__":
    main()
