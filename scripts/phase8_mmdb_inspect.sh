#!/usr/bin/env bash
# phase8_mmdb_inspect.sh — one-off inspection of maxminddb-golang's net usage.
# Helps FEAT-001 decide whether the "net" / "net/netip" imports flagged by the
# dep gate are offline-safe value types (IP parsing) or an actual dialer.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 2

MMDIR="$(go list -m -f '{{.Dir}}' github.com/oschwald/maxminddb-golang)"
echo "DIR=$MMDIR"

echo "--- non-test files importing net / net/netip / net/http / crypto/tls ---"
grep -rl -e '"net"' -e 'net/netip' -e 'net/http' -e 'crypto/tls' "$MMDIR" --include='*.go' | grep -v _test.go || echo NONE

echo "--- net.* symbols used in non-test source ---"
grep -rhoE 'net\.[A-Za-z]+' "$MMDIR" --include='*.go' | grep -v _test | sort -u || echo NONE

echo "--- netip.* symbols used in non-test source ---"
grep -rhoE 'netip\.[A-Za-z]+' "$MMDIR" --include='*.go' | grep -v _test | sort -u || echo NONE

echo "--- any Dial/Listen/DialContext/http client anywhere (incl tests) ---"
grep -rn -e 'net.Dial' -e 'net.Listen' -e 'DialContext' -e 'net/http' -e 'crypto/tls' "$MMDIR" --include='*.go' || echo NONE
