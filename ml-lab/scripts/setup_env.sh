#!/usr/bin/env bash
# Reproducible, sudo-free ML-lab Python environment bootstrap.
#
# Why this is not a plain `python3 -m venv`:
#   This machine lacks the python3-venv (ensurepip) apt package and we have no
#   sudo. We therefore create the venv WITHOUT pip and bootstrap pip into it
#   with the official get-pip.py. All downloads force IPv4 because this WSL
#   instance has a broken outbound IPv6 route (DNS returns AAAA, egress fails),
#   while IPv4 egress works.
#
# Idempotent: re-running reuses the existing venv.
set -euo pipefail

LAB_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VENV="$LAB_DIR/.venv"
PY=python3

# Force IPv4 for every network tool in this shell.
export PIP_INDEX_URL="${PIP_INDEX_URL:-https://pypi.org/simple}"
CURL_OPTS=(-4 -fsSL -m 120 --retry 3 --retry-delay 2)

# Make Python (pip/urllib) prefer IPv4 by installing force_ipv4 as sitecustomize
# on PYTHONPATH. The broken IPv6 route otherwise stalls all pip downloads.
IPV4_DIR="$(mktemp -d /tmp/bctx-ipv4-XXXXXX)"
cp "$LAB_DIR/scripts/force_ipv4.py" "$IPV4_DIR/sitecustomize.py"
export PYTHONPATH="$IPV4_DIR:${PYTHONPATH:-}"
trap 'rm -rf "$IPV4_DIR"' EXIT

echo "== ML lab env @ $LAB_DIR =="

if [ ! -x "$VENV/bin/python" ]; then
  echo "-- creating venv without pip --"
  "$PY" -m venv --without-pip "$VENV"
fi

VPY="$VENV/bin/python"

if ! "$VPY" -m pip --version >/dev/null 2>&1; then
  echo "-- bootstrapping pip via get-pip.py (IPv4) --"
  TMP_GETPIP="$(mktemp /tmp/get-pip-XXXXXX.py)"
  curl "${CURL_OPTS[@]}" https://bootstrap.pypa.io/get-pip.py -o "$TMP_GETPIP"
  "$VPY" "$TMP_GETPIP"
  rm -f "$TMP_GETPIP"
fi

echo "-- pip --"
"$VPY" -m pip --version

echo "-- installing pinned requirements (IPv4) --"
# -4 is passed to pip via the PIP flag below; also set a config so later calls inherit it.
"$VPY" -m pip config set global.timeout 120 >/dev/null 2>&1 || true
"$VPY" -m pip install --upgrade pip wheel setuptools
"$VPY" -m pip install -r "$LAB_DIR/requirements.lock.txt"

echo "-- verifying key imports --"
"$VPY" - <<'PY'
import numpy, pandas, sklearn, scipy, joblib, onnx, skl2onnx, onnxruntime
print("numpy", numpy.__version__)
print("pandas", pandas.__version__)
print("sklearn", sklearn.__version__)
print("scipy", scipy.__version__)
print("onnx", onnx.__version__)
print("skl2onnx", skl2onnx.__version__)
print("onnxruntime", onnxruntime.__version__)
print("ENV_OK")
PY
