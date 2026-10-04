"""Freeze feature-schema-v1 into authoritative artifacts.

Run: python -m ml_lab.features.freeze

Writes:
  evaluation/feature_schema_v1.json  — ordered names, dtypes, units, schema hash
  evaluation/feature_definitions.md  — human-readable definitions + formulas
  evaluation/feature_golden.json     — fixed input rows + expected vectors
"""

from __future__ import annotations

import pandas as pd

from ml_lab import paths, schema, util

# Per-feature metadata: unit, formula, missing/zero-denominator behavior.
DEFS: dict[str, dict] = {
    "tx_count": {"unit": "count", "formula": "incoming_count + outgoing_count", "missing": "0"},
    "incoming_count": {"unit": "count", "formula": "number of received txs", "missing": "0"},
    "outgoing_count": {"unit": "count", "formula": "number of sent txs", "missing": "0"},
    "incoming_volume": {"unit": "BTC", "formula": "sum of received amounts", "missing": "0"},
    "outgoing_volume": {"unit": "BTC", "formula": "sum of sent amounts", "missing": "0"},
    "avg_amount": {"unit": "BTC", "formula": "total_amount / tx_count; 0 if tx_count==0", "missing": "0"},
    "amount_variance": {"unit": "BTC^2", "formula": "(avg_amount*cv)^2", "missing": "0"},
    "fee_mean": {"unit": "BTC", "formula": "mean fee per tx", "missing": "0"},
    "tx_per_hour": {"unit": "1/hour", "formula": "tx_count / observed_hours; observed_hours>=eps", "missing": "0"},
    "median_time_gap": {"unit": "seconds", "formula": "median inter-tx gap; >=0.2", "missing": "0"},
    "burstiness": {"unit": "ratio", "formula": "(sigma-mu)/(sigma+mu) in [-1,1]", "missing": "0"},
    "velocity": {"unit": "BTC/second", "formula": "gross_flow_volume / active_seconds", "missing": "0"},
    "degree": {"unit": "count", "formula": "graph degree (fan_in+fan_out+noise)", "missing": "0"},
    "fan_in": {"unit": "count", "formula": "distinct input counterparties", "missing": "0"},
    "fan_out": {"unit": "count", "formula": "distinct output counterparties", "missing": "0"},
    "counterparty_diversity": {"unit": "ratio", "formula": "H/log(k), H=-sum(p_i log p_i) in [0,1]", "missing": "0"},
    "graph_depth": {"unit": "count", "formula": "BFS depth from subject", "missing": "1"},
    "hop_count": {"unit": "count", "formula": "flow hop count", "missing": "1"},
    "chain_length": {"unit": "count", "formula": "max(hop_count, chain samples)", "missing": "1"},
    "value_decay": {"unit": "ratio", "formula": "terminal_value/initial_value in (0,1]", "missing": "1.0"},
    "split_ratio": {"unit": "ratio", "formula": "1-max(output_share_i) in [0,1)", "missing": "0"},
    "merge_ratio": {"unit": "ratio", "formula": "1-max(input_share_i) in [0,1)", "missing": "0"},
    "obs_count": {"unit": "count", "formula": "network observations (0 if none)", "missing": "0"},
    "unique_ip_count": {"unit": "count", "formula": "distinct IPs (<=obs_count)", "missing": "0"},
    "unique_asn_count": {"unit": "count", "formula": "distinct ASNs (<=unique_ip_count)", "missing": "0"},
}

N_GOLDEN = 12


def build_schema_json() -> dict:
    cols = []
    for name in schema.FEATURES:
        cols.append({
            "name": name,
            "dtype": "int64" if name in schema.INT_FEATURES else "float64",
            "unit": DEFS[name]["unit"],
        })
    return {
        "version": schema.FEATURE_SCHEMA_VERSION,
        "count": len(schema.FEATURES),
        "order": list(schema.FEATURES),
        "columns": cols,
        "leakage_columns": sorted(schema.LEAKAGE_COLUMNS),
        "schema_sha256": schema.schema_hash(),
    }


def build_definitions_md() -> str:
    lines = [
        f"# feature-schema-v1 definitions",
        "",
        f"Schema hash: `{schema.schema_hash()}`",
        "",
        "The 25 features below are the single contract between the Python ML lab",
        "and the Go production feature engine. Order, names, units and formulas",
        "are authoritative. Missing / zero-denominator behavior is deterministic.",
        "",
        "| # | name | dtype | unit | formula | missing |",
        "|---|------|-------|------|---------|---------|",
    ]
    for i, name in enumerate(schema.FEATURES):
        d = DEFS[name]
        dtype = "int64" if name in schema.INT_FEATURES else "float64"
        lines.append(
            f"| {i} | `{name}` | {dtype} | {d['unit']} | {d['formula']} | {d['missing']} |"
        )
    lines.append("")
    return "\n".join(lines)


def build_golden() -> dict:
    """Deterministic golden fixtures: first N rows of the anomaly dataset,
    projected onto feature-schema-v1 in order. Used for Go/Python parity."""
    df = pd.read_csv(paths.ANOMALY_CSV, nrows=N_GOLDEN)
    rows = []
    for _, r in df.iterrows():
        vec = []
        for name in schema.FEATURES:
            val = r[name]
            vec.append(int(val) if name in schema.INT_FEATURES else float(val))
        rows.append({"subject_id": str(r.get("subject_id", "")), "vector": vec})
    return {
        "schema_version": schema.FEATURE_SCHEMA_VERSION,
        "schema_sha256": schema.schema_hash(),
        "feature_order": list(schema.FEATURES),
        "source": "anomaly_dataset.csv first rows",
        "fixtures": rows,
    }


def main() -> None:
    paths.ensure_dirs()
    sj = build_schema_json()
    util.write_json(paths.FEATURE_SCHEMA, sj)

    md = build_definitions_md()
    (paths.EVALUATION / "feature_definitions.md").write_text(md)

    golden = build_golden()
    util.write_json(paths.FEATURE_GOLDEN, golden)

    print(f"feature-schema-v1 frozen: {sj['count']} features")
    print(f"schema_sha256 = {sj['schema_sha256']}")
    print(f"wrote {paths.FEATURE_SCHEMA}")
    print(f"wrote {paths.EVALUATION / 'feature_definitions.md'}")
    print(f"wrote {paths.FEATURE_GOLDEN} ({len(golden['fixtures'])} fixtures)")


if __name__ == "__main__":
    main()
