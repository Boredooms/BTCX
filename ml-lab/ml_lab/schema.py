"""feature-schema-v1 — the single feature contract.

This module is the Python source of truth for the 25-feature vector. The Go
production engine must compute identical names, order, units and formulas.
The ordered tuple below is authoritative; everything else derives from it.
"""

from __future__ import annotations

import hashlib
import json

FEATURE_SCHEMA_VERSION = "feature-schema-v1"

# Fixed order. Must match ml-lab/generate.go and the future Go feature engine.
FEATURES: tuple[str, ...] = (
    "tx_count",
    "incoming_count",
    "outgoing_count",
    "incoming_volume",
    "outgoing_volume",
    "avg_amount",
    "amount_variance",
    "fee_mean",
    "tx_per_hour",
    "median_time_gap",
    "burstiness",
    "velocity",
    "degree",
    "fan_in",
    "fan_out",
    "counterparty_diversity",
    "graph_depth",
    "hop_count",
    "chain_length",
    "value_decay",
    "split_ratio",
    "merge_ratio",
    "obs_count",
    "unique_ip_count",
    "unique_asn_count",
)

INT_FEATURES: frozenset[str] = frozenset({
    "tx_count", "incoming_count", "outgoing_count", "degree", "fan_in",
    "fan_out", "graph_depth", "hop_count", "chain_length", "obs_count",
    "unique_ip_count", "unique_asn_count",
})

# Columns that are labels/metadata and must NEVER be model inputs.
LEAKAGE_COLUMNS: frozenset[str] = frozenset({
    "ground_truth_anomaly", "true_entity_id", "same_entity_ground_truth",
    "pattern_label", "suspicious_ground_truth", "scenario", "entity_type",
    "cluster_size", "relationship_basis", "common_input_link_score",
    "subject_id", "wallet_id", "flow_id", "split",
})


def schema_hash() -> str:
    """Deterministic hash of the ordered feature names + version."""
    payload = json.dumps(
        {"version": FEATURE_SCHEMA_VERSION, "features": list(FEATURES)},
        sort_keys=True,
    ).encode()
    return hashlib.sha256(payload).hexdigest()


def assert_no_leakage(columns: list[str]) -> None:
    """Raise if any model-input column is a known leakage column."""
    bad = [c for c in columns if c in LEAKAGE_COLUMNS]
    if bad:
        raise ValueError(f"leakage columns used as model inputs: {bad}")
