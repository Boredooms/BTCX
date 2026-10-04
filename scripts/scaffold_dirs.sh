#!/usr/bin/env bash
# One-shot scaffolding of the domain-grouped directory tree.
# Safe to re-run; only creates missing dirs and .gitkeep placeholders.
set -euo pipefail
cd "$(dirname "$0")/.."

dirs=(
  blockchain/acquisition/explorer blockchain/acquisition/mempool blockchain/acquisition/interfaces
  blockchain/parser blockchain/normalizer blockchain/transactions blockchain/wallets blockchain/types
  network/observations network/ip network/ports network/correlation network/types
  graph/nodes graph/edges graph/traversal graph/paths graph/clusters graph/algorithms
  ml/models/anomaly ml/models/entity ml/models/flow ml/inference ml/features ml/preprocessing ml/schemas
  detection/anomaly detection/peeling detection/mixing detection/fanout detection/velocity
  risk/scoring risk/propagation risk/signals risk/thresholds
  evidence/collector evidence/provenance evidence/explain evidence/models
  investigation/wallet investigation/transaction investigation/entity investigation/timeline investigation/orchestrator
  monitoring/sessions monitoring/events monitoring/incremental monitoring/state
  storage/repositories storage/indexes
  cases/metadata cases/lifecycle
  reporting/models reporting/markdown reporting/json reporting/html reporting/pdf
  cli/prompts cli/completion
  tui/components tui/graph tui/alerts tui/evidence tui/state
  sdk/investigation sdk/graph sdk/analysis sdk/reporting
  ml-lab/datasets ml-lab/generators ml-lab/features ml-lab/training ml-lab/evaluation ml-lab/notebooks ml-lab/export
  tests/unit tests/integration tests/e2e
)

for d in "${dirs[@]}"; do
  mkdir -p "$d"
  [ -e "$d/.gitkeep" ] || touch "$d/.gitkeep"
done

echo "scaffolded ${#dirs[@]} directories"
