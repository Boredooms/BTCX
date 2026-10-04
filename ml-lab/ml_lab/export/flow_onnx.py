"""Export the flow classifier to ONNX and verify Python<->ONNX parity.

Run: python -m ml_lab.export.flow_onnx

Compares ONNX probability output against sklearn predict_proba on a test
sample within a recorded tolerance.
"""

from __future__ import annotations

import joblib
import numpy as np
import onnxruntime as ort
import pandas as pd
from skl2onnx import to_onnx
from skl2onnx.common.data_types import FloatTensorType

from ml_lab import paths, util
from ml_lab.training.flow.train import FLOW_FEATURES

TOLERANCE = 1e-4


def main() -> None:
    out = paths.MODELS / "flow"
    pipe = joblib.load(out / "model.joblib")
    classes = list(pipe.classes_)
    n = len(FLOW_FEATURES)

    onx = to_onnx(
        pipe,
        initial_types=[("input", FloatTensorType([None, n]))],
        target_opset={"": 15, "ai.onnx.ml": 3},
        options={"zipmap": False},  # emit a plain probability tensor
    )
    onnx_path = out / "model.onnx"
    with open(onnx_path, "wb") as f:
        f.write(onx.SerializeToString())

    # Parity on a test sample.
    df = pd.read_csv(paths.FLOW_CSV, usecols=FLOW_FEATURES + ["split"],
                     low_memory=False)
    X = df[df["split"] == "test"][FLOW_FEATURES].to_numpy(np.float32)[:3000]
    py_proba = pipe.predict_proba(X.astype(np.float64))

    sess = ort.InferenceSession(str(onnx_path), providers=["CPUExecutionProvider"])
    outs = sess.run(None, {"input": X})
    names = [o.name for o in sess.get_outputs()]
    # Find the probability output (shape [N, n_classes]).
    onnx_proba = None
    for name, arr in zip(names, outs):
        a = np.asarray(arr)
        if a.ndim == 2 and a.shape[1] == len(classes):
            onnx_proba = a
            break
    if onnx_proba is None:
        raise SystemExit(f"no probability output found; outputs={names}")

    max_diff = float(np.max(np.abs(onnx_proba - py_proba)))
    # Also check argmax (predicted class) agreement.
    argmax_agree = float((onnx_proba.argmax(1) == py_proba.argmax(1)).mean())

    parity = {
        "model": "flow-pattern",
        "onnx_opset": 15,
        "onnx_output_names": names,
        "tolerance": TOLERANCE,
        "max_abs_diff": max_diff,
        "argmax_agreement": argmax_agree,
        "pass": bool(max_diff <= TOLERANCE),
        "n_compared": int(len(X)),
        "classes": classes,
        "env": util.env_block(),
    }
    paths.EXPORT.mkdir(parents=True, exist_ok=True)
    util.write_json(paths.EXPORT / "flow_parity.json", parity)

    man = util.read_json(out / "training_manifest.json")
    manifest = {
        **{k: man[k] for k in ("model_name", "model_version", "algorithm",
                               "feature_set", "n_features", "classes", "target",
                               "training_dataset", "training_dataset_sha256",
                               "hyperparameters", "preprocessing",
                               "structural_evidence", "output_schema", "seed")},
        "model_format": "onnx",
        "runtime": "onnxruntime-cpu",
        "feature_schema": "flow-features-v1",
        "model_sha256": util.sha256_file(onnx_path),
        "onnx_opset": 15,
        "parity": {"max_abs_diff": max_diff, "argmax_agreement": argmax_agree,
                   "tolerance": TOLERANCE, "pass": parity["pass"]},
        "status": "ACTIVE" if parity["pass"] else "PARITY_FAILED",
        "env": util.env_block(),
    }
    util.write_json(out / "manifest.json", manifest)

    print(f"exported {onnx_path} ({onnx_path.stat().st_size:,} bytes)")
    print(f"parity max_abs_diff={max_diff:.3e} argmax_agree={argmax_agree:.4f} "
          f"pass={parity['pass']}")
    if not parity["pass"]:
        raise SystemExit("ONNX parity FAILED")


if __name__ == "__main__":
    main()
