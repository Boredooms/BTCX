#!/usr/bin/env bash
# Phase 8.5 audit-step baseline gate runner.
# Captures build/test/vet/gofmt + the six regression gate scripts.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 99

OUT=.phase85-artifacts/baseline-run.txt
: > "$OUT"

run() {
  local label="$1"; shift
  echo "===== BEGIN: $label =====" | tee -a "$OUT"
  "$@" >>"$OUT" 2>&1
  local rc=$?
  echo "----- EXIT($label)=$rc -----" | tee -a "$OUT"
  echo "" >>"$OUT"
  return $rc
}

echo "## go build ./..."
run "go build ./..." go build ./...

echo "## go build -o bin/bctx ./apps/bctx"
run "go build -o bin/bctx" go build -o bin/bctx ./apps/bctx

echo "## go test ./... (minus 'no test files')"
echo "===== BEGIN: go test ./... =====" | tee -a "$OUT"
go test ./... 2>&1 | grep -vE 'no test files' >>"$OUT" 2>&1
echo "----- EXIT(go test)=${PIPESTATUS[0]} -----" | tee -a "$OUT"
echo "" >>"$OUT"

echo "## go vet ./..."
run "go vet ./..." go vet ./...

echo "## gofmt -l ."
echo "===== BEGIN: gofmt -l . =====" | tee -a "$OUT"
GOFMT_OUT="$(gofmt -l . 2>&1)"
echo "$GOFMT_OUT" >>"$OUT"
if [ -z "$GOFMT_OUT" ]; then
  echo "----- gofmt -l EMPTY (clean) -----" | tee -a "$OUT"
else
  echo "----- gofmt -l NON-EMPTY (dirty) -----" | tee -a "$OUT"
fi
echo "" >>"$OUT"

echo "DONE baseline build/test. See $OUT"
