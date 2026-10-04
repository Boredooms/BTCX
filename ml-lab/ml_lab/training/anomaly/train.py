"""Anomaly model trainer — Isolation Forest, unsupervised, normal-only train.

Run: python -m ml_lab.training.anomaly.train [--seed N] [--estimators N]

Pipeline:
  normal train rows -> StandardScaler (fit on train) -> IsolationForest
  -> raw score_samples -> deterministic [0,1] calibration (train quantiles)

Artifacts (models/anomaly/):
  model.joblib         frozen sklearn pipeline (scaler + iforest)
  calibration.json     score transform metadata (train-derived, frozen)
  training_manifest.json
"""

from __future__ import annotations

import argparse

import joblib
import numpy as np
from sklearn.ensemble import IsolationForest
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from ml_lab import data, paths, schema, util


def raw_scores(pipe: Pipeline, X: np.ndarray) -> np.ndarray:
    """Higher = more anomalous.

    Defined as -decision_function because that is exactly what the exported
    ONNX graph produces as its 'scores' output (verified: ONNX scores ==
    decision_function to ~2e-7). Using decision_function here keeps the Python
    calibration and the Go/ONNX runtime perfectly consistent: the Go runtime
    computes anomaly_score = clamp((-onnx_scores - raw_lo)/(raw_hi-raw_lo),0,1).
    """
    scaler: StandardScaler = pipe.named_steps["scaler"]
    iforest: IsolationForest = pipe.named_steps["iforest"]
    Xs = scaler.transform(X)
    return -iforest.decision_function(Xs)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, default=data.SEED)
    ap.add_argument("--estimators", type=int, default=200)
    ap.add_argument("--contamination", default="auto")
    args = ap.parse_args()

    paths.ensure_dirs()
    out = paths.MODELS / "anomaly"
    out.mkdir(parents=True, exist_ok=True)

    print("loading anomaly dataset (features + split + label) ...")
    df = data.load_features(paths.ANOMALY_CSV,
                            extra_cols=["split", "ground_truth_anomaly"])

    # Fit ONLY on normal training rows.
    train_mask = (df["split"] == "train") & (df["ground_truth_anomaly"] == 0)
    Xtrain = df.loc[train_mask, list(schema.FEATURES)].to_numpy(dtype=np.float64)
    schema.assert_no_leakage(list(schema.FEATURES))
    print(f"normal train rows: {Xtrain.shape[0]:,}  features: {Xtrain.shape[1]}")

    pipe = Pipeline([
        ("scaler", StandardScaler()),
        ("iforest", IsolationForest(
            n_estimators=args.estimators,
            contamination=args.contamination,
            max_samples="auto",
            random_state=args.seed,
            n_jobs=-1,
        )),
    ])
    pipe.fit(Xtrain)

    # Deterministic calibration: map raw score to [0,1] using the train-score
    # 1st and 99th percentiles (robust min/max), then clamp. Computed on TRAIN
    # ONLY and frozen — never recomputed on validation/test.
    rs_train = raw_scores(pipe, Xtrain)
    lo = float(np.percentile(rs_train, 1))
    hi = float(np.percentile(rs_train, 99))
    if hi <= lo:
        hi = lo + 1e-9
    calibration = {
        "method": "linear_minmax_p1_p99_clamped",
        "raw_lo": lo,
        "raw_hi": hi,
        "formula": "clamp((raw - raw_lo)/(raw_hi - raw_lo), 0, 1)",
        "train_raw_mean": float(rs_train.mean()),
        "train_raw_std": float(rs_train.std()),
    }

    joblib.dump(pipe, out / "model.joblib")
    util.write_json(out / "calibration.json", calibration)

    manifest = {
        "model_name": "anomaly-detector",
        "algorithm": "isolation-forest",
        "feature_schema": schema.FEATURE_SCHEMA_VERSION,
        "feature_schema_sha256": schema.schema_hash(),
        "feature_order": list(schema.FEATURES),
        "training_dataset": paths.ANOMALY_CSV.name,
        "training_dataset_sha256": util.sha256_file(paths.ANOMALY_CSV),
        "normal_train_rows": int(Xtrain.shape[0]),
        "hyperparameters": {
            "n_estimators": args.estimators,
            "contamination": args.contamination,
            "max_samples": "auto",
            "random_state": args.seed,
        },
        "calibration": calibration,
        "output_schema": {"name": "anomaly_score", "range": [0, 1]},
        "preprocessing": "StandardScaler fit on normal-train only (frozen)",
        "seed": args.seed,
        "env": util.env_block(),
    }
    util.write_json(out / "training_manifest.json", manifest)
    data.record_split_manifest("anomaly", paths.ANOMALY_CSV,
                               df["split"].value_counts().to_dict())

    print(f"saved {out/'model.joblib'}")
    print(f"calibration: lo={lo:.6f} hi={hi:.6f}")
    print("anomaly training complete")


if __name__ == "__main__":
    main()
