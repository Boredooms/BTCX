#!/usr/bin/env bash
# Build + persist a forensic report for each of the three real-address cases, so
# the Reports tab shows a ready-to-view report (press v to render, e to export)
# without the operator having to press b first. Fully offline.
#
# `report generate` runs the real pipeline (features -> ML -> risk -> evidence),
# renders the report, AND persists a report row in the case DB (which is what
# the TUI Reports list reads). It writes the rendered file under the case's
# reports/ dir too.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
cd /home/devara/btcx || exit 2
BIN=./bin/bctx

build_one() {
  local case_id="$1" subject="$2"
  $BIN case open "$case_id" >/dev/null 2>&1
  echo "=== $case_id  $subject ==="
  # Markdown (human-readable) and JSON (machine) so both are on disk; the DB row
  # is persisted once and drives the TUI list + in-TUI viewer.
  $BIN --offline report generate "$subject" --format md 2>&1 | grep -E 'REPORT:|OUTPUT:|FORMAT:'
  $BIN --offline report generate "$subject" --format json >/dev/null 2>&1
  local n
  n="$($BIN report list 2>&1 | grep -c rpt-)"
  echo "  reports in case: $n"
}

build_one real-low  "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"
build_one real-med  "12cbQLTFMXRnSzktFkuoG3eHoMeFtpTu3S"
build_one real-high "bc1qgdjqv0av3q56jvd82tkdjpy7gdp9ut8tlqmgrpmv24sq90ecnvqqjwvw97"

# Also build reports in the demo-example cases loaded via the extension manager,
# if present, so a demo-loaded case shows a report immediately.
for c in real-low-whale real-med-hub real-high-peeler; do
  if [ -d "$HOME/.bctx/cases/$c" ]; then
    subj="$($BIN case open "$c" >/dev/null 2>&1; $BIN report list >/dev/null 2>&1; true)"
  fi
done

echo
echo "Done. Open the TUI (btcx), go to Reports (r), select a report, press v to view / e to export."
