#!/usr/bin/env bash
# Finish the godmode demo: ensure config online, extract real wallets from the
# seed, run monitor sessions (fake provider) to generate alerts, analyze the
# hub, and print final state. Autonomous, offline (fake provider dials nothing).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx
BIN=./bin/bctx
CFG="$HOME/.config/bctx/config.toml"

echo "== ensure active case =="
$BIN case open demo-godmode 2>&1 | tail -1

echo "== config network block before =="
grep -A4 '\[network\]' "$CFG"

# Force online regardless of exact key spacing/order.
python3 - "$CFG" <<'PY'
import sys, re
p = sys.argv[1]
s = open(p).read()
s = re.sub(r'acquisition_enabled\s*=\s*false', 'acquisition_enabled = true', s)
s = re.sub(r'mode\s*=\s*"airgap"', 'mode = "online"', s)
s = re.sub(r'mode\s*=\s*"offline"', 'mode = "online"', s)
open(p, 'w').write(s)
print("config set online")
PY
echo "== config network block after =="
grep -A4 '\[network\]' "$CFG"

# Extract the fan-out hub input wallet and the consolidation sink from the seed.
python3 - > /tmp/wallets.txt <<'PY'
import json
d = json.load(open("scripts/bigseed.json"))
# fan-out tx is the one with 8 outputs; its single input is the "hub" feed.
hub = None; sink = None
for r in d:
    if len(r.get("outputs", [])) == 8 and len(r.get("inputs", [])) == 1:
        hub = r["inputs"][0]["address"]
    if len(r.get("inputs", [])) == 8 and len(r.get("outputs", [])) == 1:
        sink = r["outputs"][0]["address"]
print(hub or "")
print(sink or "")
PY
HUB=$(sed -n '1p' /tmp/wallets.txt)
SINK=$(sed -n '2p' /tmp/wallets.txt)
echo "hub=$HUB sink=$SINK"

echo "== monitor sessions (fake provider -> generates events/alerts) =="
for w in "$HUB" "$SINK"; do
  [ -z "$w" ] && continue
  echo "-- monitor $w --"
  $BIN monitor wallet "$w" --max-polls 3 2>&1 | tail -6
done

echo "== analyze hub =="
[ -n "$HUB" ] && $BIN analyze wallet "$HUB" 2>&1 | head -18

echo "== FINAL STATUS =="
$BIN --json status 2>&1

echo "== monitor sessions =="
$BIN monitor list 2>&1 | head -10
