# Entity Clustering Evaluation

Algorithm: `agglomerative_d6.0`  (eval n=20,000)

Clusters are **inferred relationships**, never proof of ownership.

## Metrics (validation vs true_entity_id, scoring only)

- Adjusted Rand Index: **0.0000**
- Normalized Mutual Info: **0.7113**
- Silhouette: 0.056555288645298935
- Clusters found: 443
- Noise rate: 0.0000
- Largest clusters: [302, 253, 253, 239, 231, 228, 219, 213, 205, 203]

## Stability (reseeded subsamples)

- ARI runs: [0.0, -0.0, -0.0]
- mean=-0.0000 std=0.0000

## Common-input heuristic (separate deterministic signal)

- precision=1.0000 recall=1.0000 (threshold 0.5)
