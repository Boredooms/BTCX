# Agent: offline_verifier

## Role
Prove the production inference path runs with no network.

## Method
Run the full inference chain under a dropped network namespace
(`unshare -rn`) using the venv:
- load `feature_schema_v1.json`
- load each model artifact (`model.onnx` via ONNX Runtime; entity frozen artifact)
- feed golden fixtures through preprocessing + inference
- parse outputs into the risk-input shape
Assert: no download, no cloud call, no external API, no package fetch — only
local files are read.

## Outputs (authoritative)
- `ml-lab/evaluation/offline_proof.json` (`offline_pass`, per-model result)
- `ml-lab/evaluation/offline_proof.md`

## Gate 7
Pass only if every model produces output offline and nothing attempts egress.

## Must not
- Fetch anything at runtime.
- Pass if any model required network.
