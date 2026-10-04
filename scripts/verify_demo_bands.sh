#!/usr/bin/env bash
# Verify the three demo subjects produce DISTINCT risk bands (low / elevated /
# high) end-to-end through the real analyze pipeline. Prints the exact score and
# band for each so the demo is honest and reproducible.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 1

BIN=./bin/bctx

run_one() {
  local case_id="$1" subject="$2" label="$3"
  "$BIN" case open "$case_id" >/dev/null 2>&1
  echo "=== $label  case=$case_id  subject=$subject ==="
  "$BIN" --offline analyze wallet "$subject" 2>&1 | grep -iE 'Risk:|Confidence:|Transactions:|Signals:|score=|Top pattern:' | sed 's/^/  /'
  echo
}

run_one demo-low      "bc1qlowsubjectaaaaaaaaaaaaaaaaaaaaaaaaa0" "LOW"
run_one demo-elevated "bc1qpeelsubjectbbbbbbbbbbbbbbbbbbbbbbbb0" "ELEVATED/MEDIUM"
run_one demo-high     "bc1qfanoutsubjectcccccccccccccccccccc0"  "HIGH"
