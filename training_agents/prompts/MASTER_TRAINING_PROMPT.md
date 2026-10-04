# MASTER TRAINING PROMPT — BCTX ML Lab

You are building the BCTX ML laboratory **from scratch**, locally, offline-ready,
and fully reproducible. Everything is trained, evaluated, and exported for local
Go/ONNX inference. No cloud, no pretrained BCTX models, no fake artifacts.

## The objective

> A real, reproducible, from-scratch BCTX ML laboratory producing validated
> local model artifacts (ONNX where valid, frozen deterministic artifacts
> otherwise) that integrate into the Go production runtime.

## Non-negotiable constraints

- **Offline boundary:** training may use local files only. Production inference
  must never download a model or call an external API. Prove it (Gate 7).
- **Single feature contract:** `feature-schema-v1`, 25 features, fixed order.
  Python and the future Go engine must compute identical names/order/units/
  formulas/missing-value handling.
- **Leakage control:** these columns are labels/metadata and must NEVER be model
  inputs — `ground_truth_anomaly`, `true_entity_id`, `same_entity_ground_truth`,
  `pattern_label`, `suspicious_ground_truth`, `scenario`, `entity_type`,
  `cluster_size`, `relationship_basis`, `common_input_link_score`.
- **No fake success:** report real metrics only. If a model is weak, say so and
  investigate. Never manipulate labels to fake performance.
- **Determinism:** fixed seeds, recorded splits, recorded library versions,
  recorded hashes. A model must be reproducible from its `training_manifest.json`.
- **Fit on train only:** preprocessing/calibration statistics are fit on the
  training split and frozen. Never fit on validation/test.

## Model plan

1. **anomaly** (first): Isolation Forest, unsupervised, fit on normal-only train
   rows. Output `anomaly_score ∈ [0,1]` via a documented, reproducible transform.
   Evaluate on held-out injected anomalies (PR-AUC, precision/recall/F1, FPR,
   per-scenario recall). Export ONNX; verify Python↔ONNX parity.
2. **entity** (second): behavioral + graph-neighborhood clustering. Compare
   DBSCAN and Agglomerative. Evaluate ARI/NMI/silhouette/stability/noise.
   If not naturally ONNX-representable, freeze the deterministic implementation
   + config and document why — do NOT make a fake ONNX file.
3. **flow** (third): supervised gradient boosting over flow features, target =
   `pattern_label` only. Per-pattern precision/recall/F1 + confusion + PR-AUC.
   Keep structural evidence separate from the ML score. Export ONNX; verify parity.

## Gates (do not advance past a failed gate)

1. Dataset audit passes.
2. Feature schema frozen + golden fixtures created.
3. Anomaly trained, evaluated, ONNX verified.
4. Entity clustering evaluated.
5. Flow evaluated, structural detectors verified.
6. Manifests valid, checksums valid.
7. Offline inference passes.

## Environment

- Python: `ml-lab/.venv/bin/python` (sudo-free venv; IPv4-forced for installs).
- Code lives in the importable package `ml_lab/` with `-m` CLI entry points.
- Deterministic generator: `ml-lab/generate.go`, seed `20261002`.

## Deliverable report

`ml-lab/evaluation/final_report.md` with dataset summary + hashes, feature
schema, leakage controls, split strategy, per-model metrics, ONNX verification,
Python-vs-ONNX parity, latency, model size, known limitations, reproducibility.
