#!/usr/bin/env bash
# phase8_geo_map_smoke.sh — FEAT-003 end-to-end smoke for the offline geo/map
# asset lifecycle. Exercises status (not-installed degrade), install, status
# (installed), and verify for `bctx map`, fully offline, against a throwaway
# HOME so the real ~/.bctx is never touched. The GeoIP (.mmdb) path is covered
# by the Go unit tests against a built fixture; this script covers the CLI
# wiring and the map asset lifecycle against the committed fixture asset.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${ROOT}/bin/bctx"
FIX="${ROOT}/tui/mapdata/testdata/world-110m.asset"

TH="$(mktemp -d)"
trap 'rm -rf "${TH}"' EXIT
export HOME="${TH}"

PASS=0
FAIL=0
check() { # check <desc> <needle> <command...>
  local desc="$1" needle="$2"; shift 2
  local out; out="$("$@" 2>&1)"
  if grep -qF "${needle}" <<<"${out}"; then
    echo "PASS: ${desc}"; PASS=$((PASS+1))
  else
    echo "FAIL: ${desc}"; echo "----- output -----"; echo "${out}"; echo "------------------"; FAIL=$((FAIL+1))
  fi
}

check "map status before install degrades honestly" "WORLD GEOMETRY ASSET NOT INSTALLED" "${BIN}" map status
check "geo status before install degrades honestly" "GEOIP DATABASE NOT INSTALLED"       "${BIN}" geo status
check "map install records the asset"               "Installed world-110m"               "${BIN}" map install "${FIX}"
check "map status after install shows provenance"   "public domain"                      "${BIN}" map status
check "map verify of installed asset is OK"          "VERIFY OK"                          "${BIN}" map verify

echo "== SUMMARY: PASS=${PASS} FAIL=${FAIL} =="
[[ "${FAIL}" -eq 0 ]]
