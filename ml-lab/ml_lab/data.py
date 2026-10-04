"""Dataset loading + split provenance.

The source datasets already carry a deterministic `split` column produced by the
seeded generator (seed 20261002), which splits entity/scenario-aware to avoid
leakage. We therefore CONSUME the embedded splits rather than re-shuffling
(re-splitting correlated rows ourselves would risk the very leakage the
generator already prevents). This module records the split provenance so the
choice is auditable and reproducible.
"""

from __future__ import annotations

import numpy as np
import pandas as pd

from ml_lab import paths, schema, util

SEED = 20261002


def load_features(path, extra_cols: list[str] | None = None) -> pd.DataFrame:
    """Load only the schema features (+ requested extra/label columns)."""
    header = pd.read_csv(path, nrows=0)
    present = [f for f in schema.FEATURES if f in header.columns]
    cols = list(present)
    for c in (extra_cols or []):
        if c in header.columns and c not in cols:
            cols.append(c)
    df = pd.read_csv(path, usecols=cols, low_memory=False)
    # Deterministic ordering of feature columns.
    ordered = [f for f in schema.FEATURES if f in df.columns]
    rest = [c for c in df.columns if c not in ordered]
    return df[ordered + rest]


def feature_matrix(df: pd.DataFrame) -> np.ndarray:
    cols = [f for f in schema.FEATURES if f in df.columns]
    schema.assert_no_leakage(cols)
    return df[cols].to_numpy(dtype=np.float64)


def record_split_manifest(name: str, path, split_counts: dict) -> dict:
    manifest = {
        "dataset": name,
        "source_file": path.name,
        "source_sha256": util.sha256_file(path),
        "seed": SEED,
        "split_strategy": "embedded deterministic split column from seeded "
                          "generator (entity/scenario-aware); consumed as-is",
        "split_counts": split_counts,
        "schema_version": schema.FEATURE_SCHEMA_VERSION,
    }
    util.write_json(paths.EVALUATION / f"split_{name}.json", manifest)
    return manifest
