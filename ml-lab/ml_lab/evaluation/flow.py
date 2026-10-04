"""Flow model evaluation (Gate 5).

Run: python -m ml_lab.evaluation.flow

Per-pattern precision/recall/F1 + macro/weighted averages, full confusion
matrix, and macro one-vs-rest PR-AUC. Evaluated on the held-out test split.
"""

from __future__ import annotations

import joblib
import numpy as np
import pandas as pd
from sklearn.metrics import (
    classification_report,
    confusion_matrix,
    average_precision_score,
)
from sklearn.preprocessing import label_binarize

from ml_lab import paths, util
from ml_lab.training.flow.train import FLOW_FEATURES


def main() -> None:
    out = paths.EVALUATION / "flow"
    out.mkdir(parents=True, exist_ok=True)

    pipe = joblib.load(paths.MODELS / "flow" / "model.joblib")
    classes = list(pipe.classes_)

    cols = FLOW_FEATURES + ["split", "pattern_label"]
    df = pd.read_csv(paths.FLOW_CSV, usecols=cols, low_memory=False)
    test = df[df["split"] == "test"]
    X = test[FLOW_FEATURES].to_numpy(np.float64)
    y = test["pattern_label"].to_numpy()

    pred = pipe.predict(X)
    proba = pipe.predict_proba(X)

    report = classification_report(y, pred, labels=classes, output_dict=True,
                                   zero_division=0)
    cm = confusion_matrix(y, pred, labels=classes).tolist()

    # Macro one-vs-rest PR-AUC.
    Y = label_binarize(y, classes=classes)
    pr_auc = {}
    for i, c in enumerate(classes):
        try:
            pr_auc[c] = float(average_precision_score(Y[:, i], proba[:, i]))
        except Exception:
            pr_auc[c] = None
    valid = [v for v in pr_auc.values() if v is not None]
    macro_pr_auc = float(np.mean(valid)) if valid else None

    metrics = {
        "model": "flow-pattern",
        "classes": classes,
        "test_rows": int(len(y)),
        "accuracy": float(report["accuracy"]),
        "macro_f1": float(report["macro avg"]["f1-score"]),
        "weighted_f1": float(report["weighted avg"]["f1-score"]),
        "per_class": {c: report[c] for c in classes},
        "pr_auc_ovr": pr_auc,
        "macro_pr_auc": macro_pr_auc,
        "confusion_matrix": {"labels": classes, "matrix": cm},
        "env": util.env_block(),
    }
    util.write_json(out / "metrics.json", metrics)
    util.write_json(out / "confusion.json", metrics["confusion_matrix"])
    _write_md(out, metrics)

    print(f"accuracy={metrics['accuracy']:.4f} macro_f1={metrics['macro_f1']:.4f} "
          f"weighted_f1={metrics['weighted_f1']:.4f} macro_pr_auc={macro_pr_auc:.4f}")
    for c in classes:
        r = report[c]
        print(f"  {c:20s} P={r['precision']:.3f} R={r['recall']:.3f} "
              f"F1={r['f1-score']:.3f} n={int(r['support'])}")


def _write_md(out, m):
    L = [
        "# Flow Pattern Model Evaluation", "",
        f"Classes: {m['classes']}  (test n={m['test_rows']:,})", "",
        f"- accuracy: {m['accuracy']:.4f}",
        f"- macro F1: **{m['macro_f1']:.4f}**",
        f"- weighted F1: {m['weighted_f1']:.4f}",
        f"- macro one-vs-rest PR-AUC: {m['macro_pr_auc']:.4f}", "",
        "Patterns are **matches**, not proof of criminal activity.", "",
        "## Per-pattern metrics", "",
        "| pattern | precision | recall | F1 | support | PR-AUC |",
        "|---------|-----------|--------|----|---------|--------|",
    ]
    for c in m["classes"]:
        r = m["per_class"][c]
        pa = m["pr_auc_ovr"].get(c)
        pa_s = f"{pa:.3f}" if pa is not None else "n/a"
        L.append(f"| {c} | {r['precision']:.3f} | {r['recall']:.3f} | "
                 f"{r['f1-score']:.3f} | {int(r['support'])} | {pa_s} |")
    L += ["", "Confusion matrix in `confusion.json`.", ""]
    (out / "report.md").write_text("\n".join(L))


if __name__ == "__main__":
    main()
