# feature-schema-v1 definitions

Schema hash: `d52d7e12ee047c207231b9c165fe64ad831e0d6ec120eea2f0e9cfede5a2b589`

The 25 features below are the single contract between the Python ML lab
and the Go production feature engine. Order, names, units and formulas
are authoritative. Missing / zero-denominator behavior is deterministic.

| # | name | dtype | unit | formula | missing |
|---|------|-------|------|---------|---------|
| 0 | `tx_count` | int64 | count | incoming_count + outgoing_count | 0 |
| 1 | `incoming_count` | int64 | count | number of received txs | 0 |
| 2 | `outgoing_count` | int64 | count | number of sent txs | 0 |
| 3 | `incoming_volume` | float64 | BTC | sum of received amounts | 0 |
| 4 | `outgoing_volume` | float64 | BTC | sum of sent amounts | 0 |
| 5 | `avg_amount` | float64 | BTC | total_amount / tx_count; 0 if tx_count==0 | 0 |
| 6 | `amount_variance` | float64 | BTC^2 | (avg_amount*cv)^2 | 0 |
| 7 | `fee_mean` | float64 | BTC | mean fee per tx | 0 |
| 8 | `tx_per_hour` | float64 | 1/hour | tx_count / observed_hours; observed_hours>=eps | 0 |
| 9 | `median_time_gap` | float64 | seconds | median inter-tx gap; >=0.2 | 0 |
| 10 | `burstiness` | float64 | ratio | (sigma-mu)/(sigma+mu) in [-1,1] | 0 |
| 11 | `velocity` | float64 | BTC/second | gross_flow_volume / active_seconds | 0 |
| 12 | `degree` | int64 | count | graph degree (fan_in+fan_out+noise) | 0 |
| 13 | `fan_in` | int64 | count | distinct input counterparties | 0 |
| 14 | `fan_out` | int64 | count | distinct output counterparties | 0 |
| 15 | `counterparty_diversity` | float64 | ratio | H/log(k), H=-sum(p_i log p_i) in [0,1] | 0 |
| 16 | `graph_depth` | int64 | count | BFS depth from subject | 1 |
| 17 | `hop_count` | int64 | count | flow hop count | 1 |
| 18 | `chain_length` | int64 | count | max(hop_count, chain samples) | 1 |
| 19 | `value_decay` | float64 | ratio | terminal_value/initial_value in (0,1] | 1.0 |
| 20 | `split_ratio` | float64 | ratio | 1-max(output_share_i) in [0,1) | 0 |
| 21 | `merge_ratio` | float64 | ratio | 1-max(input_share_i) in [0,1) | 0 |
| 22 | `obs_count` | int64 | count | network observations (0 if none) | 0 |
| 23 | `unique_ip_count` | int64 | count | distinct IPs (<=obs_count) | 0 |
| 24 | `unique_asn_count` | int64 | count | distinct ASNs (<=unique_ip_count) | 0 |
