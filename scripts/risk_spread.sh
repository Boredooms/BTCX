#!/usr/bin/env bash
# Show the risk spread across the demo-godmode wallets after recalibration:
# a plain transfer should read LOW/MEDIUM, the peeling-chain hub HIGH/CRITICAL.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
go build -o bin/bctx ./apps/bctx || exit 1
BIN=./bin/bctx
$BIN case open demo-godmode >/dev/null 2>&1

# Pull a spread of subjects from the seed: fan-out hub input, a plain rnd
# transfer wallet, a peeling keep wallet, the consolidation sink.
python3 - > /tmp/spread_subjects.txt <<'PY'
import json
d = json.load(open("scripts/bigseed.json"))
subs = []
for r in d:
    if len(r.get("outputs", [])) == 8:                 # fan-out hub input
        subs.append(("hub-input", r["inputs"][0]["address"]))
    if len(r.get("inputs", [])) == 8:                  # consolidation sink
        subs.append(("sink", r["outputs"][0]["address"]))
# a plain 1-in-1-out "rnd" transfer wallet
for r in d:
    if len(r.get("inputs", []))==1 and len(r.get("outputs", []))==1:
        subs.append(("plain-transfer", r["inputs"][0]["address"])); break
# a peeling keep wallet
for r in d[:4]:
    for o in r.get("outputs", []):
        subs.append(("peel-keep", o["address"])); break
    break
for tag, a in subs[:6]:
    print(f"{tag}\t{a}")
PY

printf '%-16s %-20s %-6s %s\n' "ROLE" "SUBJECT" "RISK" "BAND"
while IFS=$'\t' read -r tag addr; do
  [ -z "$addr" ] && continue
  out=$($BIN --json analyze wallet "$addr" 2>&1)
  risk=$(echo "$out" | grep -o '\"score\": [0-9]*' | head -1 | grep -o '[0-9]*')
  band=$(echo "$out" | grep -o '\"band\": \"[A-Z]*\"' | head -1 | grep -o '[A-Z]*')
  printf '%-16s %-20s %-6s %s\n' "$tag" "$(echo $addr | cut -c1-18)" "${risk:-?}" "${band:-?}"
done < /tmp/spread_subjects.txt
