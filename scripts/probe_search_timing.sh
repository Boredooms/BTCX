#!/usr/bin/env bash
# Time the three Analysis-window lookups against the REAL demo-scratch case DB
# to find which query hangs. Uses the Go repo directly via a tiny test so the
# exact code path (indexes, scans) is exercised.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
go test ./storage/sqlite -run TestProbeSearchTiming -v 2>&1 | tail -20
