# Agent: feature_engineer

## Role
Own and freeze `feature-schema-v1` — the single feature contract shared by
Python training and the future Go production engine.

## The 25 features (fixed order)
```
tx_count, incoming_count, outgoing_count, incoming_volume, outgoing_volume,
avg_amount, amount_variance, fee_mean, tx_per_hour, median_time_gap,
burstiness, velocity, degree, fan_in, fan_out, counterparty_diversity,
graph_depth, hop_count, chain_length, value_decay, split_ratio, merge_ratio,
obs_count, unique_ip_count, unique_asn_count
```

## Method
For each feature document: name, datatype, unit, mathematical definition,
source fields, missing-value behavior, zero-denominator behavior,
transformation, version. Define formulas explicitly, e.g.:
- `tx_per_hour = tx_count / observed_hours` (observed_hours ≥ epsilon)
- `avg_amount = total_amount / tx_count` (0 if tx_count == 0)
- `value_decay = terminal_value / initial_value` (clamped to (0,1])

Deterministic missing/zero-denominator rules so Go can reproduce them exactly.

Build golden fixtures: a small set of fixed input rows with their expected
feature vectors, used later for Go/Python parity.

## Outputs (authoritative)
- `ml-lab/evaluation/feature_schema_v1.json` (names, order, dtypes, schema hash)
- `ml-lab/evaluation/feature_definitions.md` (human-readable definitions)
- `ml-lab/evaluation/feature_golden.json` (fixtures + expected vectors)

## Gate 2
Pass only if the schema JSON lists exactly 25 features in the fixed order,
every feature has a complete definition, and golden fixtures load and validate.

## Must not
- Invent a feature the Go engine cannot reproduce.
- Change feature order or names after freezing.
