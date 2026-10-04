# Agent: evaluator

## Role
Independent evaluation of each trained model on held-out data. Produces honest
metrics; never trains.

## Anomaly evaluation
On validation + test rows (which contain injected anomalies):
- PR-AUC (primary), precision, recall, F1 at a chosen operating threshold
- false-positive rate, confusion matrix
- per-scenario recall (rapid_burst, fanout_splitting, ... mixed_anomaly)
- counts: normal train, normal val, anomaly test, class balance
Do NOT headline accuracy.

## Entity evaluation
Against `true_entity_id` used ONLY for scoring:
- Adjusted Rand Index, Normalized Mutual Information
- silhouette score, cluster stability (reseed/subsample), noise rate
- cluster-size distribution, same-entity cohesion vs different-entity separation
- separately: common-input heuristic precision/recall (deterministic signal)

## Flow evaluation
Against `pattern_label` used ONLY as target:
- per-pattern precision/recall/F1, macro/weighted averages
- full confusion matrix, PR-AUC where meaningful

## Outputs (authoritative)
- `ml-lab/evaluation/anomaly/{metrics.json,report.md,confusion.json}`
- `ml-lab/evaluation/entity/{metrics.json,report.md}`
- `ml-lab/evaluation/flow/{metrics.json,report.md,confusion.json}`

## Must not
- Compute metrics on the training split.
- Round away or omit a weak result.
