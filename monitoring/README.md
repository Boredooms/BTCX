# monitoring/ — continuous analysis

Event-driven monitor sessions. New events are persisted before expensive
analysis. When the network drops, acquisition pauses but local analysis stays
available — simulated events are never labeled LIVE.

```
monitoring/
├── sessions/     # session lifecycle + persistence
├── events/       # event capture + dedup
├── incremental/  # incremental graph/feature/risk updates
└── state/        # disconnect/reconnect state machine
```

Phase 9.
