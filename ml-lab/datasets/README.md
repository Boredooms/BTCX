# BCTX ML Datasets

This directory contains three large, deterministic synthetic ML datasets designed for the BCTX Bitcoin investigation engine.

## Files

- `anomaly_dataset.csv` — ~183 MiB, 934,374 rows
- `entity_dataset.csv` — ~181 MiB, 731,564 rows
- `flow_dataset.csv` — ~207 MiB, 861,843 rows
- `dataset_manifest.json` — generation metadata, feature contract and formulas
- `generate.go` — deterministic generator used to produce the datasets
- `SHA256SUMS.txt` — integrity hashes

Total CSV size is ~570 MiB.

The datasets are **synthetic by design**. They are not scraped from real wallets and do not represent real-world illicit activity labels. Their purpose is to provide a reproducible training/evaluation corpus whose structure is grounded in Bitcoin's transaction/UTXO model and Bitcoin Core terminology.

---

# 1. Bitcoin Grounding

The generator is grounded in these protocol concepts:

- Bitcoin transactions spend previous transaction outputs (UTXOs) through transaction inputs and create new outputs.
- For a non-coinbase transaction, total input value must cover total output value; the difference can be paid as the transaction fee.
- Raw transactions expose fields such as `txid`, `hash`, `size`, `vsize`, `weight`, `blockhash`, `confirmations`, and output script information through Bitcoin Core's RPC interfaces.
- Under BIP 141, transaction weight is `3 * base_size + total_size` and virtual size is the weight divided by four rounded up.
- Bitcoin P2P peers exchange transaction and block inventories; unconfirmed transactions can be tracked through the mempool/network layer.

See the companion guide for the exact references and formulas used.

---

# 2. Shared Feature Contract

The three datasets use the same BCTX behavioral feature schema where applicable:

```text
tx_count
incoming_count
outgoing_count
incoming_volume
outgoing_volume
avg_amount
amount_variance
fee_mean
tx_per_hour
median_time_gap
burstiness
velocity
degree
fan_in
fan_out
counterparty_diversity
graph_depth
hop_count
chain_length
value_decay
split_ratio
merge_ratio
obs_count
unique_ip_count
unique_asn_count
```

This is `feature-schema-v1`.

The **order is fixed** and must match the Go feature engine used in BCTX production inference.

---

# 3. Dataset: Anomaly

File:

```text
anomaly_dataset.csv
```

Purpose:

> Learn a model of normal behavior and score unusual wallet behavior.

Training rows are intentionally normal-only:

```text
train       → normal only
validation  → normal + injected anomalies
test        → normal + injected anomalies
```

Scenarios include:

```text
normal
rapid_burst
fanout_splitting
fanin_consolidation
micro_fragmentation
velocity_spike
large_value_jump
deep_chain
network_burst
mixed_anomaly
```

`ground_truth_anomaly` is included only for validation/evaluation.

Recommended first production model:

```text
Isolation Forest
```

---

# 4. Dataset: Entity

File:

```text
entity_dataset.csv
```

Purpose:

> Group wallets that exhibit correlated behavior and graph characteristics.

Each row represents one synthetic wallet profile. Multiple rows can share a `true_entity_id`, producing realistic within-entity behavioral correlation.

Additional columns:

```text
true_entity_id
entity_type
cluster_size
common_input_link_score
same_entity_ground_truth
relationship_basis
```

These are evaluation/development metadata.

They should not be treated as production observations of real ownership.

Recommended MVP method:

```text
Go common-input heuristic
+
behavioral feature clustering
```

Potential scalable clustering choices include mini-batch or sampled approaches during experimentation. Graph embeddings can be added later.

---

# 5. Dataset: Flow

File:

```text
flow_dataset.csv
```

Purpose:

> Score transaction-flow structures using a supervised classifier plus deterministic structural detectors.

Patterns:

```text
normal
peeling_chain
mixing_like
rapid_flow
fan_out
fan_in
layered_chain
burst_consolidation
```

The following structural fields are included:

```text
input_count
output_count
participant_count
hop_count
chain_length
value_decay
split_ratio
merge_ratio
fan_in
fan_out
median_time_gap
burstiness
velocity
amount_entropy
```

Protocol-grounded transaction-size/value fields are also included:

