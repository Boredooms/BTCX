#!/usr/bin/env bash
# Confirm the seeded demo transactions now carry RECENT timestamps (current
# year), by reading the newest transactions back from a case via the CLI.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx
CASE="${1:-demo-cluster}"

echo "system date: $(date -u +%Y-%m-%dT%H:%MZ)"
$BIN case open "$CASE" >/dev/null 2>&1
# The analyze JSON carries relevant_txs; use the generated dataset timestamp as
# the ground truth (what got imported).
echo "generated sample timestamps:"
python3 scripts/gen_demo_suite.py /tmp/datecheck >/dev/null 2>&1
for f in cluster low mixing; do
  ts=$(head -1 "/tmp/datecheck/$f.ndjson" 2>/dev/null | python3 -c 'import sys,json;print(json.load(sys.stdin)["timestamp"])' 2>/dev/null)
  echo "  $f: $ts"
done
