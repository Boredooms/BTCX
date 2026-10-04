"""Anomaly evaluation on held-out validation + test (which contain anomalies).

Run: python -m ml_lab.evaluation.anomaly

Primary metric: PR-AUC (imbalanced). Also precision/recall/F1 at a threshold
chosen on validation, FPR, confusion matrix, per-scenario recall. Accuracy is
NOT headlined.
"""

from __future__ import annotations

import joblib
import numpy as np
from sklearn.metrics import (
    average_precision_score,
    confusion_matrix,
    precision_recall_curve,
    precision_score,
    recall_score,
    f1_score,
)

from ml_lab import data, paths, schema, util
from ml_lab.training.anomaly.train import raw_scores


def calibrated_scores(pipe, calib, X):
    rs = raw_scores(pipe, X)
    lo, hi = calib["raw_lo"], calib["raw_hi"]
    return np.clip((rs - lo) / (hi - lo), 0.0, 1.0)


def pick_threshold(y, scores):
    """Choose the threshold maximizing F1 on the given (validation) set."""
    prec, rec, thr = precision_recall_curve(y, scores)
    # thr has len = len(prec)-1
    f1s = []
    for p, r in zip(prec[:-1], rec[:-1]):
        f1s.append(0.0 if (p + r) == 0 else 2 * p * r / (p + r))
    if not f1s:
        return 0.5
    return float(thr[int(np.argmax(f1s))])


def main() -> None:
    out = paths.EVALUATION / "anomaly"
    out.mkdir(parents=True, exist_ok=True)

    pipe = joblib.load(paths.MODELS / "anomaly" / "model.joblib")
    calib = util.read_json(paths.MODELS / "anomaly" / "calibration.json")

    df = data.load_features(paths.ANOMALY_CSV,
                            extra_cols=["split", "ground_truth_anomaly", "scenario"])
    feats = list(schema.FEATURES)

    val = df[df["split"] == "validation"]
    test = df[df["split"] == "test"]

    yval = val["ground_truth_anomaly"].to_numpy(int)
    sval = calibrated_scores(pipe, calib, val[feats].to_numpy(float))
    ytest = test["ground_truth_anomaly"].to_numpy(int)
    stest = calibrated_scores(pipe, calib, test[feats].to_numpy(float))

    thr = pick_threshold(yval, sval)

    def metrics_at(y, s, thr):
        pred = (s >= thr).astype(int)
        cm = confusion_matrix(y, pred, labels=[0, 1])
        tn, fp, fn, tp = cm.ravel()
        fpr = float(fp / (fp + tn)) if (fp + tn) else 0.0
        return {
            "pr_auc": float(average_precision_score(y, s)),
            "precision": float(precision_score(y, pred, zero_division=0)),
            "recall": float(recall_score(y, pred, zero_division=0)),
            "f1": float(f1_score(y, pred, zero_division=0)),
            "fpr": fpr,
            "threshold": float(thr),
            "confusion": {"tn": int(tn), "fp": int(fp), "fn": int(fn), "tp": int(tp)},
            "n": int(len(y)),
            "n_anomaly": int(y.sum()),
        }

    val_m = metrics_at(yval, sval, thr)
    test_m = metrics_at(ytest, stest, thr)

    # Per-scenario recall on test (how well each anomaly type is caught).
    per_scenario = {}
    tpred = (stest >= thr).astype(int)
    for sc in sorted(test["scenario"].unique()):
        if sc == "normal":
            continue
        m = test["scenario"].to_numpy() == sc
        if m.sum() == 0:
            continue
        per_scenario[sc] = {
            "n": int(m.sum()),
            "recall": float(tpred[m].mean()),  # all these rows are anomalies
        }

    metrics = {
        "model": "anomaly-detector",
        "primary_metric": "pr_auc",
        "validation": val_m,
        "test": test_m,
        "per_scenario_recall_test": per_scenario,
        "counts": {
            "normal_train": int(((df["split"] == "train") & (df["ground_truth_anomaly"] == 0)).sum()),
            "val_total": int(len(yval)), "val_anomaly": int(yval.sum()),
            "test_total": int(len(ytest)), "test_anomaly": int(ytest.sum()),
        },
        "env": util.env_block(),
    }
    util.write_json(out / "metrics.json", metrics)
    util.write_json(out / "confusion.json", {"validation": val_m["confusion"], "test": test_m["confusion"]})
    _write_md(out, metrics)

    print(f"PR-AUC test = {test_m['pr_auc']:.4f}")
    print(f"P/R/F1 test = {test_m['precision']:.3f}/{test_m['recall']:.3f}/{test_m['f1']:.3f}  FPR={test_m['fpr']:.3f}")
    print("per-scenario recall:", {k: round(v["recall"], 3) for k, v in per_scenario.items()})


def _write_md(out, m):
    t = m["test"]
    L = [
        "# Anomaly Model Evaluation", "",
        "Primary metric: **PR-AUC** (imbalanced problem; accuracy intentionally not headlined).", "",
        "## Test set", "",
        f"- PR-AUC: **{t['pr_auc']:.4f}**",
        f"- Precision: {t['precision']:.4f}",
        f"- Recall: {t['recall']:.4f}",
        f"- F1: {t['f1']:.4f}",
        f"- False-positive rate: {t['fpr']:.4f}",
        f"- Threshold (chosen on validation): {t['threshold']:.4f}",
        f"- Confusion (test): {t['confusion']}",
        f"- n={t['n']:,}  anomalies={t['n_anomaly']:,}", "",
        "## Per-scenario recall (test)", "",
        "| scenario | n | recall |",
        "|----------|---|--------|",
    ]
    for sc, v in m["per_scenario_recall_test"].items():
        L.append(f"| {sc} | {v['n']} | {v['recall']:.3f} |")
    L += ["", "## Counts", "", f"- {m['counts']}", ""]
    (out / "report.md").write_text("\n".join(L))


if __name__ == "__main__":
    main()
