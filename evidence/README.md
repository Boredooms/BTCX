# evidence/ — explanation + provenance

Turns computed signals into source-linked `schema.EvidenceItem`s. The evidence
engine never produces a statement that is not backed by a stored record.

```
evidence/
├── collector/  # gather evidence from signals/detectors/graph
├── provenance/ # link evidence to source records (txids, obs ids)
├── explain/    # build the `explain` narrative from structured evidence
└── models/     # evidence domain helpers
```

Implements `sdk.EvidenceService`. Phase 6.
