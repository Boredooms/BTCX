#!/usr/bin/env python3
from __future__ import annotations
import argparse
from pathlib import Path
import pandas as pd

FEATURES = [
    "tx_count","incoming_count","outgoing_count","incoming_volume","outgoing_volume","avg_amount","amount_variance",
    "fee_mean","tx_per_hour","median_time_gap","burstiness","velocity","degree","fan_in","fan_out","counterparty_diversity",
    "graph_depth","hop_count","chain_length","value_decay","split_ratio","merge_ratio","obs_count","unique_ip_count","unique_asn_count"
]


def main():
    ap=argparse.ArgumentParser(); ap.add_argument('dir', nargs='?', default='.')
    args=ap.parse_args(); p=Path(args.dir)
    files=['anomaly_dataset.csv','entity_dataset.csv','flow_dataset.csv']
    for fn in files:
        path=p/fn
        if not path.exists(): raise SystemExit(f'missing {path}')
        rows=0
        nan=0
        for ch in pd.read_csv(path, chunksize=200000):
            rows += len(ch)
            nan += int(ch.isna().sum().sum())
        print(fn, 'rows=', rows, 'nan=', nan)

    # Anomaly: training split must be normal only.
    bad_train=0
    for ch in pd.read_csv(p/'anomaly_dataset.csv', chunksize=200000, usecols=['split','ground_truth_anomaly']):
        bad_train += int(((ch['split']=='train') & (ch['ground_truth_anomaly']!=0)).sum())
    print('anomaly_train_nonzero_labels=', bad_train)

    # Entity: same entity ground truth must agree with cluster size > 1.
    bad_entity=0
    for ch in pd.read_csv(p/'entity_dataset.csv', chunksize=200000, usecols=['cluster_size','same_entity_ground_truth']):
        bad_entity += int((ch['same_entity_ground_truth'] != (ch['cluster_size']>1).astype(int)).sum())
    print('entity_ground_truth_mismatch=', bad_entity)

    # Flow: exact value and BIP141 size invariants.
    bad_cons=0; bad_vsize=0
    for ch in pd.read_csv(p/'flow_dataset.csv', chunksize=200000, usecols=['input_value_sats','output_value_sats','fee_sats','weight_wu','vsize_vb']):
        bad_cons += int((ch['input_value_sats'] != ch['output_value_sats'] + ch['fee_sats']).sum())
        bad_vsize += int((ch['vsize_vb'] != ((ch['weight_wu']+3)//4)).sum())
    print('flow_conservation_failures=', bad_cons)
    print('flow_bip141_vsize_failures=', bad_vsize)

if __name__=='__main__': main()
