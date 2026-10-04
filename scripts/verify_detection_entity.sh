#!/usr/bin/env bash
# Verify Detection (patterns + model predictions) and Entity (clusters) render
# real signals from the analyzer on the real tx-e2e case wallet.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
ADDR=32aneueQWesQHetWba4xU7qfEFhhEYGNgP

$BIN case open tx-e2e >/dev/null 2>&1
$BIN --offline --json analyze wallet "$ADDR" > /tmp/analysis.json 2>/dev/null

python3 - <<'PY'
import json
d = json.load(open("/tmp/analysis.json"))
print("risk        :", d["risk"]["score"], "/100  confidence", d["risk"].get("confidence"))
print("patterns    :", [(p["type"], round(p["score"], 2)) for p in d.get("patterns", [])])
print("predictions :", [(p["model"], round(p["score"], 3)) for p in d.get("predictions", [])])
print("clusters    :", len(d.get("clusters") or []), "(0 is honest for a single tx)")
print("evidence    :", len(d.get("evidence") or []), "items")
PY
