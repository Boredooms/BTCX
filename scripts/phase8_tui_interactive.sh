#!/usr/bin/env bash
# phase8_tui_interactive.sh — FEAT-005 committed interaction gate.
#
# Drives a scripted []tea.KeyMsg sequence through the real Root.Update and
# asserts the shipped interaction contract (FEAT-001..004): focus region
# transitions (Tab cycles Nav<->Body, Esc layering), SideNav navigation (cursor
# move + Enter, number jump), command-palette and global-search overlay
# open/choose/close, geo-map viewport pan/zoom/select/reset, and the HARD
# no-overflow invariant after EVERY keypress at a fixed 160x50 size.
#
# It is headless (no TTY) and deterministic: a seeded in-memory case, a frozen
# clock, and a throwaway HOME holding the synthetic GeoIP City fixture + the
# world-geometry asset — fully offline, no network, no real ~/.bctx. The gate
# is backed by the committed Go test TestInteractiveGate (tui/
# interactive_gate_test.go); this script just invokes it and surfaces the same
# `== SUMMARY: PASS=N FAIL=M ==` shape as the other five gate scripts.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 2

LOG="$(mktemp /tmp/bctx-tui-interactive.XXXXXX)"
trap 'rm -f "${LOG}"' EXIT

# Run the committed gate test, streaming its per-check PASS/FAIL lines.
go test ./tui -run TestInteractiveGate -v -count=1 >"${LOG}" 2>&1
RC=$?

# Surface the per-check lines (the test's own t.Log output carries a
# "file.go:NN:" prefix; the go-test "--- PASS/FAIL:" status lines are excluded).
grep -E '_test\.go:[0-9]+:[[:space:]]*(PASS|FAIL): ' "${LOG}" \
  | sed -E 's/^[[:space:]]*[^:]+:[0-9]+:[[:space:]]*//'

# Prefer the summary the Go gate prints; fall back to a derived one so the gate
# always ends with a single authoritative line even if the test crashed early.
SUMMARY="$(grep -oE '== SUMMARY: PASS=[0-9]+ FAIL=[0-9]+ ==' "${LOG}" | tail -1)"
if [[ -z "${SUMMARY}" ]]; then
  P="$(grep -cE '_test\.go:[0-9]+:[[:space:]]*PASS: ' "${LOG}")"
  F="$(grep -cE '_test\.go:[0-9]+:[[:space:]]*FAIL: ' "${LOG}")"
  SUMMARY="== SUMMARY: PASS=${P} FAIL=${F} =="
fi
echo "${SUMMARY}"

# The gate passes only when the Go test passed (RC==0) AND it reported FAIL=0.
if [[ "${RC}" -ne 0 ]] || ! grep -q 'FAIL=0 ==' <<<"${SUMMARY}"; then
  echo "GATE FAILED (go test rc=${RC})"
  exit 1
fi
