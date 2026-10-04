# BCTX ML Runtime (Go)

How the Go backend consumes the validated ML artifacts produced by `ml-lab/`.
Everything here is local and offline: no native onnxruntime library, no model
download, no external API in the analysis path.

## Model locations

Packaged under `models/` at the repo/install root:

```
models/
├── anomaly/
│   ├── model.trees.json     # tree ensemble extracted from model.onnx (authoritative)
│   ├── calibration.json     # score -> [0,1] transform
│   └── manifest.json        # version, feature schema, checksums, parity
├── flow/
│   ├── model.trees.json
│   ├── label_classes.json
│   └── manifest.json
└── entity/
    └── manifest.json        # frozen deterministic clustering (no ONNX by design)
```

The original `.onnx` files remain in `ml-lab/models/` as the source of truth;
`model.trees.json` is derived from them (and records `source_onnx_sha256`).

## Why not the native ONNX Runtime?

The three models are **tree ensembles** (sklearn IsolationForest →
TreeEnsembleRegressor; GradientBoosting → TreeEnsembleClassifier). Linking the
native `libonnxruntime.so` would add a heavy, offline-fragile CGO/dlopen
dependency that conflicts with BCTX's "single native binary, minimal deps" goal.

Instead, the ML lab extracts the exact tree parameters from the real
`model.onnx` (via the `onnx` parser, authoritative) into `model.trees.json`,
and `ml/inference` evaluates the actual trained trees in pure Go. This is real
inference on the real model — verified equal to ONNX Runtime within ~1e-7 on
golden vectors (`ml/inference/parity_test.go`).

## Feature contract

`feature-schema-v1` — 25 features, fixed order, hash
`d52d7e12ee047c207231b9c165fe64ad831e0d6ec120eea2f0e9cfede5a2b589` (constant
`schema.FeatureSchemaSHA256`). The Go feature engine (`ml/features/engine.go`)
computes these from local SQLite records with the exact missing-value and
zero-denominator rules from `ml-lab/evaluation/feature_definitions.md`.
Parity with the Python golden fixtures is enforced by
`ml/features/golden_test.go`.

The **flow** model uses a separate 17-feature set (`flow-features-v1`); see
`detection/flow/structural.go` `ModelFeatureOrder`.

## Anomaly calibration

ONNX `scores` output == sklearn `decision_function`. The Go runtime computes:

```
raw           = -decision_function
anomaly_score = clamp((raw - raw_lo) / (raw_hi - raw_lo), 0, 1)
```

with `raw_lo`/`raw_hi` from `models/anomaly/calibration.json`. The IsolationForest
path length is reconstructed exactly (`ml/inference/iforest.go`):
`depth(leaf) + c(n_samples)` summed over 200 trees, `/ denominator`, then
`decision = -2^(-avg) - offset`.

## Flow class mapping

Classes are read from the model (NOT alphabetised). Order:

```
burst_consolidation, fan_in, fan_out, layered_chain,
mixing_like, normal, peeling_chain, rapid_flow
```

Flow suspicion fed to risk = `1 - P(normal)`.

## Entity strategy

The authoritative entity signal is the deterministic **common-input heuristic**
(`detection/entity/common_input.go`): addresses co-spent as inputs to the same
transaction are inferred co-controlled (precision/recall ≈ 1.0 on the synthetic
corpus). The ml-lab behavioral clustering is a supplementary signal and is not
run in the Go path (no valid ONNX representation). A cluster is an **inferred
relationship, never proof of ownership**.

## Model loading, validation, lifecycle

`ml/inference.Registry` lazy-loads and caches compiled models. On load it
validates the manifest and, for feature-schema-v1 models, fail-closes on a
feature-schema hash mismatch. Tree evaluation is read-only and safe for
concurrent use; `Registry.Close()` clears the cache (no native resources).

## Investigation pipeline

`investigation/orchestrator` runs, in fixed order:

```
local data -> features -> anomaly ONNX -> entity (common-input) ->
flow ONNX + structural detectors -> risk -> evidence -> InvestigationResult
```

Consumed identically by CLI (`bctx analyze wallet <id>`, `--json`, `--offline`),
and by future TUI/reports.

## Risk

`risk/scoring` aggregates anomaly + flow + entity with configurable weights
(default 0.50 / 0.35 / 0.15) into a 0–100 score with confidence, a per-signal
breakdown and linked evidence IDs. Deterministic given the same inputs. Risk is
an investigative prioritization signal, not proof of wrongdoing. The LLM never
calculates risk.

## Offline guarantee

The analysis path imports no HTTP client. Verified by running
`bctx analyze wallet <id> --offline` under `unshare -rn` (network namespace
dropped) — identical output, mode OFFLINE. Reproduce:

```bash
make build
# in an initialized case with imported data:
unshare -rn ./bin/bctx analyze wallet W123 --offline
```

## Reproducing the artifacts

```bash
cd ml-lab
make all                                   # trains + exports ONNX + manifests
.venv/bin/python -m ml_lab.export.onnx_to_treejson     # ONNX -> model.trees.json
.venv/bin/python -m ml_lab.export.golden_inference     # Go parity vectors
# copy models/<m>/{model.trees.json,calibration.json,manifest.json,label_classes.json}
# into the repo-root models/<m>/
```
