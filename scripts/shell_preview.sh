#!/usr/bin/env bash
# shell_preview.sh — print the FULL interactive shell for the four required
# scenes at 160x50 and 80x24 so a human can eyeball the cockpit, the command
# palette overlay, the Search screen with results, and the zoomed Geo Map.
#
# It drives the committed Preview tests (tui/shell_preview_test.go), which build
# a real Root over a seeded in-memory case with a FROZEN clock and a THROWAWAY
# HOME holding the synthetic GeoIP City fixture + the world-geometry fixture —
# fully offline, deterministic, no real ~/.bctx, no network. Mirrors
# scripts/geomap_preview.sh.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2

go test ./tui -run 'Preview' -v -count=1 2>&1 | sed -n '/\[SHELL —/,/^=== RUN\|^--- \|^ok /p'
