# BCTX Graph Runtime (Phase 3)

The graph is a core analytical subsystem built from the local case database. It
is not a visualization structure. All queries are local and offline.

## Model

Nodes (referenced by canonical id, never duplicated into new tables):

- `WALLET` — address
- `TRANSACTION` — txid
- `IP` — network source address
- `ENTITY` — inferred cluster (via common-input heuristic)

Edges (`graph_edges` table; typed constants in `pkg/schema`):

| type | from → to | meaning |
|------|-----------|---------|
| `input_to` | wallet → transaction | address was an input |
| `output_to` | transaction → wallet | address was an output |
| `sent_to` | wallet → wallet | derived value movement (output-weighted) |
| `observed_with` | ip → transaction | network observation linked a tx |
| `member_of` | wallet → entity | inferred cluster membership |

`sent_to` is a **derived analytical edge** — it does not assert that a specific
input funded a specific output, only that both appeared in the same transaction.

## Persistence

Edges live in `graph_edges` (migration `0001`), with composite adjacency indexes
added in `0002`: `(from_type, from_id)`, `(to_type, to_id)`, `type`, `timestamp`.
Edge ids are deterministic (`sha1(type|from|to)`), so builds are **idempotent**:
`INSERT OR REPLACE` collapses duplicates and rebuilding yields identical edges.

## Build

`graph.Builder`:

- `BuildAll` — derive edges from every local transaction + observation.
- `BuildIncremental` — add edges for a specific batch (used by `dataset import`).
- `Rebuild` — `DeleteEdges` then `BuildAll`. Canonical records are never deleted.

`bctx dataset import` builds incrementally; `bctx graph build` / `graph rebuild`
run explicitly. `bctx graph stats` reports edge counts by type.

## Traversal (`graph.Service`)

Database-backed, bounded — never loads the whole graph:

- `Neighbors(id)` — direct neighbors.
- `NeighborsDepth(id, depth)` / `Subgraph(center, depth)` — bounded BFS; the
  subgraph is JSON-serializable and capped at `MaxNodes` (default 5000).
- `Path(src, dst)` — shortest path (BFS) as a node sequence; hop count = len-1.

Traversal is undirected for reachability (edges connect both directions).

## Graph-derived features

`graph.Service.WalletMetrics(address, radius)` computes the real values that
replace the former nominal ones (feature-schema-v1 definitions preserved):

- `degree` — distinct undirected neighbors
- `fan_in` / `fan_out` — distinct wallet sources / destinations over `sent_to`
- `graph_depth` — max BFS depth over forward `sent_to` within radius
- `hop_count` — longest explored forward chain depth
- `chain_length` — nodes in the dominant (highest-value-first) chain
- `value_decay` — terminal/initial value along the dominant chain, clamped (0,1]
- `counterparty_diversity` — normalized entropy `H/ln(k)` over counterparty
  `sent_to` amounts

The feature engine (`ml/features`) consumes these via `WithGraphMetrics` and a
`graph.MetricsAdapter` (avoids an import cycle). When no graph is built yet, it
falls back to bounded tx-local estimates. `split_ratio` / `merge_ratio` remain
transaction-structural.

## Risk propagation (`risk/propagation`)

Baseline deterministic distance-decay from seed entities:

```
contribution(target) = seed.risk * decay^distance   (decay 0.5, maxDepth 3)
```

Each step records seed, target, distance, contribution and the discovery path,
so a score change is explainable. `PropagateAll` keeps each target's strongest
nearby seed. Label propagation / personalized PageRank are future options
evaluated against this baseline.

## Investigation integration

`investigation/orchestrator` builds the feature engine with real graph metrics
and attaches a bounded subgraph (depth 2) to every `InvestigationResult`. The
pipeline is unchanged in order: local data → features (graph-backed) → anomaly →
entity → flow + structural → risk → evidence → result.

## Offline

The entire graph path is local SQLite + in-process traversal; no network client
is imported. Verified: `unshare -rn ./bin/bctx analyze wallet W123 --offline`
produces output identical to connected mode.

## Limitations

- `sent_to` amount weighting is output value, not a matched input→output flow.
- Per-transaction size (`vsize`/`feerate`) is not yet in the canonical schema, so
  flow structural features use nominal values there (flagged for Phase 4).
- Connected components / centrality are not yet implemented (future).

## CLI

```
bctx graph build | rebuild | stats
bctx graph wallet <id> --depth N [--json]
bctx neighbors <id> --depth N [--json]
bctx path <src> <dst> [--json]
```
