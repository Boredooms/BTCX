"""Entity clustering trainer — inferred relationship signal (NOT ownership).

Run: python -m ml_lab.training.entity.train [--seed N] [--sample N]

Compares DBSCAN and Agglomerative clustering on standardized behavioral +
graph-neighborhood features (feature-schema-v1), fit on the `fit` split. Ground
truth (`true_entity_id`) is used ONLY for evaluation, never as a model input.

ONNX note: clustering (DBSCAN/Agglomerative) is not a feed-forward transform and
is not validly ONNX-exportable. We therefore freeze the deterministic pipeline
(scaler + algorithm + params) as model.joblib and document the decision in the
manifest. The Go runtime reproduces the deterministic routine; it does NOT run
a fake ONNX graph.
"""

from __future__ import annotations

import argparse

import joblib
import numpy as np
from sklearn.cluster import AgglomerativeClustering, DBSCAN
from sklearn.metrics import (
    adjusted_rand_score,
    normalized_mutual_info_score,
    silhouette_score,
)
from sklearn.preprocessing import StandardScaler

from ml_lab import data, paths, schema, util

# Behavioral + graph-neighborhood subset used for entity similarity. These are
# all legitimate feature-schema-v1 inputs (no ground truth).
ENTITY_FEATURES = [
    "avg_amount", "amount_variance", "tx_per_hour", "velocity", "burstiness",
    "degree", "fan_in", "fan_out", "counterparty_diversity",
    "incoming_volume", "outgoing_volume", "fee_mean",
]


def _metrics(X, labels, truth):
    mask = labels != -1  # exclude DBSCAN noise from cohesion metrics
    n_clusters = len(set(labels)) - (1 if -1 in labels else 0)
    noise_rate = float((labels == -1).mean())
    out = {
        "n_clusters": int(n_clusters),
        "noise_rate": noise_rate,
        "ari": float(adjusted_rand_score(truth, labels)),
        "nmi": float(normalized_mutual_info_score(truth, labels)),
    }
    # silhouette needs >=2 clusters and non-noise points.
    try:
        if n_clusters >= 2 and mask.sum() > n_clusters:
            out["silhouette"] = float(silhouette_score(X[mask], labels[mask]))
        else:
            out["silhouette"] = None
    except Exception:
        out["silhouette"] = None
    return out


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, default=data.SEED)
    ap.add_argument("--sample", type=int, default=20000,
                    help="subsample size for the O(n^2) agglomerative comparison")
    args = ap.parse_args()

    paths.ensure_dirs()
    out = paths.MODELS / "entity"
    out.mkdir(parents=True, exist_ok=True)

    print("loading entity dataset ...")
    df = data.load_features(paths.ENTITY_CSV,
                            extra_cols=["split", "true_entity_id"])
    schema.assert_no_leakage(ENTITY_FEATURES)

    fit_df = df[df["split"] == "fit"].reset_index(drop=True)
    rng = np.random.RandomState(args.seed)
    idx = rng.choice(len(fit_df), size=min(args.sample, len(fit_df)), replace=False)
    sub = fit_df.iloc[idx].reset_index(drop=True)

    scaler = StandardScaler().fit(sub[ENTITY_FEATURES].to_numpy(float))
    X = scaler.transform(sub[ENTITY_FEATURES].to_numpy(float))
    truth = sub["true_entity_id"].to_numpy()

    print(f"fit sample: {X.shape[0]:,} wallets x {X.shape[1]} features")

    candidates = {}

    # DBSCAN — density based; eps tuned coarsely.
    for eps in (1.5, 2.0, 2.5):
        db = DBSCAN(eps=eps, min_samples=5, n_jobs=-1).fit(X)
        candidates[f"dbscan_eps{eps}"] = (db, db.labels_)

    # Agglomerative — distance-threshold based.
    for dist in (6.0, 9.0):
        ag = AgglomerativeClustering(n_clusters=None, distance_threshold=dist,
                                     linkage="ward")
        lab = ag.fit_predict(X)
        candidates[f"agglomerative_d{dist}"] = (ag, lab)

    scored = {}
    for name, (_model, labels) in candidates.items():
        scored[name] = _metrics(X, labels, truth)
        print(f"  {name}: {scored[name]}")

    # Select best by ARI (primary cluster-quality-vs-truth metric), tie-break NMI.
    best_name = max(scored, key=lambda k: (scored[k]["ari"], scored[k]["nmi"]))
    best_model = candidates[best_name][0]
    print(f"best: {best_name} ari={scored[best_name]['ari']:.4f}")

    # Freeze scaler + chosen algorithm config. (Fitted estimator saved too, but
    # the authoritative deployable artifact is the config + scaler, since
    # clustering is re-run on the target population in production.)
    bundle = {
        "scaler": scaler,
        "algorithm": best_name,
        "model": best_model,
        "entity_features": ENTITY_FEATURES,
    }
    joblib.dump(bundle, out / "model.joblib")

    config_hash = util.sha256_file(out / "model.joblib")
    manifest = {
        "model_name": "entity-cluster",
        "model_version": "1.0.0",
        "algorithm": best_name,
        "model_format": "joblib-frozen-deterministic",
        "onnx": False,
        "onnx_reason": "DBSCAN/Agglomerative clustering is not a feed-forward "
                       "transform and has no valid ONNX representation. The "
                       "deterministic pipeline (scaler + algorithm + params) is "
                       "frozen; the Go runtime reproduces the routine. No fake "
                       "ONNX file is produced.",
        "feature_schema": schema.FEATURE_SCHEMA_VERSION,
        "feature_schema_sha256": schema.schema_hash(),
        "entity_features": ENTITY_FEATURES,
        "training_dataset": paths.ENTITY_CSV.name,
        "training_dataset_sha256": util.sha256_file(paths.ENTITY_CSV),
        "fit_sample_size": int(X.shape[0]),
        "candidates_evaluated": scored,
        "selected": best_name,
        "model_sha256": config_hash,
        "output_schema": {"cluster_id": "int", "confidence": "float",
                          "basis": "list[str]"},
        "disclaimer": "A cluster is an INFERRED relationship, not proof of "
                      "real-world ownership.",
        "seed": args.seed,
        "env": util.env_block(),
    }
    util.write_json(out / "manifest.json", manifest)
    util.write_json(out / "training_manifest.json", manifest)
    data.record_split_manifest("entity", paths.ENTITY_CSV,
                               df["split"].value_counts().to_dict())
    # Persist the comparison for the evaluator/report.
    util.write_json(paths.EVALUATION / "entity" / "candidates.json", scored)
    print("entity training complete")


if __name__ == "__main__":
    main()
