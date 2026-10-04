#!/usr/bin/env bash
# phase8_json_check.sh — validate the FEAT-001 artifact JSON parses.
set -u
cd /home/devara/btcx || exit 2
python3 - <<'PY'
import json
p = ".phase8-artifacts/task-phase8-tui/features/FEAT-001.json"
d = json.load(open(p))
print("JSON OK")
print("status =", d["status"])
print("findings length =", len(d["findings"]))
PY
