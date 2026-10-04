# BCTX Dataset Audit

Schema: `feature-schema-v1`  hash `d52d7e12ee047c207231b9c165fe64ad831e0d6ec120eea2f0e9cfede5a2b589`

## anomaly

- file: `anomaly_dataset.csv`
- size: 191,091,869 bytes
- sha256: `498bfaf8f283ac659a6ddc484abb25eaa955819c2e4aac37edc7d93af8e5ff2e`
- rows: 934,374
- columns: 29
- NaN cells: 0
- duplicate ids: 0
- split counts: {'train': 654063, 'test': 186874, 'validation': 93437}
- leakage columns present: ['ground_truth_anomaly', 'scenario', 'split', 'subject_id']
- training columns (25): ['tx_count', 'incoming_count', 'outgoing_count', 'incoming_volume', 'outgoing_volume', 'avg_amount', 'amount_variance', 'fee_mean', 'tx_per_hour', 'median_time_gap', 'burstiness', 'velocity', 'degree', 'fan_in', 'fan_out', 'counterparty_diversity', 'graph_depth', 'hop_count', 'chain_length', 'value_decay', 'split_ratio', 'merge_ratio', 'obs_count', 'unique_ip_count', 'unique_asn_count']
- invariants: OK

label distribution:

  - `ground_truth_anomaly`: {'0': 788418, '1': 145956}
  - `scenario`: {'normal': 788418, 'velocity_spike': 16337, 'large_value_jump': 16124, 'fanin_consolidation': 16361, 'micro_fragmentation': 16156, 'mixed_anomaly': 16320, 'deep_chain': 16241, 'rapid_burst': 16310, 'network_burst': 16024, 'fanout_splitting': 16083}

## entity

- file: `entity_dataset.csv`
- size: 189,599,045 bytes
- sha256: `c07047587d90e553a7d057d4fc47e4ac1ffe61bef6b875f34f2a2dd376021814`
- rows: 731,564
- columns: 33
- NaN cells: 0
- duplicate ids: 0
- split counts: {'fit': 512096, 'test': 146312, 'validation': 73156}
- leakage columns present: ['cluster_size', 'common_input_link_score', 'entity_type', 'relationship_basis', 'same_entity_ground_truth', 'split', 'true_entity_id', 'wallet_id']
- training columns (25): ['tx_count', 'incoming_count', 'outgoing_count', 'incoming_volume', 'outgoing_volume', 'avg_amount', 'amount_variance', 'fee_mean', 'tx_per_hour', 'median_time_gap', 'burstiness', 'velocity', 'degree', 'fan_in', 'fan_out', 'counterparty_diversity', 'graph_depth', 'hop_count', 'chain_length', 'value_decay', 'split_ratio', 'merge_ratio', 'obs_count', 'unique_ip_count', 'unique_asn_count']
- invariants: OK

label distribution:

  - `same_entity_ground_truth`: {'1': 529746, '0': 201818}
  - `entity_type`: {'ordinary': 417417, 'merchant': 87982, 'service': 72149, 'exchange_like': 66633, 'custodial_like': 43869, 'miner_like': 43514}

## flow

- file: `flow_dataset.csv`
- size: 216,600,569 bytes
- sha256: `9e08f9ca242ec2f8373e4cc94ac46419f76712e6bbb7b270417f47cef8a0e2db`
- rows: 861,843
- columns: 28
- NaN cells: 0
- duplicate ids: 0
- split counts: {'train': 603291, 'test': 172368, 'validation': 86184}
- leakage columns present: ['flow_id', 'pattern_label', 'split', 'subject_id', 'suspicious_ground_truth']
- training columns (10): ['median_time_gap', 'burstiness', 'velocity', 'fan_in', 'fan_out', 'hop_count', 'chain_length', 'value_decay', 'split_ratio', 'merge_ratio']
- MISSING schema features: ['tx_count', 'incoming_count', 'outgoing_count', 'incoming_volume', 'outgoing_volume', 'avg_amount', 'amount_variance', 'fee_mean', 'tx_per_hour', 'degree', 'counterparty_diversity', 'graph_depth', 'obs_count', 'unique_ip_count', 'unique_asn_count']
- invariants: OK

label distribution:

  - `pattern_label`: {'normal': 482667, 'peeling_chain': 103847, 'mixing_like': 86023, 'rapid_flow': 60150, 'fan_out': 51970, 'fan_in': 34283, 'layered_chain': 25980, 'burst_consolidation': 16923}
  - `suspicious_ground_truth`: {'0': 482667, '1': 379176}

## Verdict

critical_issues = **0**
GATE 1: **PASS**
