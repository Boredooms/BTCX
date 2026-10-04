#!/usr/bin/env bash
# Render the risk-spread case's alert band distribution from the DB, as a quick
# non-TUI confirmation that the data behind the RISK DISTRIBUTION chart is real.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
$BIN case open risk-spread >/dev/null 2>&1
$BIN --json status > /tmp/rs-status.json 2>/dev/null
python3 - <<'PY'
import json
try:
    d = json.load(open("/tmp/rs-status.json"))
    print("risk-spread alerts in DB:", d.get("alerts"))
except Exception as e:
    print("status parse error:", e)
PY
echo "Open the TUI (btcx), case open risk-spread, press a (Alerts) to see the multi-colored RISK DISTRIBUTION chart."
