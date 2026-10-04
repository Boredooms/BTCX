# risk/ — scoring + propagation

Aggregates ML, pattern, cluster, graph and propagation signals into an
explainable 0–100 score with confidence and a per-signal breakdown. Risk is an
investigative prioritization signal, never proof of wrongdoing.

```
risk/
├── scoring/     # weighted signal aggregation -> schema.RiskAssessment
├── propagation/ # distance-decay propagation from seed entities
├── signals/     # signal definitions + normalization
└── thresholds/  # configurable alert thresholds
```

Implements `sdk.RiskService`. Deterministic for a fixed dataset+model+config.
Phase 6.
