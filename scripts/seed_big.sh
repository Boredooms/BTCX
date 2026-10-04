#!/usr/bin/env bash
# Generate the big seed, import it into a fresh rich case, build graph, run a
# monitor session (fake provider, offline-deterministic) to generate alerts,
# analyze a high-value wallet, and print final counts. Autonomous end-to-end.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx

BIN=./bin/bctx
CFG="$HOME/.config/bctx/config.toml"

echo "== generate big seed =="
python3 scripts/gen_bigseed.py > scripts/bigseed.json
python3 - <<'PY'
import json
d = json.load(open("scripts/bigseed.json"))
obs = sum(len(r.get("network_observations", [])) for r in d)
print(f"generated: {len(d)} transactions, {obs} network observations")
PY

echo "== build =="
go build -o bin/bctx ./apps/bctx || exit 1

echo "== fresh case demo-godmode =="
$BIN case create demo-godmode 2>&1 | tail -1

echo "== import big seed =="
$BIN dataset import scripts/bigseed.json --format json 2>&1 | tail -8

echo "== flip config online so the fake provider can run a monitor session =="
# The fake provider needs no real network; set online so the offline gate allows it.
cp "$CFG" "$CFG.bak" 2>/dev/null || true
sed -i 's/^  acquisition_enabled = false/  acquisition_enabled = true/' "$CFG"
sed -i 's/^  mode = "airgap"/  mode = "online"/' "$CFG"
grep -E 'acquisition_enabled|mode' "$CFG" | head -2

echo "== run monitor sessions (fake provider) to generate alerts =="
for w in bc1qhub0 bc1qsink0; do
  pref=$(grep -o "\"address\": \"${w}[a-f0-9]*\"" scripts/bigseed.json | head -1 | sed 's/.*\"\(bc1q[a-f0-9]*\)\".*/\1/')
  if [ -n "$pref" ]; then
    echo "-- monitor $pref --"
    $BIN monitor wallet "$pref" --max-polls 3 2>&1 | tail -4
  fi
done

echo "== analyze the fan-out hub =="
HUB=$(grep -o '\"bc1qhub0[a-f0-9]*\"' scripts/bigseed.json | head -1 | tr -d '\"')
[ -n "$HUB" ] && $BIN analyze wallet "$HUB" 2>&1 | head -18

echo "== final counts =="
$BIN --json status 2>&1

echo "== alerts =="
$BIN monitor list 2>&1 | head -10
