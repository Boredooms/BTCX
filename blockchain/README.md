# blockchain/ — Bitcoin data domain

Owns everything about Bitcoin-layer data: acquisition adapters, parsing,
normalization into the canonical `pkg/schema` types, and wallet/transaction
domain helpers.

```
blockchain/
├── acquisition/   # network adapters (ONLY place allowed to use the network)
│   ├── explorer/  # blockchain explorer provider adapter
│   ├── mempool/   # mempool/stream provider adapter
│   └── interfaces/# sdk.DataSource implementations live behind these
├── parser/        # CSV/JSON/XML streaming parsers
├── normalizer/    # source format -> canonical schema.Transaction
├── transactions/  # transaction domain logic
├── wallets/       # wallet domain logic
└── types/         # blockchain-specific value types (re-export schema where apt)
```

## Rules
- Acquisition is the only sub-package permitted to import an HTTP client.
- An adapter's job ends once data is normalized and ready to persist locally.
- Never invent a wallet↔IP relationship; network evidence lives in `network/`.

Status: scaffolded. Implemented in Phase 2 (ingestion) and Phase-later
(acquisition adapters).
