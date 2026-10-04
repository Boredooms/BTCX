#!/usr/bin/env bash
# Install/upgrade pinned packages into the venv, forcing IPv4 (broken IPv6 egress).
# Usage: pip_install.sh <pip args...>
set -euo pipefail
LAB_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IPV4_DIR="$(mktemp -d /tmp/bctx-ipv4-XXXXXX)"
cp "$LAB_DIR/scripts/force_ipv4.py" "$IPV4_DIR/sitecustomize.py"
export PYTHONPATH="$IPV4_DIR:${PYTHONPATH:-}"
trap 'rm -rf "$IPV4_DIR"' EXIT
"$LAB_DIR/.venv/bin/python" -m pip install "$@"
