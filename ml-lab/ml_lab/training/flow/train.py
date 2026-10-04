"""Flow pattern classifier trainer (Gate 5).

Run: python -m ml_lab.training.flow.train [--seed N] [--estimators N]

Supervised HistGradientBoosting over flow-structural features. Target =
pattern_label ONLY. suspicious_ground_truth and pattern_label are never inputs.
Structural evidence (deterministic detector flags) is computed SEPARATELY in
ml_lab.training.flow.structural and is not an ML input.

Artifacts (models/flow/):
  model.joblib, scaler is inside pipeline, label_classes.json,
  manifest.json, training_manifest.json
"""

from __future__ import annotations

import argparse

import joblib
import numpy as np
from sklearn.ensemble import GradientBoostingClassifier
from sklearn.pipeline import Pipeline
from sklearn.preprocessing import StandardScaler

from ml_lab import paths, util
import pandas as pd

# Flow-structural ML inputs. Excludes: flow_id, split, subject_id,
# pattern_label, suspicious_ground_truth (leakage), and the raw
# input/output/fee sats conservation triple (would not generalize and is a
# protocol identity, not a behavioral signal).
FLOW_FEATURES = [
    "input_count", "output_count", "participant_count", "hop_count",
    "chain_length", "value_decay", "split_ratio", "merge_ratio",
    "fan_in", "fan_out", "median_time_gap", "burstiness", "velocity",
    "total_volume_btc", "amount_entropy", "vsize_vb", "feerate_sat_vb",
]

LEAKAGE = {"flow_id", "split", "subject_id", "pattern_label",
           "suspicious_ground_truth"}


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, default=20261002)
    ap.add_argument("--estimators", type=int, default=120)
    ap.add_argument("--max-depth", type=int, default=3)
    ap.add_argument("--subsample-train", type=int, default=200000,
                    help="cap train rows for tractable GradientBoosting fit")
    args = ap.parse_args()

    paths.ensure_dirs()
    out = paths.MODELS / "flow"
    out.mkdir(parents=True, exist_ok=True)

    assert not (set(FLOW_FEATURES) & LEAKAGE), "leakage column in FLOW_FEATURES"

    print("loading flow dataset ...")
    cols = FLOW_FEATURES + ["split", "pattern_label"]
    df = pd.read_csv(paths.FLOW_CSV, usecols=cols, low_memory=False)

    train = df[df["split"] == "train"]
    # Stratified subsample keeps GradientBoosting tractable while preserving
    # class balance. Deterministic via seed.
    if args.subsample_train and len(train) > args.subsample_train:
        frac = args.subsample_train / len(train)
        train = (train.groupby("pattern_label", group_keys=False)
                 .apply(lambda g: g.sample(frac=frac, random_state=args.seed)))
    Xtr = train[FLOW_FEATURES].to_numpy(np.float64)
    ytr = train["pattern_label"].to_numpy()

    classes = sorted(df["pattern_label"].unique())
    print(f"train rows: {len(Xtr):,}  classes: {classes}")

    pipe = Pipeline([
        ("scaler", StandardScaler()),
        ("gb", GradientBoostingClassifier(
            n_estimators=args.estimators,
            learning_rate=0.1,
            max_depth=args.max_depth,
            random_state=args.seed,
        )),
    ])
    pipe.fit(Xtr, ytr)

    joblib.dump(pipe, out / "model.joblib")
    util.write_json(out / "label_classes.json", {"classes": list(pipe.classes_)})

    manifest = {
        "model_name": "flow-pattern",
        "model_version": "1.0.0",
        "algorithm": "gradient-boosting-classifier",
        "model_format": "onnx",
        "runtime": "onnxruntime-cpu",
        "feature_set": FLOW_FEATURES,
        "n_features": len(FLOW_FEATURES),
        "classes": list(pipe.classes_),
        "target": "pattern_label",
        "training_dataset": paths.FLOW_CSV.name,
        "training_dataset_sha256": util.sha256_file(paths.FLOW_CSV),
        "train_rows": int(len(Xtr)),
        "hyperparameters": {
            "n_estimators": args.estimators, "learning_rate": 0.1,
            "max_depth": args.max_depth, "random_state": args.seed,
            "stratified_subsample_train": args.subsample_train,
        },
        "preprocessing": "StandardScaler fit on train only (frozen)",
        "structural_evidence": "computed separately in flow.structural; "
                               "NOT an ML input. Production detector in Go "
                               "combines ML score + structural evidence.",
        "output_schema": {"name": "pattern_scores", "classes": list(pipe.classes_)},
        "seed": args.seed,
        "env": util.env_block(),
    }
    util.write_json(out / "training_manifest.json", manifest)
    print(f"saved {out/'model.joblib'}")
    print("flow training complete")


if __name__ == "__main__":
    main()
