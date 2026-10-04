#!/usr/bin/env bash
# Deletes ONLY the empty .gitkeep-only scaffold dirs (no .go, no other files).
# Re-verifies each is empty immediately before removing. Safe + idempotent.
set -eu
cd /home/devara/btcx

DIRS="
blockchain/acquisition/interfaces
blockchain/acquisition/mempool
blockchain/transactions
blockchain/types
blockchain/wallets
cases/lifecycle
cases/metadata
cli/completion
cli/prompts
detection/anomaly
detection/fanout
detection/mixing
detection/peeling
detection/velocity
evidence/explain
evidence/models
evidence/provenance
graph/algorithms
graph/clusters
graph/nodes
graph/paths
graph/traversal
investigation/entity
investigation/timeline
investigation/transaction
investigation/wallet
ml/models/anomaly
ml/models/entity
ml/models/flow
ml/models
ml/preprocessing
ml/schemas
monitoring/events
monitoring/incremental
monitoring/sessions
monitoring/state
network/correlation
network/ip
network/observations
network/ports
network/types
risk/signals
risk/thresholds
sdk/analysis
sdk/graph
sdk/investigation
sdk/reporting
storage/indexes
storage/repositories
tests/e2e
tests/integration
tests/unit
tui/alerts
tui/evidence
tui/graph
tui/state
"

removed=0
skipped=0
for d in $DIRS; do
  if [ ! -d "$d" ]; then
    continue
  fi
  go=$(find "$d" -name '*.go' 2>/dev/null | wc -l)
  other=$(find "$d" -type f ! -name '.gitkeep' 2>/dev/null | wc -l)
  if [ "$go" -eq 0 ] && [ "$other" -eq 0 ]; then
    rm -rf "$d"
    echo "removed  $d"
    removed=$((removed+1))
  else
    echo "SKIP (not empty)  $d  (go=$go other=$other)"
    skipped=$((skipped+1))
  fi
done
echo "---"
echo "removed=$removed skipped=$skipped"