```text
base_size_vb
total_size_vb
weight_wu
vsize_vb
feerate_sat_vb
input_value_sats
output_value_sats
fee_sats
```

For every row:

```text
input_value_sats = output_value_sats + fee_sats
```

and:

```text
weight_wu = 3 * base_size_vb + total_size_vb
vsize_vb = ceil(weight_wu / 4)
```

This makes the dataset mathematically checkable instead of simply assigning arbitrary fee/size values.

---

# 6. Mathematical Feature Definitions

## 6.1 Fee

For non-coinbase transactions:

```text
fee_sats = sum(input_sats) - sum(output_sats)
```

The generator uses integer satoshis for conservation checks.

---

## 6.2 Transaction Weight

Following BIP 141:

```text
weight = 3 * base_size + total_size
```

---

## 6.3 Virtual Size

```text
vsize = ceil(weight / 4)
```

---

## 6.4 Feerate

```text
feerate_sat_vb = fee_sats / vsize_vb
```

The generator samples a latent feerate and derives the fee from transaction size, subject to value-conservation limits.

---

## 6.5 Burstiness

For an inter-arrival distribution with mean `mu` and standard deviation `sigma`:

```text
B = (sigma - mu) / (sigma + mu)
```

Values closer to `1` indicate more bursty arrival behavior; values closer to `-1` indicate more regular spacing.

The synthetic generator uses a gamma-family latent process, for which the coefficient of variation is controlled by its shape parameter.

---

## 6.6 Counterparty Diversity

For normalized counterpart shares `p_i`:

```text
H = -sum(p_i * ln(p_i))
```

and normalized diversity:

```text
D = H / ln(k)
```

where `k` is the number of modeled counterpart categories.

This produces a bounded feature in approximately `[0,1]`.

---

## 6.7 Split Ratio

For output shares:

```text
split_ratio = 1 - max(output_share_i)
```

Interpretation:

- near `0`: one output dominates;
- larger values: value is distributed more evenly across outputs.

---

## 6.8 Merge Ratio

For input shares:

```text
merge_ratio = 1 - max(input_share_i)
```

Interpretation:

- near `0`: one input dominates;
- larger values: several inputs contribute materially to the flow.

These are BCTX analytical features, not Bitcoin consensus rules.

---

## 6.9 Value Decay

For a modeled flow:

```text
value_decay = terminal_value / initial_value
```

It is constrained to `(0, 1]` in the synthetic flow generator.

This is a BCTX flow-analysis feature, not a protocol-defined Bitcoin metric.

---

## 6.10 Velocity

```text
velocity = gross_flow_volume / active_seconds
```

This captures how quickly value moves through the modeled activity window.

---

# 7. Why These Are Better Than Random Synthetic Features

The generator does not sample every feature independently.

It creates latent behavioral variables first and derives dependent fields from them.

For example:

```text
activity intensity
      ↓
transaction count
      ↓
active duration
      ↓
transaction rate
      ↓
velocity
```

and:

```text
input/output structure
      ↓
fan-in / fan-out
      ↓
split / merge ratios
      ↓
graph degree
```

and:

```text
transaction size
      ↓
base/total size
      ↓
weight
      ↓
vsize
      ↓
feerate + fee
```

This produces correlated features that are more useful for ML than independent random columns.

---

# 8. Why We Do Not Scrape Real Wallets for Training Labels

A real blockchain can supply transaction structure, but it cannot automatically provide the ground-truth labels required for:

```text
normal
anomaly
peeling chain
mixing-like
same entity
```

Scraping random real wallets and declaring some to be malicious would create unverified labels and potentially contaminate training.

Instead, BCTX uses:

```text
Bitcoin protocol constraints
+
controlled latent behavior
+
synthetic ground truth
```

This is the correct foundation for a reproducible ML lab.

Real blockchain data should be used later for **external validation / robustness testing**, not as invented ground truth.

---

# 9. Training Strategy

## Anomaly

1. Select `split=train`.
2. Keep only `ground_truth_anomaly=0`.
3. Fit Isolation Forest.
4. Score validation/test.
5. Normalize the model output into `[0,1]`.
6. Measure precision, recall, F1, PR-AUC, and false-positive rate using the held-out ground truth.

## Entity

