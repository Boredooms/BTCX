"""Entity clustering evaluation (Gate 4).

Run: python -m ml_lab.evaluation.entity

Evaluates the frozen clustering on the validation split against true_entity_id
(scoring only): ARI, NMI, silhouette, noise rate, cluster-size distribution,
and stability across reseeded subsamples. Also reports the common-input
heuristic precision/recall as a separate deterministic signal.
"""

from __future__ import annotations

import joblib
import numpy as np
from sklearn.cluster import AgglomerativeClustering, DBSCAN
from sklearn.metrics import (
    adjusted_rand_score,
    normalized_mutual_info_score,
    silhouette_score,
)

from ml_lab import data, paths, schema, util
from ml_lab.training.entity.train import ENTITY_FEATURES


def _rebuild(name: str):
    """Reconstruct a fresh estimator from the selected algorithm name."""
    if name.startswith("dbscan_eps"):
        eps = float(name.replace("dbscan_eps", ""))
        return DBSCAN(eps=eps, min_samples=5, n_jobs=-1)
    if name.startswith("agglomerative_d"):
        dist = float(name.replace("agglomerative_d", ""))
        return AgglomerativeClustering(n_clusters=None, distance_threshold=dist,
                                       linkage="ward")
    raise ValueError(name)


def _cluster(name, X):
    est = _rebuild(name)
    return est.fit_predict(X)


def main() -> None:
    out = paths.EVALUATION / "entity"
    out.mkdir(parents=True, exist_ok=True)

    bundle = joblib.load(paths.MODELS / "entity" / "model.joblib")
    scaler = bundle["scaler"]
    algo = bundle["algorithm"]

    df = data.load_features(paths.ENTITY_CSV, extra_cols=["split", "true_entity_id",
                                                          "cluster_size",
                                                          "common_input_link_score",
                                                          "same_entity_ground_truth"])
    val = df[df["split"] == "validation"].reset_index(drop=True)
    # Cap evaluation size for the O(n^2) agglomerative path.
    cap = 20000
    if len(val) > cap:
        rng = np.random.RandomState(data.SEED + 1)
        val = val.iloc[rng.choice(len(val), cap, replace=False)].reset_index(drop=True)

    X = scaler.transform(val[ENTITY_FEATURES].to_numpy(float))
    truth = val["true_entity_id"].to_numpy()
    labels = _cluster(algo, X)

    n_clusters = len(set(labels)) - (1 if -1 in labels else 0)
    noise_rate = float((labels == -1).mean())
    sizes = {}
    for l in labels:
        if l == -1:
            continue
        sizes[int(l)] = sizes.get(int(l), 0) + 1
    size_values = sorted(sizes.values(), reverse=True)

    mask = labels != -1
    sil = None
    try:
        if n_clusters >= 2 and mask.sum() > n_clusters:
            sil = float(silhouette_score(X[mask], labels[mask]))
    except Exception:
        sil = None

    # Stability: re-run on reseeded subsamples, compare ARI vs truth variance.
    stab = []
    for s in range(3):
        rng = np.random.RandomState(data.SEED + 10 + s)
        sub = rng.choice(len(val), size=min(10000, len(val)), replace=False)
        lab_s = _cluster(algo, X[sub])
        stab.append(float(adjusted_rand_score(truth[sub], lab_s)))

    # Common-input heuristic (deterministic signal, evaluated separately).
    # Treat common_input_link_score >= 0.5 as a predicted same-entity link.
    ci = val["common_input_link_score"].to_numpy(float)
    gt = val["same_entity_ground_truth"].to_numpy(int)
    pred = (ci >= 0.5).astype(int)
    tp = int(((pred == 1) & (gt == 1)).sum())
    fp = int(((pred == 1) & (gt == 0)).sum())
    fn = int(((pred == 0) & (gt == 1)).sum())
    ci_prec = tp / (tp + fp) if (tp + fp) else 0.0
    ci_rec = tp / (tp + fn) if (tp + fn) else 0.0

    metrics = {
        "model": "entity-cluster",
        "algorithm": algo,
        "eval_n": int(len(val)),
        "ari": float(adjusted_rand_score(truth, labels)),
        "nmi": float(normalized_mutual_info_score(truth, labels)),
        "silhouette": sil,
        "n_clusters": int(n_clusters),
        "noise_rate": noise_rate,
        "cluster_size_top10": size_values[:10],
        "stability_ari": {"runs": stab, "mean": float(np.mean(stab)),
                          "std": float(np.std(stab))},
        "common_input_heuristic": {"precision": ci_prec, "recall": ci_rec,
                                   "threshold": 0.5},
        "note": "Clusters are INFERRED relationships, not proof of ownership.",
        "env": util.env_block(),
    }
    util.write_json(out / "metrics.json", metrics)
    _write_md(out, metrics)
    print(f"ARI={metrics['ari']:.4f} NMI={metrics['nmi']:.4f} "
          f"silhouette={sil} clusters={n_clusters} noise={noise_rate:.3f}")
    print(f"stability ARI mean={metrics['stability_ari']['mean']:.4f}")
    print(f"common-input heuristic P/R={ci_prec:.3f}/{ci_rec:.3f}")


def _write_md(out, m):
    L = [
        "# Entity Clustering Evaluation", "",
        f"Algorithm: `{m['algorithm']}`  (eval n={m['eval_n']:,})", "",
        "Clusters are **inferred relationships**, never proof of ownership.", "",
        "## Metrics (validation vs true_entity_id, scoring only)", "",
        f"- Adjusted Rand Index: **{m['ari']:.4f}**",
        f"- Normalized Mutual Info: **{m['nmi']:.4f}**",
        f"- Silhouette: {m['silhouette']}",
        f"- Clusters found: {m['n_clusters']}",
        f"- Noise rate: {m['noise_rate']:.4f}",
        f"- Largest clusters: {m['cluster_size_top10']}", "",
        "## Stability (reseeded subsamples)", "",
        f"- ARI runs: {[round(x,4) for x in m['stability_ari']['runs']]}",
        f"- mean={m['stability_ari']['mean']:.4f} std={m['stability_ari']['std']:.4f}", "",
        "## Common-input heuristic (separate deterministic signal)", "",
        f"- precision={m['common_input_heuristic']['precision']:.4f} "
        f"recall={m['common_input_heuristic']['recall']:.4f} "
        f"(threshold {m['common_input_heuristic']['threshold']})", "",
    ]
    (out / "report.md").write_text("\n".join(L))


if __name__ == "__main__":
    main()
