# ml-lab/ — Python training & research environment

This is the **offline model development** workspace. It is NOT shipped to end
users and NOT required at runtime. Go + ONNX Runtime handle inference in
production; this directory only produces the `.onnx` files + manifests that get
packaged into `models/`.

```
ml-lab/
├── datasets/    # generated synthetic datasets (gitignored)
├── generators/  # deterministic synthetic data generators (seeded)
├── features/    # feature extraction mirroring Go feature-schema-v1
├── training/    # training scripts per model
├── evaluation/  # precision/recall/F1/PR-AUC, latency
├── notebooks/   # exploration
└── export/      # ONNX export + manifest generation
```

## Pipeline

```
synthetic data (seeded)
   -> feature extraction (feature-schema-v1)
   -> train (scikit-learn / PyTorch)
   -> evaluate (held-out, no leakage)
   -> export ONNX + manifest (version, feature schema, checksum)
   -> copy into ../models/<model>/
```

## Setup

```bash
cd ml-lab
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

## The three models to build (in order)

See `MODELS.md` for the full spec, inputs, and training recipe for each.

1. **anomaly** — Isolation Forest baseline (unsupervised).
2. **entity** — behavioral/graph clustering (+ common-input heuristic in Go).
3. **flow** — supervised classifier over flow features, hybrid with Go graph
   detectors.

---

## STATUS: ML lab built and verified (all 7 gates PASS)

The lab is implemented as the importable `ml_lab/` package with CLI entry
points and a `Makefile`. All three models are trained, evaluated, exported and
offline-verified from local data only.

```bash
cd ml-lab
make env     # one-time: sudo-free venv + pinned deps (requirements.lock.txt)
make all     # freeze -> audit -> anomaly -> entity -> flow -> offline -> report
```

Outputs:
- `evaluation/feature_schema_v1.json`, `feature_definitions.md`, `feature_golden.json`
- `evaluation/dataset_audit.{json,md}`
- `evaluation/{anomaly,entity,flow}/{metrics.json,report.md}`
- `export/{anomaly,flow}_parity.json`
- `evaluation/offline_proof.{json,md}`
- `evaluation/final_report.{md,json}`  ← start here
- `models/{anomaly,flow}/model.onnx` + `models/*/manifest.json`
- `models/entity/model.joblib` (frozen deterministic clustering, no ONNX by design)

Headline results (test splits; synthetic data):
- anomaly PR-AUC **0.9504**
- flow macro-F1 **0.9966**
- entity NMI **0.71** (common-input heuristic P/R ≈ 1.0 is the strong signal)

Environment note: this WSL instance has broken outbound IPv6; the setup scripts
force IPv4 for pip. Inference needs no network at all (proven under
`unshare -rn` + a socket guard).
