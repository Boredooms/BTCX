#!/usr/bin/env bash
# build_mapdata.sh — RELEASE-TIME, OFFLINE world geometry asset builder
# (design §16, FEAT-003).
#
# Invokes the in-repo generator tools/mapgen to turn a LOCAL Natural Earth
# 1:110m admin-0 / coastline GeoJSON source (public domain) into the compact,
# deterministic asset world-110m.asset (coastline polylines + a derived
# representative-point centroid per country, both covered by one sha256).
#
# This runs ONCE at release time. It is NEVER run at runtime and tui/* never
# imports mapgen. It performs NO network access: it reads a local source file
# supplied by the release engineer and writes a local asset. The printed sha256
# is recorded in the release manifest / installer and verified by
# `bctx map install` / `bctx map verify`.
#
# Usage:
#   scripts/build_mapdata.sh <natural-earth-110m.geojson> [out.asset]
#
# After building, install with:
#   bctx map install <out.asset>
set -euo pipefail

SRC="${1:-}"
OUT="${2:-world-110m.asset}"

if [[ -z "${SRC}" ]]; then
  echo "usage: scripts/build_mapdata.sh <natural-earth-110m.geojson> [out.asset]" >&2
  exit 2
fi
if [[ ! -f "${SRC}" ]]; then
  echo "build_mapdata.sh: source not found: ${SRC}" >&2
  exit 1
fi

# Resolve the repo root so this works regardless of CWD.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "build_mapdata.sh: generating ${OUT} from ${SRC} (offline, deterministic)"
go run "${ROOT}/tools/mapgen" -in "${SRC}" -out "${OUT}"
echo "build_mapdata.sh: done — install with: bctx map install ${OUT}"
