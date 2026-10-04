"""Gate 1 — dataset auditor.

Run: python -m ml_lab.audit.datasets

Measures size/rows/dtypes/missing/dupes/ranges/distributions/leakage and the
dataset-specific invariants. Writes evaluation/dataset_audit.{json,md}. Trusts
nothing. Streams in chunks so the ~570 MiB corpus never loads fully into RAM.
"""

from __future__ import annotations

import math

import numpy as np
import pandas as pd

from ml_lab import paths, schema, util

CHUNK = 200_000


def _audit_one(path, id_col, label_cols, splits_col="split"):
    size = path.stat().st_size
    sha = util.sha256_file(path)

    rows = 0
    nan_total = 0
    columns: list[str] = []
    dtypes: dict[str, str] = {}
    split_counts: dict[str, int] = {}
    label_counts: dict[str, dict] = {lc: {} for lc in label_cols}
    # Streaming min/max/sum/sumsq for the schema features present.
    feat_present: list[str] = []
    stats: dict[str, dict] = {}
    seen_ids: set[str] = set()
    dup_ids = 0

    for ch in pd.read_csv(path, chunksize=CHUNK, low_memory=False):
        if not columns:
            columns = list(ch.columns)
            dtypes = {c: str(ch[c].dtype) for c in ch.columns}
            feat_present = [f for f in schema.FEATURES if f in ch.columns]
            stats = {f: {"min": math.inf, "max": -math.inf, "sum": 0.0, "sumsq": 0.0, "n": 0}
                     for f in feat_present}
        rows += len(ch)
        nan_total += int(ch.isna().sum().sum())

        if splits_col in ch.columns:
            vc = ch[splits_col].value_counts()
            for k, v in vc.items():
                split_counts[str(k)] = split_counts.get(str(k), 0) + int(v)

        for lc in label_cols:
            if lc in ch.columns:
                vc = ch[lc].value_counts(dropna=False)
                for k, v in vc.items():
                    key = str(k)
                    label_counts[lc][key] = label_counts[lc].get(key, 0) + int(v)

        for f in feat_present:
            col = pd.to_numeric(ch[f], errors="coerce").to_numpy(dtype=float)
            col = col[~np.isnan(col)]
            if col.size:
                s = stats[f]
                s["min"] = min(s["min"], float(col.min()))
                s["max"] = max(s["max"], float(col.max()))
                s["sum"] += float(col.sum())
                s["sumsq"] += float((col * col).sum())
                s["n"] += int(col.size)

        if id_col in ch.columns:
            # Duplicate detection by id (bounded memory: track a set; ids are
            # synthetic and unique by construction, so this stays small or
            # grows linearly — acceptable for the audit step).
            for v in ch[id_col].astype(str):
                if v in seen_ids:
                    dup_ids += 1
                else:
                    seen_ids.add(v)

    feat_stats = {}
    for f, s in stats.items():
        if s["n"] > 0:
            mean = s["sum"] / s["n"]
            var = max(s["sumsq"] / s["n"] - mean * mean, 0.0)
            feat_stats[f] = {
                "min": s["min"], "max": s["max"], "mean": mean,
                "std": math.sqrt(var), "n": s["n"],
            }

    return {
        "file": path.name,
        "size_bytes": size,
        "sha256": sha,
        "rows": rows,
        "columns": columns,
        "dtypes": dtypes,
        "nan_total": nan_total,
        "duplicate_ids": dup_ids,
        "split_counts": split_counts,
        "label_counts": label_counts,
        "feature_stats": feat_stats,
    }


