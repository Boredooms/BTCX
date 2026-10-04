#!/usr/bin/env bash
# Seed a real demo case so every TUI box fills: transactions across wallets +
# worldwide network observations (real public IPs GeoLite2 resolves), then
# analyze a subject so risk/evidence/alerts populate. Fully offline/local.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx

BIN=./bin/bctx

echo "== build =="
go build -o bin/bctx ./apps/bctx || exit 1

echo "== create demo case =="
$BIN case create demo-live 2>&1 | tail -1

echo "== import seed dataset (transactions + network observations) =="
$BIN dataset import scripts/seed_demo.json --format json 2>&1 | tail -8

echo "== counts after import =="
$BIN --json status 2>&1 | head -40

echo "== analyze a subject so risk/evidence/alerts populate =="
$BIN analyze wallet bc1qdst0mixer00000000000000000000000000bb 2>&1 | head -25

echo "== dataset list =="
$BIN dataset list 2>&1 | head -10

echo "== done: launch with ./bin/bctx (case demo-live active) =="
