#!/usr/bin/env bash
# Run every phase acceptance script and print a compact pass/fail summary.
set -u
export PATH=/home/devara/go-sdk/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /home/devara/btcx

for s in phase8_tui_e2e phase8_tui_interactive phase8_airgap_tui phase7_offline_e2e phase7_airgap_bundle phase6_offline_proof; do
  echo "=== $s ==="
  if bash "scripts/$s.sh" > "/tmp/$s.log" 2>&1; then
    echo "  EXIT_OK"
  else
    echo "  EXIT_FAIL (rc=$?)"
  fi
  grep -E 'SUMMARY|PASS=[0-9]|FAIL=[0-9]' "/tmp/$s.log" | tail -1
done
