# BCTX Training Agents

This directory defines the **ML training agent system** for BCTX. These are
specialist operating specifications — each describes one worker's scope,
inputs, authoritative outputs, and the gate it must pass before the next
worker runs. They are not independent uncontrolled developers; the
`orchestrator` controls ordering and enforces gates.

The agents exist so the ML lab is built the same way every time: auditable,
reproducible, leakage-controlled, and never faked.

## Execution order (authoritative)

```
dataset_auditor
      ↓   (Gate 1: dataset audit passes)
feature_engineer
      ↓   (Gate 2: feature-schema-v1 frozen + golden fixtures)
anomaly_trainer
      ↓
evaluator (anomaly)
      ↓   (Gate 3: anomaly trained, evaluated, ONNX verified)
entity_trainer
      ↓
evaluator (entity)
      ↓   (Gate 4: entity clustering evaluated)
flow_trainer
      ↓
evaluator (flow)
      ↓   (Gate 5: flow evaluated, structural detectors verified)
onnx_integrator
      ↓   (Gate 6: manifests + checksums valid)
offline_verifier
          (Gate 7: offline inference passes)
```

## Hard rules (apply to every agent)

1. Never modify the source datasets in `ml-lab/datasets/`.
2. Never overwrite another agent's authoritative artifacts.
3. Never skip a failed gate — stop and report instead.
4. Never fabricate metrics, scores, confidences, or ONNX files.
5. Preserve a single feature contract: `feature-schema-v1`.
6. Record exact training configuration in a `training_manifest.json`.
7. Every stage must be reproducible from recorded config + seed.
8. Leakage columns are never model inputs (see `agents/feature_engineer.md`).
9. The optional local LLM is never part of training and never produces a score.

## Artifact ownership map

| Agent | Owns (writes) |
|-------|---------------|
| dataset_auditor | `ml-lab/evaluation/dataset_audit.{json,md}` |
| feature_engineer | `ml-lab/evaluation/feature_schema_v1.json`, `feature_definitions.md`, `feature_golden.json`, split manifests |
| anomaly_trainer | `ml-lab/training/anomaly/*`, `ml-lab/models/anomaly/{model.onnx,manifest.json,*.joblib}` |
| entity_trainer | `ml-lab/training/entity/*`, `ml-lab/models/entity/*` |
| flow_trainer | `ml-lab/training/flow/*`, `ml-lab/models/flow/*` |
| evaluator | `ml-lab/evaluation/{anomaly,entity,flow}/*` |
| onnx_integrator | `ml-lab/export/*`, ONNX parity reports, checksum validation |
| offline_verifier | `ml-lab/evaluation/offline_proof.{json,md}` |
| orchestrator | gate status, `ml-lab/evaluation/final_report.md` |

## How these map to code

The agent specs are realized as Python modules under `ml_lab/` (a real,
importable package) with CLI entry points, e.g.:

```
.venv/bin/python -m ml_lab.audit.datasets
.venv/bin/python -m ml_lab.training.anomaly.train
.venv/bin/python -m ml_lab.evaluation.anomaly
```

See `prompts/MASTER_TRAINING_PROMPT.md` for the single controlling brief.
