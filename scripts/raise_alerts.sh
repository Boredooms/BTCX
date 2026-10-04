#!/usr/bin/env bash
# Analyze the high-value seed wallets with --alert so real alerts persist and
# the dashboard Alerts / Risk panels fill. Uses the demo-godmode case.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
BIN=./bin/bctx

$BIN case open demo-godmode >/dev/null 2>&1

# Collect a spread of subjects from the seed: hub, sink, and several chain/leaf
# wallets so multiple alerts of varying risk land.
python3 - > /tmp/subjects.txt <<'PY'
import json
d = json.load(open("scripts/bigseed.json"))
subs = []
for r in d:
    if len(r.get("outputs", [])) == 8:            # fan-out hub input
        subs.append(r["inputs"][0]["address"])
    if len(r.get("inputs", [])) == 8:             # consolidation sink
        subs.append(r["outputs"][0]["address"])
# a few peeling-chain keep wallets (interesting flow)
for r in d[:6]:
    for o in r.get("outputs", []):
        subs.append(o["address"])
# de-dup preserve order, cap at 10
seen=set(); out=[]
for s in subs:
    if s not in seen:
        seen.add(s); out.append(s)
print("\n".join(out[:10]))
PY

n=0
while IFS= read -r w; do
  [ -z "$w" ] && continue
  n=$((n+1))
  out=$($BIN analyze wallet "$w" --alert --alert-threshold 60 2>&1)
  risk=$(echo "$out" | grep -oE 'Risk:[[:space:]]+[0-9]+' | grep -oE '[0-9]+' | head -1)
  raised=$(echo "$out" | grep -c 'Alert raised')
  echo "subject $n: risk=${risk:-?} alert_raised=$raised"
done < /tmp/subjects.txt

echo "== alerts now in case =="
$BIN --json status 2>&1 | grep -E 'alerts|case'
