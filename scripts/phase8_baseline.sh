#!/usr/bin/env bash
# phase8_baseline.sh — Phase 8 baseline GREEN verification.
# Runs the mandatory build/test/vet/fmt gates and reports pass/fail per gate.
# Non-destructive; produces bin/bctx as a side effect of the build gate.
set -u

export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || { echo "FATAL: repo dir missing"; exit 2; }

line() { printf '\n========== %s ==========\n' "$1"; }

line "go version"
go version

line "go build ./..."
if go build ./...; then echo "BUILD: PASS"; else echo "BUILD: FAIL"; fi

line "go build -o bin/bctx ./apps/bctx"
if go build -o bin/bctx ./apps/bctx; then echo "BUILD-BIN: PASS"; else echo "BUILD-BIN: FAIL"; fi

line "go test ./... (excluding 'no test files')"
go test ./... 2>&1 | grep -vE 'no test files'
echo "TEST-EXIT: ${PIPESTATUS[0]}"

line "go test -race ./..."
go test -race ./... 2>&1 | tail -40
echo "RACE-EXIT: ${PIPESTATUS[0]}"

line "go vet ./..."
if go vet ./...; then echo "VET: PASS"; else echo "VET: FAIL"; fi

line "gofmt -s -l ."
fmtout="$(gofmt -s -l . 2>&1)"
if [ -z "$fmtout" ]; then echo "FMT: PASS (no files need formatting)"; else echo "FMT: FILES NEED FORMATTING:"; echo "$fmtout"; fi

line "DONE"
