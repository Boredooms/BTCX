#!/usr/bin/env bash
# Phase 8.5 GREEN baseline verification script.
# Runs build/test/vet/fmt and records pass/fail for each gate.
set -u

export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 99

sep() { echo ""; echo "===== $1 ====="; }

sep "go build ./..."
go build ./... 2>&1
echo "EXIT:go_build=$?"

sep "go build -o bin/bctx ./apps/bctx"
go build -o bin/bctx ./apps/bctx 2>&1
echo "EXIT:go_build_app=$?"

sep "go test ./... (excluding 'no test files')"
go test ./... 2>&1 | grep -vE 'no test files'
echo "EXIT:go_test=${PIPESTATUS[0]}"

sep "go test -race ./..."
go test -race ./... 2>&1 | grep -vE 'no test files'
echo "EXIT:go_test_race=${PIPESTATUS[0]}"

sep "go vet ./..."
go vet ./... 2>&1
echo "EXIT:go_vet=$?"

sep "gofmt -s -w . && gofmt -l . (must be empty)"
gofmt -s -w .
echo "---gofmt -l output (should be empty below)---"
gofmt -l .
echo "EXIT:gofmt_list_lines_above"

echo ""
echo "===== DONE ====="
