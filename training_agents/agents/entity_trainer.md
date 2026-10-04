# Agent: entity_trainer

## Role
Train the entity-relationship (clustering) signal. This is INFERRED
relationship detection — never proof of real-world ownership.

## Method
- Load `entity_dataset.csv`, select behavioral + graph-neighborhood features
  from `feature-schema-v1`. Fit on `split == fit`.
- Compare baselines: **DBSCAN** and **Agglomerative Clustering**.
- Standardize features (fit on fit-split only, frozen).
- Output per wallet: `cluster_id`, `confidence`, `basis` signals.
- The deterministic common-input heuristic belongs in Go; here it is an
  additional evaluated signal, not a model input.

## Inputs
- `ml-lab/evaluation/feature_schema_v1.json`
- `ml-lab/datasets/entity_dataset.csv` (read-only)

## Outputs (authoritative)
- `ml-lab/models/entity/model.joblib` (clustering pipeline + config)
- `ml-lab/models/entity/manifest.json`
- `ml-lab/models/entity/training_manifest.json`

## ONNX decision
Clustering (DBSCAN/Agglomerative) is not a pure feed-forward transform and is
not naturally ONNX-exportable. If export is not valid:
- freeze the deterministic implementation + exact config/params,
- document in the manifest WHY it is not ONNX,
- do NOT create a fake `.onnx` file.
The Go runtime will reproduce the frozen clustering logic (or call it as a
deterministic routine), not run a bogus ONNX graph.

## Must not
- Use `true_entity_id`, `entity_type`, `cluster_size`,
  `same_entity_ground_truth`, `relationship_basis`,
  `common_input_link_score` as model inputs.
- Phrase clusters as proven ownership.
