#!/usr/bin/env bash
# Phase 8 cross-FEAT integration gate.
# Runs the FULL build/test/vet/fmt gate for the whole repo. Non-trivial logic
# lives here (not inline) because the PowerShell->WSL layer mangles nested
# quotes/$(). Prints a PASS/FAIL summary line per step.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2

pass=0
fail=0
step() {
  local name="$1"; shift
  echo "=== STEP: $name ==="
  if "$@"; then
    echo "STEP_RESULT: PASS - $name"
    pass=$((pass+1))
  else
    echo "STEP_RESULT: FAIL - $name"
    fail=$((fail+1))
  fi
}

# 1. go build ./...
step "go build ./..." bash -c 'go build ./...'

# 2. go build -o bin/bctx ./apps/bctx
step "go build bin/bctx" bash -c 'go build -o bin/bctx ./apps/bctx'

# 3. go test ./... (filter 'no test files' lines but keep real failures)
echo "=== STEP: go test ./... ==="
test_out=$(go test ./... 2>&1)
test_rc=$?
echo "$test_out" | grep -v 'no test files'
if [ $test_rc -eq 0 ]; then
  echo "STEP_RESULT: PASS - go test ./..."
  pass=$((pass+1))
else
  echo "STEP_RESULT: FAIL - go test ./..."
  fail=$((fail+1))
fi

# 4. go test -race ./...
echo "=== STEP: go test -race ./... ==="
race_out=$(go test -race ./... 2>&1)
race_rc=$?
echo "$race_out" | grep -v 'no test files'
if [ $race_rc -eq 0 ]; then
  echo "STEP_RESULT: PASS - go test -race ./..."
  pass=$((pass+1))
else
  echo "STEP_RESULT: FAIL - go test -race ./..."
  fail=$((fail+1))
fi

# 5. go vet ./...
step "go vet ./..." bash -c 'go vet ./...'

# 6. gofmt -s -w . && gofmt -l . (must be empty)
echo "=== STEP: gofmt -s -w . && gofmt -l . ==="
gofmt -s -w .
fmt_out=$(gofmt -l .)
if [ -z "$fmt_out" ]; then
  echo "STEP_RESULT: PASS - gofmt (clean)"
  pass=$((pass+1))
else
  echo "gofmt -l reported:"
  echo "$fmt_out"
  echo "STEP_RESULT: FAIL - gofmt"
  fail=$((fail+1))
fi

echo "INTEGRATION_GATE_SUMMARY: PASS=$pass FAIL=$fail"
[ $fail -eq 0 ]
