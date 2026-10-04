#!/usr/bin/env bash
# Phase 8.5 audit-step regression gate runner.
# Runs the six gate scripts and records exit code + the pass/fail summary line.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx || exit 99

OUT=.phase85-artifacts/gates-run.txt
: > "$OUT"

gates=(
  scripts/phase8_tui_e2e.sh
  scripts/phase8_tui_interactive.sh
  scripts/phase8_airgap_tui.sh
  scripts/phase7_offline_e2e.sh
  scripts/phase7_airgap_bundle.sh
  scripts/phase6_offline_proof.sh
)

for g in "${gates[@]}"; do
  echo "========================================================" | tee -a "$OUT"
  echo "GATE: $g" | tee -a "$OUT"
  echo "========================================================" | tee -a "$OUT"
  bash "$g" >>"$OUT" 2>&1
  rc=$?
  echo "----- EXIT($g)=$rc -----" | tee -a "$OUT"
  echo "" >>"$OUT"
done

echo "DONE gates. See $OUT"
