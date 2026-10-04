# BCTX ML Lab — Final Report

Feature schema: `feature-schema-v1`  hash `d52d7e12ee047c207231b9c165fe64ad831e0d6ec120eea2f0e9cfede5a2b589`

All models trained, evaluated and exported locally. Production inference
runs via local ONNX (anomaly, flow) and a frozen deterministic clustering
artifact (entity). No cloud, no runtime download. Offline proof passed.

## Gate status

| gate | status |
|------|--------|
| gate1_dataset_audit | PASS |
| gate2_feature_schema | PASS |
| gate3_anomaly_onnx | PASS |
| gate4_entity | PASS |
| gate5_flow_onnx | PASS |
| gate6_manifests | PASS |
| gate7_offline | PASS |

## Datasets

| dataset | rows | sha256 |
|---------|------|--------|
| anomaly | 934,374 | `498bfaf8f283ac65…` |
| entity | 731,564 | `c07047587d90e553…` |
| flow | 861,843 | `9e08f9ca242ec2f8…` |

Dataset audit critical issues: **0**

## Anomaly model (Isolation Forest)

- Primary metric PR-AUC (test): **0.9504**
- Precision/Recall/F1 (test): 0.877 / 0.920 / 0.898
- False-positive rate: 0.140
- Decision threshold (chosen on validation): 0.4203
- Per-scenario recall (test):
  - deep_chain: 0.844 (n=10904)
  - fanin_consolidation: 0.912 (n=10991)
  - fanout_splitting: 0.900 (n=10761)
  - large_value_jump: 0.843 (n=10870)
  - micro_fragmentation: 0.978 (n=10752)
  - mixed_anomaly: 0.818 (n=10879)
  - network_burst: 1.000 (n=10659)
  - rapid_burst: 0.998 (n=10796)
  - velocity_spike: 0.993 (n=10857)
- ONNX parity: max_abs_diff=2.13e-07 tol=0.0001 pass=True

## Entity model (clustering — inferred relationships)

- Algorithm: `agglomerative_d6.0`
- ARI: 0.0000  NMI: 0.7113  silhouette: 0.056555288645298935
- Clusters: 443  noise rate: 0.000
- Stability ARI mean: -0.0000
- Common-input heuristic (deterministic): P=1.000 R=1.000

**Interpretation (honest):** behavioral clustering recovers entity
*type* structure (NMI≈0.71) but cannot reconstruct the fine-grained
true-entity partition from behavior alone (ARI≈0): there are hundreds
of thousands of tiny ground-truth entities. This confirms the BCTX
design — the deterministic common-input heuristic (P/R≈1.0, Go-owned)
is the strong entity signal; ML clustering is a supplementary signal.
No ONNX by design (clustering is not a feed-forward transform).

## Flow model (Gradient Boosting classifier)

- Accuracy: 0.9985  macro-F1: **0.9966**  weighted-F1: 0.9985
- Macro one-vs-rest PR-AUC: 0.9980
- Per-pattern F1:
  - burst_consolidation: 0.997
  - fan_in: 0.999
  - fan_out: 0.999
  - layered_chain: 0.979
  - mixing_like: 1.000
  - normal: 0.999
  - peeling_chain: 1.000
  - rapid_flow: 1.000
- ONNX parity: max_abs_diff=2.14e-07 argmax_agree=1.0000 pass=True

**Note:** performance is high because synthetic pattern signatures are
cleanly separable. Real-world flows will be noisier; external
validation is required before production claims. Structural detectors
(deterministic) are kept separate from the ML score and combined in Go.

## ONNX / parity

| model | format | parity max_abs_diff | pass |
|-------|--------|---------------------|------|
| anomaly | onnx | 2.13e-07 | True |
| flow | onnx | 2.14e-07 | True |
| entity | joblib (no onnx by design) | n/a | n/a |

## Model artifacts

| artifact | bytes |
|----------|-------|
| anomaly/model.onnx | 1,382,684 |
| anomaly/model.joblib | 1,860,163 |
| entity/model.joblib | 641,648 |
| flow/model.onnx | 532,240 |
| flow/model.joblib | 1,710,723 |

## Offline proof

- offline_pass: **True**
- AF_INET/AF_INET6 connect blocked for this process
- elapsed: 4.85s

## Manifest validation (Gate 6)

- anomaly: OK (format=onnx, version=1.0.0)
- entity: OK (format=joblib-frozen-deterministic, version=1.0.0)
- flow: OK (format=onnx, version=1.0.0)

## Reproducibility

- Deterministic generator seed: 20261002
- All models fixed random_state; preprocessing fit on train only, frozen.
- Each model has training_manifest.json with dataset+schema hashes,
  hyperparameters, library versions, git commit and timestamp.
- Environment: {'python': '3.12.3', 'os': 'Linux 6.18.40.1-microsoft-standard-WSL2', 'numpy': '1.26.4', 'sklearn': '1.5.1', 'git_commit': 'none', 'timestamp': '2026-10-02T16:36:06Z', 'onnx': '1.17.0', 'onnxruntime': '1.18.1', 'skl2onnx': '1.20.0'}

## Known limitations

- Datasets are synthetic; metrics are not real-world performance.
- Entity ARI is low by nature of fine-grained synthetic entities; the
  common-input heuristic is the authoritative entity signal.
- Flow metrics are optimistic due to clean synthetic separability.
- Anomaly FPR≈0.14 at the F1-optimal threshold; tune per deployment.
- torch-based flow MLP comparison deferred (baseline already strong).
