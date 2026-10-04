"""Offline inference proof (Gate 7).

Run (ideally under a dropped network namespace):
    unshare -rn .venv/bin/python -m ml_lab.offline.verify

Loads every model artifact from local disk and runs inference on the golden
fixtures / a tiny local sample. Writes evaluation/offline_proof.{json,md}.

To make 'no network' explicit even without a namespace, this process installs a
socket guard that raises on any outbound connection attempt. The only resources
touched are local files.
"""

from __future__ import annotations

import socket
import sys
import time

import joblib
import numpy as np
import onnxruntime as ort

from ml_lab import paths, schema, util
from ml_lab.training.anomaly.train import raw_scores  # noqa: F401 (import sanity)


class NetworkBlocked(RuntimeError):
    pass


def _install_socket_guard():
    """Make any real outbound socket connection raise. Local AF_UNIX allowed."""
    real_connect = socket.socket.connect

    def guarded(self, address):  # noqa: ANN001
        fam = getattr(self, "family", None)
        if fam in (socket.AF_INET, socket.AF_INET6):
            raise NetworkBlocked(f"outbound network blocked: {address}")
        return real_connect(self, address)

    socket.socket.connect = guarded


def _check_anomaly():
    pipe = joblib.load(paths.MODELS / "anomaly" / "model.joblib")
    calib = util.read_json(paths.MODELS / "anomaly" / "calibration.json")
    golden = util.read_json(paths.FEATURE_GOLDEN)
    X = np.array([f["vector"] for f in golden["fixtures"]], dtype=np.float32)

    sess = ort.InferenceSession(str(paths.MODELS / "anomaly" / "model.onnx"),
                                providers=["CPUExecutionProvider"])
    outs = sess.run(None, {"input": X})
    scores = None
    for o in outs:
        a = np.asarray(o).reshape(len(X), -1)
        if a.shape[1] == 1:
            scores = a.ravel()
    anomaly_score = np.clip((-scores - calib["raw_lo"]) /
                            (calib["raw_hi"] - calib["raw_lo"]), 0, 1)
    return {"n": int(len(X)), "sample_scores": [float(s) for s in anomaly_score[:3]]}


def _check_flow():
    from ml_lab.training.flow.train import FLOW_FEATURES
    import pandas as pd

    sess = ort.InferenceSession(str(paths.MODELS / "flow" / "model.onnx"),
                                providers=["CPUExecutionProvider"])
    df = pd.read_csv(paths.FLOW_CSV, usecols=FLOW_FEATURES + ["split"],
                     nrows=200)
    X = df[FLOW_FEATURES].to_numpy(np.float32)[:5]
    outs = sess.run(None, {"input": X})
    classes = util.read_json(paths.MODELS / "flow" / "label_classes.json")["classes"]
    proba = None
    for o in outs:
        a = np.asarray(o)
        if a.ndim == 2 and a.shape[1] == len(classes):
            proba = a
    top = [classes[i] for i in proba.argmax(1)]
    return {"n": int(len(X)), "sample_predictions": top}


def _check_entity():
    bundle = joblib.load(paths.MODELS / "entity" / "model.joblib")
    return {"algorithm": bundle["algorithm"],
            "features": len(bundle["entity_features"]),
            "onnx": False,
            "note": "frozen deterministic clustering (no ONNX by design)"}


def main() -> None:
    _install_socket_guard()
    results = {}
    ok = True
    t0 = time.time()
    for name, fn in (("anomaly", _check_anomaly), ("flow", _check_flow),
                     ("entity", _check_entity)):
        try:
            results[name] = {"status": "ok", **fn()}
        except NetworkBlocked as e:
            results[name] = {"status": "network_attempt", "error": str(e)}
            ok = False
        except Exception as e:  # noqa: BLE001
            results[name] = {"status": "error", "error": repr(e)}
            ok = False

    proof = {
        "offline_pass": ok,
        "socket_guard": "AF_INET/AF_INET6 connect blocked for this process",
        "elapsed_sec": round(time.time() - t0, 3),
        "models": results,
        "schema_sha256": schema.schema_hash(),
        "env": util.env_block(),
    }
    paths.ensure_dirs()
    util.write_json(paths.EVALUATION / "offline_proof.json", proof)

    L = ["# Offline Inference Proof", "",
         f"offline_pass: **{ok}**",
         "Socket guard blocks all AF_INET/AF_INET6 connects for this process; "
         "only local files are read.", ""]
    for name, r in results.items():
        L.append(f"- **{name}**: {r}")
    L.append("")
    (paths.EVALUATION / "offline_proof.md").write_text("\n".join(L))

    print("offline_pass =", ok)
    for name, r in results.items():
        print(f"  {name}: {r.get('status')}")
    if not ok:
        sys.exit(1)


if __name__ == "__main__":
    main()
