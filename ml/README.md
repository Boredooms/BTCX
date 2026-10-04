# ml/ — local model runtime + feature engine (Go)

Production inference only. Models are local ONNX files; nothing is downloaded
at runtime. Training/research lives in `ml-lab/` (Python).

```
ml/
├── models/        # per-model Go loaders
│   ├── anomaly/
│   ├── entity/
│   └── flow/
├── inference/     # ONNX Runtime integration, model loading/caching
├── features/      # feature extraction (implements sdk.FeatureService)
├── preprocessing/ # scaling/encoding matching training
└── schemas/       # feature/model schema definitions + compat checks
```

Implements `sdk.FeatureService` and `sdk.MLService`. Every prediction carries
model version + feature schema + confidence. Phases 4–5.
