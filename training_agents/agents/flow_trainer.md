# Agent: flow_trainer

## Role
Train the suspicious-flow pattern classifier.

## Method
- Load `flow_dataset.csv`. Target = `pattern_label` only.
- Input features (flow-structural, NOT the leakage columns):
  `input_count, output_count, participant_count, hop_count, chain_length,
   value_decay, split_ratio, merge_ratio, fan_in, fan_out, median_time_gap,
   burstiness, velocity, amount_entropy` plus protocol size/value fields where
   they are legitimate inputs (`vsize_vb`, `feerate_sat_vb`). Decide the exact
   input set in `training_manifest.json` and keep it stable.
- Model: **Gradient Boosting classifier** (scikit-learn), fixed seed. Fit on
  `split == train`. Only compare a small MLP later if the baseline is weak.
- Keep **structural evidence** (deterministic detector outputs) SEPARATE from
  the ML score; the production detector in Go combines both.

## Inputs
- `ml-lab/datasets/flow_dataset.csv` (read-only)

## Outputs (authoritative)
- `ml-lab/models/flow/model.joblib`
- `ml-lab/models/flow/model.onnx` (with onnx_integrator)
- `ml-lab/models/flow/manifest.json`
- `ml-lab/models/flow/training_manifest.json`

## Patterns
`normal, peeling_chain, mixing_like, rapid_flow, fan_out, fan_in,
layered_chain, burst_consolidation`.

## Must not
- Use `pattern_label` or `suspicious_ground_truth` as input features.
- Describe a detected pattern as proof of criminal activity.
