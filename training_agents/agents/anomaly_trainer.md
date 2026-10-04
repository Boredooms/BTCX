# Agent: anomaly_trainer

## Role
Train the production anomaly model.

## Method
- Load `anomaly_dataset.csv`, select the 25 `feature-schema-v1` columns only.
- Fit **only** on `split == train` AND `ground_truth_anomaly == 0` (normal only).
- Model: **Isolation Forest** (scikit-learn), fixed `random_state`.
- Preprocessing (if any, e.g. StandardScaler) fit on train only, then frozen.
- Transform the raw `decision_function` / `score_samples` into
  `anomaly_score ∈ [0,1]` with a documented, reproducible monotonic transform.
  Calibration statistics (e.g. train-score min/max or quantiles) are computed on
  the training population only and stored — never recomputed on test.

## Inputs
- `ml-lab/evaluation/feature_schema_v1.json`
- `ml-lab/datasets/anomaly_dataset.csv` (read-only)

## Outputs (authoritative)
- `ml-lab/training/anomaly/train.py` logic (via `ml_lab.training.anomaly.train`)
- `ml-lab/models/anomaly/model.joblib` (sklearn pipeline)
- `ml-lab/models/anomaly/model.onnx` (exported; owned jointly with onnx_integrator)
- `ml-lab/models/anomaly/calibration.json` (score transform metadata)
- `ml-lab/models/anomaly/training_manifest.json`

## Must not
- Use `ground_truth_anomaly`, `scenario`, or `split` as model features.
- Fit calibration on validation/test.
- Report accuracy as the primary metric (imbalanced problem).
