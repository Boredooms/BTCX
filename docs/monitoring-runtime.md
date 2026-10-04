# BCTX Phase 6 — Live Monitoring Runtime

The monitoring runtime watches a wallet for new on-chain activity, persists it
through the canonical ingestion path, re-runs the existing analysis pipeline on
the affected subject, and emits deterministic, deduplicated alerts when risk
changes. It introduces **no new analysis** — it is pure orchestration over
components built in Phases 3–5.

## Architecture

```
monitor wallet <addr>
      │
      ▼
monitoring.Service.MonitorWallet(ctx, addr, sessionID)   # poll loop, bounded by MaxPolls
      │
      ├─ acquisition.DataSource.FetchWalletHistory(...)   # the ONLY network hop
      │        (blockchain/acquisition/explorer: real Esplora provider)
      │
      ├─ ingestion.Persister.PersistBatch(canon, obs, enrich=true)
      │        → normalizer.Finalize (completeness / vsize / feerate)
      │        → repo.SaveTransactions  (idempotent by txid)
      │        → graph.BuildIncremental (edges, incremental)
      │
      ├─ monitor events recorded (idempotent: event id = hash(txid|state))
      │
      ├─ Analyzer(subject)                                # injected orchestrator wrapper
      │        → features → ONNX (anomaly/flow) → risk scoring → evidence
      │
      ├─ risk delta recorded when score / signals / patterns change
      └─ alerts emitted + deduplicated (dedup_key UNIQUE)
```

The monitoring package imports **no** `net/http` / `net` — enforced by
`tests/offline/boundary_test.go`. Network access is confined to the acquisition
provider. The import cycle with the orchestrator is avoided by injecting an
`Analyzer func(ctx, subject) (AnalysisResult, error)`; the CLI wires the real
orchestrator into it.

## Providers

| Provider  | Package                                   | Network | Use              |
|-----------|-------------------------------------------|---------|------------------|
| `esplora` | `blockchain/acquisition/explorer`         | yes     | real Bitcoin data |
| `fake`    | `acquisition` (NewFakeProvider)           | no      | deterministic tests |

**Esplora** targets the Blockstream.info / mempool.space REST API
(`https://blockstream.info/api`). It is the only component that opens sockets.
The HTTP client forces IPv4 (`DialContext` → `tcp4`) because the WSL dev host
has a broken outbound IPv6 route, bounds response bodies to 8 MiB, and maps HTTP
status to typed errors (404→NotFound, 429→RateLimited with `Retry-After`,
5xx→Unavailable).

### Serialization (satoshis)

Esplora reports monetary values as integer **satoshis**. The adapter maps them
straight into the canonical sat fields via `schema.SatsToBTC` / `BTCToSats`
(1 BTC = 100,000,000 sats) — fee/size math is never reimplemented.

- `vout[].value` / `vin[].prevout.value` → `AcquiredIO.AmountBTC` (sat-exact).
- `size` → BaseSize and TotalSize (Esplora exposes total serialized size).
- `weight` → `VSize = ceil(weight/4)`.
- `fee` (sats) → `FeeBTC` + `FeeSats`.
- `status.{confirmed,block_height,block_time}` → confirmation state + timestamp.

**Addressless but value-bearing outputs/inputs** (P2PK such as the genesis
coinbase, bare multisig) are retained with a deterministic `script:<type>`
placeholder so no satoshis are lost from the canonical record; valueless
non-address outputs (OP_RETURN) are dropped. Graph edges key on real addresses
and ignore the placeholder.

Verified live against the genesis coinbase
`4a5e1e4b…da33b`: single output = 5,000,000,000 sats (exactly 50 BTC),
weight 816, vsize 204, confirmed at height 0.

## Persistence (migration 0006)

| Table              | Purpose                                              |
|--------------------|------------------------------------------------------|
| `monitor_sessions` | resumable session state + run counters               |
| `monitor_events`   | bounded operational event history (idempotent)       |
| `risk_deltas`      | factual change log of score / signals / patterns     |
| `monitor_alerts`   | deduplicated alerts (`dedup_key` UNIQUE)             |

Migration 0006 drops the unused `monitor_sessions` / `monitor_events`
placeholder stubs shipped (empty) in 0001 and replaces them with the real
operational schema. Canonical transactions, wallets, and graph edges continue to
live in their existing Phase 3–5 tables; monitoring stores orchestration state
only.

> Migration runner note: `storage/migrations/embed.go` executes each statement
> of a migration file individually (`splitStatements`) because go-sqlite3 does
> not reliably run every statement of a multi-statement string inside an
> explicit transaction.

## Idempotency & crash recovery

- Canonical persist dedups by txid; re-seeing a transaction persists nothing.
- Monitor events dedup by `hash(txid|confirmed-state)`, so a mempool→confirmed
  transition is a distinct event while a repeated poll is not.
- Alerts dedup by `dedup_key = session|subject|trigger|event`.
- On restart with the same `sessionID`, run-scoped counters
  (`events_*`, `tx_acquired`, `alerts_generated`) reset to report what **this**
  run did; the risk baseline and canonical data survive. A crash+restart that
  only re-sees persisted data reports **0 new** work.

## Health & resilience

- Retryable provider errors (timeout, 5xx, 429) trigger bounded reconnect with
  backoff; health transitions `connected → reconnecting → degraded`.
- Exceeding `MaxReconnectAttempts` records a gap and surfaces the error.
- Context cancellation is a clean stop (`status=paused`), not a failure.
- Capability check: a provider without `WalletHistory` is refused with
  `ErrCapabilityUnsupported` before any work.

## CLI

```
bctx monitor wallet <address> [--interval 30s] [--max-polls N]
bctx monitor status <session-id>
bctx monitor list
bctx monitor pause|stop <session-id>
bctx monitor resume <session-id> [--max-polls N]
```

`--max-polls` defaults to `1` (one poll then exit); `0` runs until Ctrl+C
(production). Configure the provider in `config.toml`:

```toml
[acquisition]
provider = "esplora"          # or "fake"

[network]
acquisition_enabled = true
mode = "online"               # "offline"/"airgap" refuse acquisition
```

## Offline boundary

`monitor wallet` / `monitor resume` call `acquisitionAllowed()` **before**
constructing any provider. With `--offline`, `--airgap`, or
`acquisition_enabled=false` they refuse with `ErrOfflineAcquisition`
("acquisition unavailable offline"). Local, read-only commands
(`monitor list`, `monitor status`, `provider list`, `analyze --offline`,
`dataset list`) always work offline.

Proven by `scripts/phase6_offline_proof.sh`, which asserts refusal with
`--offline`, with `--airgap`, and under `unshare -rn` (no network namespace),
while confirming local commands still succeed.

## Tests

- `monitoring/service_test.go` — 11 scenarios: session creation, event/tx dedup,
  mempool→confirmed, risk delta + alert, alert dedup, provider outage +
  reconnect, cancellation safety, capability-unsupported, crash-recovery
  idempotency, case isolation, confirmation-state representation.
- `monitoring/scripted_provider_test.go` — deterministic provider with per-poll
  pages and injected failures.
- `blockchain/acquisition/explorer/esplora_live_test.go` — **opt-in** live
  integration (`BCTX_LIVE=1`): genesis serialization + real wallet-history
  mapping; skips cleanly when offline.
- `scripts/phase6_offline_proof.sh` — offline hard-gate proof.
- `scripts/phase6_live_e2e.sh` — opt-in full live E2E (acquire → persist →
  analyze → idempotent re-poll) through the real CLI.
```
