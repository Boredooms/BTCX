#!/usr/bin/env bash
# Seed a dedicated 'risk-spread' demo case whose alerts span the risk bands, so
# the Alerts screen's RISK DISTRIBUTION chart shows a full multi-colored
# breakdown driven from the database. Fully offline.
#
# Approach: import several subject datasets (real + synthetic, each shaped to a
# different band) into ONE case, then analyze each subject with --alert so a
# real alert row is persisted at that subject's real risk score. The chart then
# buckets those alerts by band.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
REAL=/tmp/bctx-real
SUITE=/tmp/bctx-suite

# Regenerate the datasets so the files exist.
python3 scripts/gen_real_wallets.py "$REAL" >/dev/null 2>&1
python3 scripts/gen_demo_suite.py "$SUITE" >/dev/null 2>&1

rm -rf "$HOME/.bctx/cases/risk-spread"
$BIN case create risk-spread >/dev/null 2>&1
$BIN case open risk-spread >/dev/null 2>&1

import() { [ -f "$1" ] && $BIN --offline dataset import "$1" --format ndjson >/dev/null 2>&1; }

# Pull in a spread of subjects (each dataset shapes a different band) PLUS each
# subject's network-observation (geo) dataset, so the Geo Map / Network / Geo
# Activity screens are populated for this case too.
import "$REAL/real-low.ndjson";   import "$REAL/real-low-geo.ndjson"
import "$REAL/real-med.ndjson";   import "$REAL/real-med-geo.ndjson"
import "$REAL/real-high.ndjson";  import "$REAL/real-high-geo.ndjson"
import "$SUITE/low.ndjson";       import "$SUITE/demo-low-geo.ndjson"
import "$SUITE/elevated.ndjson";  import "$SUITE/demo-elevated-geo.ndjson"
import "$SUITE/high.ndjson";      import "$SUITE/demo-high-geo.ndjson"
import "$SUITE/cluster.ndjson";   import "$SUITE/demo-cluster-geo.ndjson"
import "$SUITE/mixing.ndjson";    import "$SUITE/demo-mixing-geo.ndjson"
$BIN graph build >/dev/null 2>&1

# Analyze each subject with --alert so a real alert row persists at its score.
alert() { $BIN --offline analyze wallet "$1" --alert --alert-threshold 1 >/dev/null 2>&1; }
alert "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"
alert "12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S"
alert "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"
alert "bc1qlowsubjectaaaaaaaaaaaaaaaaaaaaaaaaa0"
alert "bc1qpeelsubjectbbbbbbbbbbbbbbbbbbbbbbbb0"
alert "bc1qfanoutsubjectcccccccccccccccccccc0"
alert "bc1qclustersubjectdddddddddddddddddddd0"
alert "bc1qmixsubjecteeeeeeeeeeeeeeeeeeeeeeee0"

echo "=== risk-spread alerts (band distribution) ==="
$BIN --json status 2>/dev/null | python3 -c 'import sys,json; print("total alerts:", json.load(sys.stdin).get("alerts","?"))' 2>/dev/null || true
echo "Open: bctx case open risk-spread  -> Alerts tab (a) shows the RISK DISTRIBUTION chart"

# Populate geo + recent timestamps for every case so no screen is ever empty.
go build -o bin/geopop ./tools/geopop >/dev/null 2>&1 && ./bin/geopop --all >/dev/null 2>&1
echo "geo populated for all cases (geopop --all)"
