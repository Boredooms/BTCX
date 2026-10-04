# BCTX ML Models — What to Train (from scratch, local, Linux)

All models are trained in Python here in `ml-lab/`, exported to **ONNX**, and
run in production by the Go runtime via **ONNX Runtime** — fully offline, no
cloud, no runtime download. Every model ships with a `manifest.json`
(version + feature schema + checksum).

The problem statement names four ML focus areas: **anomaly detection, entity
clustering, peeling-chain/mixing detection, risk scoring**. BCTX maps those to
three packaged models plus a Go-side risk aggregator.

---

## Feature schema (shared contract): `feature-schema-v1`

The Go feature engine (`ml/features/`) and the Python trainer must compute the
SAME features in the SAME order. Candidate features (per wallet/subject):

```
tx_count, incoming_count, outgoing_count,
incoming_volume, outgoing_volume, avg_amount, amount_variance,
fee_mean, tx_per_hour, median_time_gap, burstiness, velocity,
degree, fan_in, fan_out, counterparty_diversity,
graph_depth, hop_count, chain_length, value_decay, split_ratio, merge_ratio,
obs_count, unique_ip_count, unique_asn_count   # only when network evidence exists
```

---

## Model 1 — `anomaly` (BUILD FIRST)

- **Goal:** flag wallets/transactions whose behavior is statistically unusual
  vs learned normal behavior.
- **Type:** unsupervised. **Baseline: Isolation Forest** (scikit-learn).
  Alternatives to compare later: One-Class SVM, autoencoder (PyTorch).
- **Input:** `feature-schema-v1` vector (numeric).
- **Output:** `anomaly_score ∈ [0,1]` (normalize `decision_function`).
- **Why first:** no labels needed, works directly on the synthetic generator
  output, and immediately feeds the risk engine + evidence.
- **Export:** `skl2onnx` → `models/anomaly/model.onnx`.
- **Eval:** inject known anomaly scenarios from the generator; report
  precision/recall/F1/PR-AUC + false-positive rate.

## Model 2 — `entity` (BUILD SECOND)

- **Goal:** group wallets that appear related (inferred, not proven ownership).
- **Type:** feature-based clustering + graph similarity. Start with the
  **common-input heuristic in Go** (deterministic) and a clustering model
  (e.g. DBSCAN/agglomerative on behavioral+graph features) exported where it
  makes sense. Later: graph embeddings (Node2Vec/GraphSAGE).
- **Input:** behavioral features + graph-neighborhood features.
- **Output:** cluster assignment + confidence + basis signals.
- **Note:** clustering output phrased as inferred relationship only.

## Model 3 — `flow` (BUILD THIRD)

- **Goal:** score suspicious flow structures (peeling-chain-like, mixing-like,
  rapid-flow, high fan-in/out).
- **Type:** hybrid — Go graph detectors produce structural features; a
  **supervised classifier** (gradient boosting / small MLP) scores them using
  generator ground-truth labels.
- **Input:** flow features (`chain_length, value_decay, split_ratio,
  merge_ratio, hop_count, fan_in, fan_out, timing`).
- **Output:** per-pattern score + confidence.
- **Export:** ONNX; detectors in `detection/` combine model score + structural
  evidence.

---

## Risk aggregator (NOT an ONNX model — Go, `risk/`)

Risk is deterministic Go code combining the three model scores + graph +
propagation into a 0–100 score with a signal breakdown. **The LLM and the
models never ARE the risk score — they feed it.**

---

## Local LLM (optional, summaries only)

`gemma3:1b` via Ollama can turn already-computed structured evidence into
readable prose for reports/monitor summaries. It must never invent evidence,
transactions, IP associations, or the risk score. Endpoint is configurable in
`config.toml` (`[llm]`), disabled by default.

---

## Build order checklist

- [ ] Deterministic synthetic generator with ground-truth labels (`generators/`)
- [ ] Go + Python feature parity for `feature-schema-v1`
- [ ] Train + evaluate `anomaly` (Isolation Forest) → export ONNX + manifest
- [ ] Integrate ONNX Runtime in Go (`ml/inference/`), prove offline inference
- [ ] `entity` clustering + common-input heuristic
- [ ] `flow` detectors + supervised scorer
- [ ] Risk aggregation + propagation in Go

---

## BUILD STATUS (ML phase complete — all 7 gates PASS)

Trained, evaluated, exported and offline-verified locally. See
`evaluation/final_report.md` for the authoritative numbers.

| model | algorithm | key metric | ONNX | parity |
|-------|-----------|------------|------|--------|
| anomaly | Isolation Forest | PR-AUC 0.9504 (test) | yes (1.38 MB) | 2.1e-7 |
| entity | Agglomerative clustering | NMI 0.71 / ARI ~0 (see note) | no (by design) | n/a |
| flow | Gradient Boosting | macro-F1 0.9966 (test) | yes (532 KB) | 2.1e-7 |

Reproduce the whole pipeline:

```bash
cd ml-lab
make env      # one-time: sudo-free venv + pinned deps (IPv4)
make all      # freeze -> audit -> anomaly -> entity -> flow -> offline -> report
```

### Entity note (honest)
Behavioral clustering recovers entity *type* structure (NMI≈0.71) but cannot
reconstruct the fine-grained `true_entity_id` partition from behavior alone
(ARI≈0) — the synthetic corpus has hundreds of thousands of tiny entities. The
deterministic **common-input heuristic** (precision/recall ≈ 1.0, owned by Go)
is the authoritative entity signal; ML clustering is supplementary. This is the
intended BCTX design, not a defect.

### Flow note (honest)
Metrics are high because synthetic pattern signatures are cleanly separable.
Real-world flows are noisier; external validation is required before production
performance claims. Deterministic structural detectors are kept separate from
the ML score and combined in the Go runtime.

### Toolchain pins that matter
- `skl2onnx==1.20.0` (older 1.17/1.18 raise "Expected an int, got a boolean"
  on tree-ensemble conversion).
- Export with `target_opset={'': 15, 'ai.onnx.ml': 3}` (onnxruntime 1.18 only
  supports `ai.onnx.ml` v3).
- Anomaly ONNX `scores` output == sklearn `decision_function`; the Go runtime
  computes `anomaly_score = clamp((-scores - raw_lo)/(raw_hi - raw_lo), 0, 1)`
  using `models/anomaly/calibration.json`.

### Local LLM (optional, summaries only)
`gemma3:1b` via Ollama is NOT part of training and never produces a score. It
runs on the Windows host; from WSL reach it via the host IP (default route
gateway) at port 11434, configured in `config.toml` `[llm]`, disabled by
default.