def _invariants(path, name):
    """Dataset-specific invariant checks (streamed). Returns list of failures."""
    issues = []
    if name == "anomaly":
        bad = 0
        for ch in pd.read_csv(path, chunksize=CHUNK,
                              usecols=["split", "ground_truth_anomaly"]):
            bad += int(((ch["split"] == "train") & (ch["ground_truth_anomaly"] != 0)).sum())
        if bad:
            issues.append(f"anomaly train split has {bad} non-normal rows (label leakage)")
    elif name == "entity":
        bad = 0
        for ch in pd.read_csv(path, chunksize=CHUNK,
                              usecols=["cluster_size", "same_entity_ground_truth"]):
            bad += int((ch["same_entity_ground_truth"] != (ch["cluster_size"] > 1).astype(int)).sum())
        if bad:
            issues.append(f"entity ground-truth mismatch in {bad} rows")
    elif name == "flow":
        bad_cons = bad_vsize = 0
        for ch in pd.read_csv(path, chunksize=CHUNK,
                              usecols=["input_value_sats", "output_value_sats",
                                       "fee_sats", "weight_wu", "vsize_vb"]):
            bad_cons += int((ch["input_value_sats"] != ch["output_value_sats"] + ch["fee_sats"]).sum())
            bad_vsize += int((ch["vsize_vb"] != ((ch["weight_wu"] + 3) // 4)).sum())
        if bad_cons:
            issues.append(f"flow value-conservation failures: {bad_cons}")
        if bad_vsize:
            issues.append(f"flow BIP141 vsize failures: {bad_vsize}")
    return issues


def main() -> None:
    paths.ensure_dirs()

    specs = [
        (paths.ANOMALY_CSV, "anomaly", "subject_id", ["ground_truth_anomaly", "scenario"]),
        (paths.ENTITY_CSV, "entity", "wallet_id", ["same_entity_ground_truth", "entity_type"]),
        (paths.FLOW_CSV, "flow", "flow_id", ["pattern_label", "suspicious_ground_truth"]),
    ]

    results = {}
    all_issues = []
    for path, name, id_col, label_cols in specs:
        print(f"auditing {name} ...")
        a = _audit_one(path, id_col, label_cols)
        inv = _invariants(path, name)
        a["invariant_failures"] = inv
        # leakage columns actually present
        a["leakage_columns_present"] = sorted(
            set(a["columns"]) & schema.LEAKAGE_COLUMNS
        )
        # training columns = schema features present
        a["training_columns"] = [f for f in schema.FEATURES if f in a["columns"]]
        a["missing_schema_features"] = [f for f in schema.FEATURES if f not in a["columns"]]
        all_issues.extend(inv)
        if a["nan_total"] > 0:
            all_issues.append(f"{name}: {a['nan_total']} NaN cells")
        if a["duplicate_ids"] > 0:
            all_issues.append(f"{name}: {a['duplicate_ids']} duplicate ids")
        results[name] = a

    audit = {
        "schema_version": schema.FEATURE_SCHEMA_VERSION,
        "schema_sha256": schema.schema_hash(),
        "datasets": results,
        "critical_issues": len(all_issues),
        "issues": all_issues,
        "env": util.env_block(),
    }
    util.write_json(paths.EVALUATION / "dataset_audit.json", audit)
    _write_md(audit)

    print(f"\ncritical_issues = {audit['critical_issues']}")
    for i in all_issues:
        print(f"  - {i}")
    if audit["critical_issues"] == 0:
        print("GATE 1: PASS")
    else:
        print("GATE 1: FAIL — stop and investigate")


def _write_md(audit: dict) -> None:
    L = ["# BCTX Dataset Audit", "", f"Schema: `{audit['schema_version']}`  hash `{audit['schema_sha256']}`", ""]
    for name, a in audit["datasets"].items():
        L += [
            f"## {name}", "",
            f"- file: `{a['file']}`",
            f"- size: {a['size_bytes']:,} bytes",
            f"- sha256: `{a['sha256']}`",
            f"- rows: {a['rows']:,}",
            f"- columns: {len(a['columns'])}",
            f"- NaN cells: {a['nan_total']}",
            f"- duplicate ids: {a['duplicate_ids']}",
            f"- split counts: {a['split_counts']}",
            f"- leakage columns present: {a['leakage_columns_present']}",
            f"- training columns ({len(a['training_columns'])}): {a['training_columns']}",
        ]
        if a["missing_schema_features"]:
            L.append(f"- MISSING schema features: {a['missing_schema_features']}")
        if a["invariant_failures"]:
            L.append(f"- INVARIANT FAILURES: {a['invariant_failures']}")
        else:
            L.append("- invariants: OK")
        L += ["", "label distribution:", ""]
        for lc, counts in a["label_counts"].items():
            L.append(f"  - `{lc}`: {counts}")
        L.append("")
    L += ["## Verdict", "", f"critical_issues = **{audit['critical_issues']}**"]
    L.append("GATE 1: **PASS**" if audit["critical_issues"] == 0 else "GATE 1: **FAIL**")
    L.append("")
    (paths.EVALUATION / "dataset_audit.md").write_text("\n".join(L))


if __name__ == "__main__":
    main()
