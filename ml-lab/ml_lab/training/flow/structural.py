"""Deterministic structural flow detectors.

These are NOT ML and NOT model inputs. They produce boolean/scored structural
evidence from flow features, mirroring the detector concepts the Go runtime
will implement. The production detector combines the ML score (flow model) with
this structural evidence; keeping them separate preserves auditability.

Each detector returns a score in [0,1] plus a short reason. Thresholds are
deterministic and documented.
"""

from __future__ import annotations

import pandas as pd


def peeling_chain(row) -> float:
    """Long chain, high value_decay retained, small splits — peeling-like."""
    s = 0.0
    if row["hop_count"] >= 4:
        s += 0.4
    if row["value_decay"] >= 0.7:
        s += 0.3
    if row["split_ratio"] <= 0.4:
        s += 0.3
    return min(s, 1.0)


def mixing_like(row) -> float:
    """Many inputs AND outputs, high entropy, near-balanced value."""
    s = 0.0
    if row["input_count"] >= 6 and row["output_count"] >= 6:
        s += 0.5
    if row["amount_entropy"] >= 0.7:
        s += 0.3
    if row["value_decay"] >= 0.9:
        s += 0.2
    return min(s, 1.0)


def fan_out(row) -> float:
    return min((1.0 if row["fan_out"] >= 10 else 0.0)
               + (0.4 if row["split_ratio"] >= 0.7 else 0.0), 1.0)


def fan_in(row) -> float:
    return min((1.0 if row["fan_in"] >= 10 else 0.0)
               + (0.4 if row["merge_ratio"] >= 0.7 else 0.0), 1.0)


def rapid_flow(row) -> float:
    return min((0.6 if row["median_time_gap"] <= 10 else 0.0)
               + (0.4 if row["burstiness"] >= 0.2 else 0.0), 1.0)


DETECTORS = {
    "peeling_chain_like": peeling_chain,
    "mixing_like": mixing_like,
    "high_fan_out": fan_out,
    "high_fan_in": fan_in,
    "rapid_flow": rapid_flow,
}


def evidence_frame(df: pd.DataFrame) -> pd.DataFrame:
    """Compute all structural detector scores for a flow dataframe."""
    out = pd.DataFrame(index=df.index)
    for name, fn in DETECTORS.items():
        out[name] = df.apply(fn, axis=1)
    return out
