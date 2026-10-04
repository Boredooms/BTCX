# Anomaly Model Evaluation

Primary metric: **PR-AUC** (imbalanced problem; accuracy intentionally not headlined).

## Test set

- PR-AUC: **0.9504**
- Precision: 0.8774
- Recall: 0.9204
- F1: 0.8984
- False-positive rate: 0.1402
- Threshold (chosen on validation): 0.4203
- Confusion (test): {'tn': 76871, 'fp': 12534, 'fn': 7763, 'tp': 89706}
- n=186,874  anomalies=97,469

## Per-scenario recall (test)

| scenario | n | recall |
|----------|---|--------|
| deep_chain | 10904 | 0.844 |
| fanin_consolidation | 10991 | 0.912 |
| fanout_splitting | 10761 | 0.900 |
| large_value_jump | 10870 | 0.843 |
| micro_fragmentation | 10752 | 0.978 |
| mixed_anomaly | 10879 | 0.818 |
| network_burst | 10659 | 1.000 |
| rapid_burst | 10796 | 0.998 |
| velocity_spike | 10857 | 0.993 |

## Counts

- {'normal_train': 654063, 'val_total': 93437, 'val_anomaly': 48487, 'test_total': 186874, 'test_anomaly': 97469}
