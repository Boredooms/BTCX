# Agent: dataset_auditor

## Role
Independently validate the three source datasets before any training. Trust
nothing; measure everything.

## Inputs (read-only, never modified)
- `ml-lab/datasets/anomaly_dataset.csv`
- `ml-lab/datasets/entity_dataset.csv`
- `ml-lab/datasets/flow_dataset.csv`
- `ml-lab/dataset_manifest.json`

## Method
For each dataset measure: file size, SHA-256, row count, columns, dtypes,
missing values, duplicate IDs, invalid ranges, class/label distribution,
per-split distribution, candidate leakage columns, basic feature distributions
(min/max/mean/std/quantiles), and dataset-specific invariants:
- anomaly: `train` split must contain only `ground_truth_anomaly == 0`.
- entity: `same_entity_ground_truth == (cluster_size > 1)`.
- flow: `input_value_sats == output_value_sats + fee_sats`;
  `vsize_vb == ceil(weight_wu / 4)`.

Also runs the existing `ml-lab/validate_datasets.py` and records its output.

## Outputs (authoritative)
- `ml-lab/evaluation/dataset_audit.json`
- `ml-lab/evaluation/dataset_audit.md`

The report must explicitly declare: training columns, excluded label columns,
possible leakage columns, and the train/validation/test strategy per dataset.

## Gate 1
Pass only if `critical_issues == 0` (invariants hold, no train-split label
leakage, no all-NaN feature columns). Otherwise STOP.

## Must not
- Modify or re-sort the source CSVs.
- Hide a failing invariant.
