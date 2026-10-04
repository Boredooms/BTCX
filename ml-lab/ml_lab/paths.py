"""Canonical paths for the ML lab. All relative to the ml-lab directory."""

from __future__ import annotations

from pathlib import Path

# ml_lab/paths.py -> ml_lab -> ml-lab
LAB_DIR = Path(__file__).resolve().parent.parent
DATASETS = LAB_DIR / "datasets"
EVALUATION = LAB_DIR / "evaluation"
TRAINING = LAB_DIR / "training"
MODELS = LAB_DIR / "models"
EXPORT = LAB_DIR / "export"
ARTIFACTS = LAB_DIR / "artifacts"
OUTPUTS = LAB_DIR / "outputs"

ANOMALY_CSV = DATASETS / "anomaly_dataset.csv"
ENTITY_CSV = DATASETS / "entity_dataset.csv"
FLOW_CSV = DATASETS / "flow_dataset.csv"

DATASET_MANIFEST = LAB_DIR / "dataset_manifest.json"
FEATURE_SCHEMA = EVALUATION / "feature_schema_v1.json"
FEATURE_GOLDEN = EVALUATION / "feature_golden.json"


def ensure_dirs() -> None:
    for d in (EVALUATION, TRAINING, MODELS, EXPORT, ARTIFACTS, OUTPUTS):
        d.mkdir(parents=True, exist_ok=True)
