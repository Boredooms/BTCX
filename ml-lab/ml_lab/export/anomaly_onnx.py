"""Export the anomaly pipeline to ONNX and verify Python<->ONNX parity.

Run: python -m ml_lab.export.anomaly_onnx

skl2onnx exports the scaler + IsolationForest. The IsolationForest ONNX graph
exposes the anomaly 'scores' output (sklearn score_samples). The Go runtime
reads that output and applies the SAME deterministic calibration recorded in
calibration.json to produce anomaly_score in [0,1]. We verify parity on the
raw ONNX score vs Python score_samples using the golden fixtures and a sample.
"""

from __future__ import annotations

import numpy as np
import onnxruntime as ort
from skl2onnx import to_onnx
from skl2onnx.common.data_types import FloatTensorType

from ml_lab import data, paths, schema, util
import joblib

TOLERANCE = 1e-4


def main() -> None:
    out = paths.MODELS / "anomaly"
    pipe = joblib.load(out / "model.joblib")

    n_features = len(schema.FEATURES)
    initial_types = [("input", FloatTensorType([None, n_features]))]
    # ai.onnx.ml pinned to 3 for onnxruntime 1.18 compatibility.
    onx = to_onnx(pipe, initial_types=initial_types,
                  target_opset={"": 15, "ai.onnx.ml": 3})
    onnx_path = out / "model.onnx"
    with open(onnx_path, "wb") as f:
        f.write(onx.SerializeToString())

    # Parity check on golden fixtures + a dataset sample.
    golden = util.read_json(paths.FEATURE_GOLDEN)
    Xg = np.array([fx["vector"] for fx in golden["fixtures"]], dtype=np.float32)

    df = data.load_features(paths.ANOMALY_CSV, extra_cols=["split"])
    sample = df[df["split"] == "test"][list(schema.FEATURES)].to_numpy(np.float32)[:2000]
    X = np.vstack([Xg, sample])

    sess = ort.InferenceSession(str(onnx_path), providers=["CPUExecutionProvider"])
    outputs = sess.run(None, {"input": X})
    out_names = [o.name for o in sess.get_outputs()]
    # ONNX 'scores' output == sklearn decision_function. The Go runtime negates
    # it and applies calibration. Verify parity against decision_function.
    py_decision = pipe.named_steps["iforest"].decision_function(
        pipe.named_steps["scaler"].transform(X.astype(np.float64))
    )
    best = None
    best_diff = np.inf
    for name, arr in zip(out_names, outputs):
        a = np.asarray(arr).reshape(len(X), -1)
        if a.shape[1] != 1:
            continue
        diff = float(np.max(np.abs(a.ravel() - py_decision)))
        if diff < best_diff:
            best_diff = diff
            best = name

    parity = {
        "model": "anomaly-detector",
        "onnx_opset": 15,
        "onnx_output_used": best,
        "onnx_output_names": out_names,
        "tolerance": TOLERANCE,
        "max_abs_diff": best_diff,
        "pass": bool(best_diff <= TOLERANCE),
        "n_compared": int(len(X)),
        "note": "ONNX 'scores' == sklearn decision_function. Go computes "
                "anomaly_score = clamp((-scores - raw_lo)/(raw_hi-raw_lo),0,1) "
                "using calibration.json.",
        "env": util.env_block(),
    }
    paths.EXPORT.mkdir(parents=True, exist_ok=True)
    util.write_json(paths.EXPORT / "anomaly_parity.json", parity)

    # Finalize manifest with checksums (only now that the file is final).
    man = util.read_json(out / "training_manifest.json")
    manifest = {
        **{k: man[k] for k in ("model_name", "feature_schema", "feature_schema_sha256",
                               "feature_order", "training_dataset", "training_dataset_sha256",
                               "calibration", "output_schema", "seed")},
        "model_version": "1.0.0",
        "algorithm": "isolation-forest",
        "model_format": "onnx",
        "runtime": "onnxruntime-cpu",
        "model_sha256": util.sha256_file(onnx_path),
        "onnx_opset": 15,
        "parity": {"max_abs_diff": best_diff, "tolerance": TOLERANCE, "pass": parity["pass"]},
        "status": "ACTIVE" if parity["pass"] else "PARITY_FAILED",
        "env": util.env_block(),
    }
    util.write_json(out / "manifest.json", manifest)

    print(f"exported {onnx_path} ({onnx_path.stat().st_size:,} bytes)")
    print(f"parity max_abs_diff={best_diff:.3e} tol={TOLERANCE} pass={parity['pass']}")
    if not parity["pass"]:
        raise SystemExit("ONNX parity FAILED")


if __name__ == "__main__":
    main()
