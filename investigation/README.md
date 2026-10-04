# investigation/ — orchestration

High-level investigation services that drive the fixed pipeline. Presentation
layers (CLI/TUI) call these; they never reconstruct the pipeline themselves.

```
investigation/
├── wallet/       # AnalyzeWallet orchestration
├── transaction/  # AnalyzeTransaction orchestration
├── entity/       # AnalyzeEntity orchestration
├── timeline/     # temporal reconstruction
└── orchestrator/ # shared pipeline runner producing schema.InvestigationResult
```

The engine facade in `sdk/engine.go` is the current orchestration entry point;
these packages will host the per-subject orchestration as it grows. Phase 7.
