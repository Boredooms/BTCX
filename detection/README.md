# detection/ — suspicious pattern + clustering

Structured detectors that emit `schema.PatternResult` with evidence IDs. All
findings use "-like" language and are described as pattern matches, never proof
of criminal activity.

```
detection/
├── anomaly/   # anomaly-signal wiring around the ML score
├── peeling/   # peeling-chain-like detector
├── mixing/    # mixing-like structure detector
├── fanout/    # high fan-out / high fan-in detectors
└── velocity/  # rapid-flow / transaction-velocity detector
```

Implements `sdk.DetectionService`. Phase 6.
