#!/usr/bin/env bash
# Build a suite of demo cases spanning ML risk bands + a real entity cluster, so
# the TUI shows a spread of risk scores and a populated Entity Cluster screen.
# Fully offline (dataset import + local analysis); no network.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
DIR=/tmp/bctx-suite

# Fresh start: drop any prior suite cases so stale data never shadows the
# current generator output (a subject that kept old txs would score wrong).
for c in demo-low demo-elevated demo-high demo-cluster demo-mixing; do
  rm -rf "$HOME/.bctx/cases/$c"
done

python3 scripts/gen_demo_suite.py "$DIR" >/dev/null 2>&1

printf '%-16s %-10s %-9s %-9s %-5s %s\n' CASE RISK BAND CLUSTERS GEO SUBJECT
while read -r CASE FILE SUBJECT GEOFILE; do
  [ -z "$CASE" ] && continue
  $BIN case create "$CASE" >/dev/null 2>&1
  $BIN case open "$CASE" >/dev/null 2>&1
  $BIN --offline dataset import "$DIR/$FILE" --format ndjson >/dev/null 2>&1
  # Import this case's network observations too, so its Geo Map / Network
  # screens are populated with real worldwide pins (block/tx data carries no
  # peer IPs, so observations come from this offline telemetry dataset).
  if [ -n "${GEOFILE:-}" ] && [ -f "$DIR/$GEOFILE" ]; then
    $BIN --offline dataset import "$DIR/$GEOFILE" --format ndjson >/dev/null 2>&1
  fi
  $BIN graph build >/dev/null 2>&1
  # Raise an alert so the risk persists for the dashboard (prints a human line),
  # then read a clean JSON analysis for the summary table.
  $BIN --offline analyze wallet "$SUBJECT" --alert --alert-threshold 1 >/dev/null 2>&1
  $BIN --offline --json analyze wallet "$SUBJECT" >/tmp/bctx-suite-out.json 2>/dev/null
  $BIN --json status >/tmp/bctx-suite-status.json 2>/dev/null
  CASE="$CASE" SUBJECT="$SUBJECT" python3 - <<'PY'
import os, json
case, subj = os.environ["CASE"], os.environ["SUBJECT"]
try:
    d = json.load(open("/tmp/bctx-suite-out.json"))
    r = d["risk"]["score"]
    band = ("CRITICAL" if r >= 75 else "HIGH" if r >= 50 else "ELEVATED" if r >= 25 else "LOW")
    clusters = len(d.get("clusters") or [])
    try:
        geo = json.load(open("/tmp/bctx-suite-status.json")).get("network_recs", 0)
    except Exception:
        geo = "?"
    print("%-16s %-10s %-9s %-9d %-5s %s" % (case, "%d/100" % r, band, clusters, str(geo), subj[:28]))
except Exception as e:
    print("%-16s (analyze failed: %s)" % (case, e))
PY
done < "$DIR/subjects.txt"
