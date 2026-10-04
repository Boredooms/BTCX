# network/ — IP / network metadata domain

Owns network-layer observations and their correlation with blockchain data.
Network evidence is only ever populated from supplied/acquired telemetry —
never fabricated.

```
network/
├── observations/ # network observation ingest + domain logic
├── ip/           # IP parsing/validation, local GeoIP (MMDB) lookup
├── ports/        # port/protocol helpers
├── correlation/  # correlate observations <-> transactions
├── state/        # runtime connectivity status (CONNECTED/DISCONNECTED/AIRGAP)
└── types/        # network value types
```

Implemented so far: `state/` (connectivity probing for honest status display).
Correlation and GeoIP land in later phases.
