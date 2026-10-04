# Agent: onnx_integrator

## Role
Export models to ONNX where valid, verify Python↔ONNX parity, and validate all
model manifests + checksums.

## Method
For each ONNX-eligible model (anomaly, flow):
- Convert the frozen sklearn pipeline with `skl2onnx` (opset recorded).
- Load the `.onnx` with ONNX Runtime (CPUExecutionProvider only).
- Run golden input vectors through both Python and ONNX Runtime.
- Assert agreement within a recorded tolerance (e.g. max abs diff ≤ 1e-5).
- Record `model_sha256` only AFTER the file is finalized.

For non-ONNX models (entity clustering): verify the frozen artifact loads and
its config hash matches the manifest; record why ONNX is not used.

## Outputs (authoritative)
- `ml-lab/export/<model>_parity.json` (tolerance, max diff, pass/fail)
- updates `model_sha256`, `feature_schema_sha256`, `training_dataset_sha256`
  in each `models/*/manifest.json`

## Gate 6
Pass only if every manifest has valid, non-empty checksums and every ONNX model
passes parity within tolerance.

## Must not
- Create an ONNX file for a model that cannot be validly exported.
- Write a checksum before the artifact is final.
