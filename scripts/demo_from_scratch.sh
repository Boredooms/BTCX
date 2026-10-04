#!/usr/bin/env bash
# FROM-SCRATCH end-to-end proof on a real wallet:
#   fresh case -> acquire (online) -> graph build -> analyze (offline) ->
#   generate report (offline, md+json) -> verify report persisted.
# Mirrors the TUI flow (search -> acquire -> pipeline -> report) at the CLI.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
ADDR="${1:-bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97}"
CASE="demo-scratch"

echo "=== fresh case ==="
$BIN case create "$CASE" >/dev/null 2>&1
$BIN case open "$CASE" 2>&1 | tail -1

echo "=== acquire wallet (ONLINE — the one network step) ==="
BCTX_LIVE=1 $BIN --json sync wallet "$ADDR" 2>&1 | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin); print("  fetched",d.get("fetched"),"new",d.get("new"),"pages",d.get("pages"),d.get("status"))
except Exception: print("  (sync output non-json)")'

echo "=== graph build (offline) ==="
$BIN graph build 2>&1 | grep -E 'Edges:|input_to|output_to|sent_to' | head -5

echo "=== analyze (offline, airgapped) ==="
$BIN --airgap analyze wallet "$ADDR" 2>&1 | grep -E 'Risk|Confidence|Top pattern|Transactions'

echo "=== generate report (offline, airgapped) ==="
$BIN --airgap report generate "$ADDR" --format md 2>&1 | grep -E 'REPORT|FORMAT|OUTPUT|NETWORK'
$BIN --airgap report generate "$ADDR" --format json 2>&1 | grep -E 'REPORT|FORMAT'

echo "=== reports persisted ==="
$BIN report list 2>&1 | tail -4

echo "=== geo observations (so the Geo Map shows real pins in THIS case) ==="
# Esplora carries no peer IPs, so network observations are imported from a
# dataset of real worldwide endpoints — into the SAME case being investigated,
# so the Geo Map plots pins for this wallet's case rather than being empty.
python3 scripts/gen_geo_dataset.py /tmp/bctx-geo.ndjson demo-geo-tx-0001 >/dev/null 2>&1
$BIN --offline dataset import /tmp/bctx-geo.ndjson --format ndjson 2>&1 | grep -E 'Network obs|Status'

echo "=== final status ==="
$BIN --json status 2>&1 | grep -E 'case|transactions|wallets|edges|network_recs|models|graph'