1. Fit the clustering method on `split=fit`.
2. Do not use `true_entity_id` as a feature.
3. Use `true_entity_id` only for evaluation.
4. Compare inferred clusters against ground truth with cluster-aware metrics.
5. Separately evaluate common-input heuristic precision/recall.

## Flow

1. Fit classifier on `split=train`.
2. Use `pattern_label` only as the target.
3. Do not include `suspicious_ground_truth` as an input feature.
4. Evaluate on validation/test.
5. Combine classifier score with deterministic detector evidence in Go.

---

# 10. Leakage Rules

The following columns must never become model inputs when they are the target/evaluation labels:

```text
ground_truth_anomaly
true_entity_id
same_entity_ground_truth
pattern_label
suspicious_ground_truth
```

The entity model must not receive the true entity identifier.

The flow model must not receive the pattern label.

The anomaly model must not receive the anomaly label.

---

# 11. Synthetic Identity Rules

Wallet IDs and TXIDs in these datasets are synthetic.

They are designed for machine learning and testing only.

They must not be interpreted as real Bitcoin addresses or real transaction identifiers.

For future raw network fixtures, use documentation/private address space rather than real user IP addresses.

---

# 12. Dataset Regeneration

The generator is deterministic.

Default seed:

```text
20261002
```

The Go generator can be rerun to reproduce the corpus.

Example:

```bash
go run generate.go \
  -out-dir ./datasets \
  -anomaly-mb 180 \
  -entity-mb 180 \
  -flow-mb 180 \
  -seed 20261002
```

To make a larger training corpus, increase the target sizes.

Example:

```bash
go run generate.go \
  -out-dir ./datasets-large \
  -anomaly-mb 500 \
  -entity-mb 500 \
  -flow-mb 500
```

The 500 MiB-per-file configuration is intentionally not the default because three 500 MiB CSVs produce a ~1.5 GiB corpus.

---

# 13. Recommended Model Training Architecture

```text
                  BCTX ML LAB
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
       ANOMALY       ENTITY       FLOW
          │            │            │
     Isolation      Cluster       Classifier
      Forest       + Heuristic       │
          │            │             │
          └────────────┼─────────────┘
                       ▼
                 Evaluation
                       │
                       ▼
                 ONNX Export
                       │
                       ▼
                  BCTX Go Runtime
```

Production runtime:

```text
Go
 ↓
feature-schema-v1
 ↓
ONNX Runtime
 ↓
local model.onnx
 ↓
prediction
```

---

# 14. Recommended Next Step

The next ML-lab implementation step should be:

```text
1. Implement the Go feature generator in BCTX.
2. Implement the Python feature reader using the exact schema.
3. Run a parity test on the same wallets/windows.
4. Train anomaly first.
5. Export to ONNX.
6. Load the ONNX model from Go.
7. Compare Python vs Go predictions.
8. Only then build the entity and flow models.
```

This prevents the most dangerous failure mode for the BCTX ML system: training a model on one feature definition and running it in production on a slightly different definition.

---

# 14. Protocol / Provider References Used for Grounding

The synthetic schema and constraints were grounded against current Bitcoin documentation and public explorer/API specifications during dataset design.

## Bitcoin Developer Documentation

- Transactions: https://developer.bitcoin.org/devguide/transactions.html
- Raw transaction reference: https://developer.bitcoin.org/reference/transactions.html
- `getrawtransaction`: https://developer.bitcoin.org/reference/rpc/getrawtransaction.html
- Block chain: https://developer.bitcoin.org/devguide/block_chain.html
- P2P network: https://developer.bitcoin.org/devguide/p2p_network.html

## Bitcoin Core / BIP

- BIP 141 (Segregated Witness): https://bips.dev/141/
- Bitcoin Core mempool policy/design references:
  - https://github.com/bitcoin/bitcoin/blob/master/doc/policy/mempool-design.md
  - https://github.com/bitcoin/bitcoin/blob/master/doc/policy/README.md

## Explorer / Live Data Interface References

- Blockstream Esplora API: https://github.com/Blockstream/esplora/blob/master/API.md
- Mempool.space WebSocket API: https://mempool.space/docs/api/websocket

The provider references are used to keep BCTX's acquisition adapter aligned with real-world data shapes and availability. The training files themselves remain synthetic and use no real wallet or person labels.
