#!/usr/bin/env bash
# Environment probe for the ML lab: reports which ML packages are already
# importable and whether pip can reach PyPI. No installs, no mutations.
set -uo pipefail

echo "=== python ==="
python3 --version

echo "=== importable packages ==="
for p in numpy pandas sklearn scipy joblib onnx skl2onnx onnxruntime matplotlib; do
  if out=$(python3 -c "import $p; print(getattr($p,'__version__','?'))" 2>/dev/null); then
    echo "OK: $p $out"
  else
    echo "MISSING: $p"
  fi
done

echo "=== pip ==="
python3 -m pip --version 2>&1 | head -1

echo "=== network reachability (pypi) ==="
curl -s -m 8 -o /dev/null -w "pypi_status=%{http_code}\n" https://pypi.org/simple/ 2>&1 || echo "no-net"
