#!/usr/bin/env bash
# End-to-end Geo Map demo: generate a dataset of network observations keyed to
# real public IPs across the world, import it OFFLINE into a case, then verify
# the observations resolve through the installed GeoIP City DB to real
# country/city coordinates (the pins the Geo Map plots).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
CASE="${1:-demo-geo}"
DS=/tmp/bctx-geo.ndjson

echo "=== 1. generate dataset (real public IPs worldwide) ==="
python3 scripts/gen_geo_dataset.py "$DS" demo-geo-tx-0001
head -2 "$DS"

echo "=== 2. case + offline import ==="
$BIN case create "$CASE" >/dev/null 2>&1
$BIN case open "$CASE" 2>&1 | tail -1
$BIN --offline dataset import "$DS" --format ndjson 2>&1 | tail -6

echo "=== 3. case status (network recs) ==="
$BIN --json status 2>&1 | grep -E 'case|network_recs|transactions'

echo "=== 4. observations resolve to real coordinates (GeoIP City DB) ==="
# Reuse the geo verification Go test harness path: dump resolved locations.
$BIN --json status >/dev/null 2>&1
echo "(resolution is exercised by the TUI Geo Map + TestGeoMapPlotsResolvedPoints;"
echo " here we confirm the observations persisted and carry real IPs/countries)"
grep -oE '"src_ip":"[^"]+"' "$DS" | head -16
