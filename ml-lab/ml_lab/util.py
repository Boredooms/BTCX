"""Shared utilities: hashing, JSON IO, reproducibility manifests."""

from __future__ import annotations

import hashlib
import json
import platform
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


def sha256_file(path: Path, chunk: int = 1 << 20) -> str:
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            b = f.read(chunk)
            if not b:
                break
            h.update(b)
    return h.hexdigest()


def write_json(path: Path, obj: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with open(path, "w") as f:
        json.dump(obj, f, indent=2, sort_keys=True, default=_default)
        f.write("\n")


def read_json(path: Path) -> Any:
    with open(path) as f:
        return json.load(f)


def _default(o: Any) -> Any:
    import numpy as np

    if isinstance(o, (np.integer,)):
        return int(o)
    if isinstance(o, (np.floating,)):
        return float(o)
    if isinstance(o, np.ndarray):
        return o.tolist()
    raise TypeError(f"not serializable: {type(o)}")


def git_commit() -> str:
    try:
        out = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            capture_output=True, text=True, timeout=5,
        )
        return out.stdout.strip() or "none"
    except Exception:
        return "none"


def now_iso() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def env_block() -> dict:
    """Reproducibility environment block for manifests."""
    import numpy
    import sklearn

    block = {
        "python": sys.version.split()[0],
        "os": f"{platform.system()} {platform.release()}",
        "numpy": numpy.__version__,
        "sklearn": sklearn.__version__,
        "git_commit": git_commit(),
        "timestamp": now_iso(),
    }
    try:
        import onnx
        import onnxruntime
        import skl2onnx

        block["onnx"] = onnx.__version__
        block["onnxruntime"] = onnxruntime.__version__
        block["skl2onnx"] = skl2onnx.__version__
    except Exception:
        pass
    return block
