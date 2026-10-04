#!/usr/bin/env bash
# geomap_preview.sh — print the Geo Map View() at 160x50 to stdout so the
# plotted coastline + GeoIP-resolved markers can be eyeballed. It drives the
# committed TestGeoMapPreview, which seeds a few observations with resolvable
# IPs against a THROWAWAY HOME holding the synthetic City fixture + the world
# geometry fixture — fully offline, deterministic, no real ~/.bctx, no network.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2

go test -run TestGeoMapPreview -v ./tui/screens 2>&1 | sed -n '/GEO MAP/,/PASS/p'
