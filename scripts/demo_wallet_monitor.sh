#!/usr/bin/env bash
# End-to-end demo on a REAL mainnet wallet:
#   1. create/open a case
#   2. acquire the wallet's history from Esplora (the one online step)
#   3. build the graph
#   4. analyze the wallet offline (risk + patterns + evidence)
#   5. run a bounded live monitor loop (patterns re-evaluated per poll)
#
# Usage: bash scripts/demo_wallet_monitor.sh [ADDRESS] [POLLS] [INTERVAL]
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

ADDR="${1:-bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97}"
POLLS="${2:-3}"
INTERVAL="${3:-15s}"
CASE="demo-monitor"

echo "=== 1. case ==="
$BIN case create "$CASE" >/dev/null 2>&1
$BIN case open "$CASE" 2>&1 | tail -1

echo "=== 2. acquire wallet history (online) ==="
BCTX_LIVE=1 $BIN --json sync wallet "$ADDR" 2>&1 | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin); print("fetched",d.get("fetched"),"new",d.get("new"),"pages",d.get("pages"),"status",d.get("status"))
except Exception as e: print("sync output:", sys.stdin.read()[:400])'

echo "=== 3. graph build ==="
$BIN graph build 2>&1 | tail -4

echo "=== 4. analyze (offline) ==="
$BIN --offline analyze wallet "$ADDR" 2>&1 | grep -E 'Risk|Confidence|Top pattern|Related|Transactions|HIGH|MED|LOW'

echo "=== 5. live monitor (bounded ${POLLS} polls @ ${INTERVAL}) ==="
BCTX_LIVE=1 $BIN monitor wallet "$ADDR" --interval "$INTERVAL" --max-polls "$POLLS" 2>&1 | tail -20

echo "=== sessions recorded ==="
$BIN monitor list 2>&1 | tail -6
echo "=== case status ==="
$BIN --json status 2>&1 | grep -E 'case|transactions|wallets|edges|alerts'
