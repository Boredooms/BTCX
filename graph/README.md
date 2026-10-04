# graph/ — relationship engine

Custom Go graph subsystem over locally persisted edges. Traversal is always
bounded; the full graph is never materialized at once.

```
graph/
├── nodes/      # node abstractions (wallet/transaction/ip/entity)
├── edges/      # edge construction + persistence helpers
├── traversal/  # BFS/DFS, N-hop (bounded)
├── paths/      # shortest path / path finding
├── clusters/   # connected components, cluster queries
└── algorithms/ # degree, fan-in/out, centrality, risk propagation support
```

Implements `sdk.GraphService`. Phase 3.
