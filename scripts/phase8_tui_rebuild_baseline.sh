#!/usr/bin/env bash
# phase8_tui_rebuild_baseline.sh
# Idempotent, read-only baseline capture for the TUI interactive rebuild.
# Records the GREEN baseline that later work must not regress.
# Does NOT modify any source file.

set -u

export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 1

ART=/home/devara/btcx/.phase8-artifacts/tui-interactive-rebuild
mkdir -p "$ART"
LOG="$ART/baseline-raw.log"
: > "$LOG"

run() {
  # run "<label>" <command...>
  local label="$1"; shift
  echo "============================================================" >>"$LOG"
  echo "### $label" >>"$LOG"
  echo "\$ $*" >>"$LOG"
  echo "------------------------------------------------------------" >>"$LOG"
  "$@" >>"$LOG" 2>&1
  local rc=$?
  echo "" >>"$LOG"
  echo "[exit=$rc] $label" >>"$LOG"
  echo "RESULT|$label|$rc" >>"$LOG"
  return $rc
}

echo "===== BASELINE RUN $(date -u +%Y-%m-%dT%H:%M:%SZ) =====" >>"$LOG"

run "go build ./..." go build ./...
run "go build -o bin/bctx ./apps/bctx" go build -o bin/bctx ./apps/bctx
run "go vet ./..." go vet ./...

# gofmt -l . : expect empty output. Capture it specially so we can count lines.
echo "============================================================" >>"$LOG"
echo "### gofmt -l ." >>"$LOG"
echo "\$ gofmt -l ." >>"$LOG"
echo "------------------------------------------------------------" >>"$LOG"
GOFMT_OUT="$(gofmt -l . 2>&1)"
echo "$GOFMT_OUT" >>"$LOG"
GOFMT_COUNT=$(printf '%s' "$GOFMT_OUT" | grep -c '[^[:space:]]')
echo "" >>"$LOG"
echo "[gofmt_unformatted_files=$GOFMT_COUNT]" >>"$LOG"
if [ "$GOFMT_COUNT" -eq 0 ]; then
  echo "RESULT|gofmt -l . (expect empty)|0" >>"$LOG"
else
  echo "RESULT|gofmt -l . (expect empty)|1" >>"$LOG"
fi

# go test, filtering out the 'no test files' noise lines
echo "============================================================" >>"$LOG"
echo "### go test ./... (filtered)" >>"$LOG"
echo "\$ go test ./... 2>&1 | grep -vE 'no test files'" >>"$LOG"
echo "------------------------------------------------------------" >>"$LOG"
go test ./... 2>&1 | grep -vE 'no test files' >>"$LOG"
TEST_RC=${PIPESTATUS[0]}
echo "" >>"$LOG"
echo "[exit=$TEST_RC] go test ./..." >>"$LOG"
echo "RESULT|go test ./...|$TEST_RC" >>"$LOG"

# Gate scripts (run only if present)
for g in \
  scripts/phase8_tui_e2e.sh \
  scripts/phase8_airgap_tui.sh \
  scripts/phase7_offline_e2e.sh \
  scripts/phase7_airgap_bundle.sh \
  scripts/phase6_offline_proof.sh
do
  if [ -f "$g" ]; then
    run "gate: $g" bash "$g"
  else
    echo "RESULT|gate: $g|MISSING" >>"$LOG"
  fi
done

echo "===== BASELINE RUN COMPLETE =====" >>"$LOG"

# Emit the machine-readable result summary to stdout for the caller.
echo "========== RESULT SUMMARY =========="
grep '^RESULT|' "$LOG"
echo "========== GATE PASS/FAIL COUNTS =========="
# Many gates print lines like 'PASS'/'FAIL' or 'N/M'. Surface any obvious counters.
grep -nE 'PASS|FAIL|passed|failed|[0-9]+/[0-9]+' "$LOG" | tail -n 120
