# Flow Pattern Model Evaluation

Classes: ['burst_consolidation', 'fan_in', 'fan_out', 'layered_chain', 'mixing_like', 'normal', 'peeling_chain', 'rapid_flow']  (test n=172,368)

- accuracy: 0.9985
- macro F1: **0.9966**
- weighted F1: 0.9985
- macro one-vs-rest PR-AUC: 0.9980

Patterns are **matches**, not proof of criminal activity.

## Per-pattern metrics

| pattern | precision | recall | F1 | support | PR-AUC |
|---------|-----------|--------|----|---------|--------|
| burst_consolidation | 0.996 | 0.999 | 0.997 | 3363 | 0.999 |
| fan_in | 0.999 | 0.999 | 0.999 | 6820 | 1.000 |
| fan_out | 1.000 | 0.999 | 0.999 | 10474 | 1.000 |
| layered_chain | 0.987 | 0.971 | 0.979 | 5204 | 0.985 |
| mixing_like | 1.000 | 1.000 | 1.000 | 17256 | 1.000 |
| normal | 0.998 | 0.999 | 0.999 | 96585 | 1.000 |
| peeling_chain | 1.000 | 1.000 | 1.000 | 20597 | 1.000 |
| rapid_flow | 1.000 | 0.999 | 1.000 | 12069 | 1.000 |

Confusion matrix in `confusion.json`.
