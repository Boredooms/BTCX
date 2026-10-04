#!/usr/bin/env bash
# Confirm network acquisition STILL fetches after the air-gapped-posture change
# (NET shows DISCONNECTED at rest, but an explicit acquire still goes online).
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

$BIN case create acquire-check >/dev/null 2>&1
$BIN case open acquire-check 2>&1 | tail -1

echo "=== provider list (should be esplora) ==="
$BIN provider list 2>&1 | tail -2

echo "=== live acquire a real tx (the one network moment) ==="
BCTX_LIVE=1 timeout 60 $BIN --json sync tx fb070dcdd26715c8dfd26ad4fbd4ff199764e86ffc179a1e3b53ceee3b64ae14 2>&1 | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
    print("  fetched=%s new=%s status=%s provider=%s" % (d.get("fetched"), d.get("new"), d.get("status"), d.get("provider")))
except Exception:
    print("  (non-json / error)")
'
echo "=== case status after acquire ==="
$BIN --json status 2>&1 | python3 -c 'import sys,json; d=json.load(sys.stdin); print("  tx=%s" % d.get("transactions"))'
