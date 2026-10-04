#!/usr/bin/env bash
# Render key TUI screens against the active (seeded demo-live) case to prove
# the boxes fill with real data. Uses the plain non-TTY render paths + the
# geo inspect to confirm GeoIP resolves the seeded IPs worldwide.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx

BIN=./bin/bctx

echo "=================== GEOIP RESOLVES SEEDED IPs (worldwide) ==================="
for ip in 8.8.8.8 151.101.1.69 77.88.8.8 202.12.27.33 223.5.5.5 200.160.2.3 41.0.5.5 1.0.0.1; do
  printf '%-16s ' "$ip"
  $BIN geo inspect "$ip" 2>&1 | grep -E 'Country' | head -1
done

echo
echo "=================== MAP ASSET ==================="
$BIN map status 2>&1 | grep -E 'asset|centroids|sha256' | head -3

echo
echo "=================== NETWORK / IP (CLI json counts) ==================="
$BIN --json status 2>&1

echo
echo "=================== GEO MAP RENDER (model-level, seeded) ==================="
go test -run 'TestGeoMapPreview|TestShellGeoMapPreview' -v ./tui/ ./tui/screens/ 2>&1 | sed -n '1,70p'
