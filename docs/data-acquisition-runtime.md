# BCTX Data Acquisition Runtime (Phase 5)

Production acquisition/sync layer. It fetches wallet/transaction data from a
provider while connected, converts it into the **exact same canonical records**
produced by local file ingestion, and persists them through the shared
ingestion path so graph/features/ML/risk work identically regardless of origin.

## Network boundary (hard rule)

```
NETWORK
   ↓
acquisition.DataSource (provider adapter)   ← ONLY network-capable layer
   ↓
acquisition DTO  →  canonical schema.Transaction / NetworkObservation
   ↓
ingestion.Persister  (shared with CSV/JSON/XML import)
   ↓
SQLite canonical records → graph → features → ML → risk → evidence
```

Enforced by `tests/offline/boundary_test.go`: analysis packages (storage,
graph, ml, risk, evidence, investigation, ingestion, reporting, pkg,
blockchain/normalizer, blockchain/parser) must not import `net/http`/`net/url`.
Only `acquisition/` and `network/state` (a connectivity probe) touch the
network.

## DataSource interface (capability-oriented, provider-neutral)

```go
type DataSource interface {
    ProviderName() string
    ProviderVersion() string
    Capabilities() ProviderCapabilities
    FetchWalletHistory(ctx, WalletHistoryRequest) (AcquiredWalletPage, error)
    FetchTransaction(ctx, txid) (AcquiredTransaction, error)
}
```

Provider-neutral DTOs (`AcquiredTransaction`, `AcquiredWalletPage`,
`AcquiredNetworkObservation`, `PaginationState`, `AcquisitionMetadata`,
`ProviderCapabilities`) keep the rest of the system independent of any vendor
response model. Unsupported capabilities return `ErrCapabilityUnsupported`.

## Provider implemented

A deterministic in-memory **fake provider** (`provider = "fake"`, the default)
is implemented for tests and local demos — it opens no network connections. A
real explorer adapter plugs in behind the same `DataSource` interface without
changing storage/graph/ML/risk. `bctx provider list` reports capabilities
locally (never requires the network).

## Wallet sync

`bctx sync wallet <address>` streams history page-by-page:

```
page → discover txids → bounded-concurrent detail fetch → map to canonical →
Persister batch → graph BuildIncremental → checkpoint → next page
```

Bounded memory: a 4000-tx / 80-page sync uses ~1 MiB heap (test
`TestLargePaginationBoundedMemory`) — work is `O(page + queue + batch)`, not
`O(total history)`.

## Transaction sync

`bctx sync tx <txid>` fetches one transaction, maps to canonical, persists
idempotently. Returns `already_local` without a fetch when the tx is already
present and complete.

## Shared canonical path (no duplication)

Both file import and acquisition call `ingestion.NewPersister(...).PersistBatch`,
the single source of truth for satoshi conversion, size/weight/vsize, fee-rate,
script normalization, completeness, conservation, dedup, and incremental graph
update. The acquisition adapter never reimplements this math and never writes
graph tables directly.

## Pagination

Provider-neutral `PaginationState{Cursor, Page}` — the engine treats it as
opaque and advances until `Done` or an empty state. Bounded page size; no
unbounded in-memory collection; cursor stored in the checkpoint for resume.

## Concurrency

Bounded worker pool (`max_workers`) over a bounded job channel for transaction
detail fetches; no goroutine leaks (producer/consumer close discipline);
persistence remains single-path/batched through the Persister. Race-clean.

## Rate limiting + retry

- `RateLimiter`: token-interval limiter at `requests_per_second` (0 = unlimited),
  cancellation-aware.
- `Retrier`: exponential backoff with full jitter, honoring a provider
  `RetryAfter`. Retries only retryable categories
  (`ErrProviderUnavailable/RateLimited/Transport/Timeout`); never retries
  `ErrInvalidTarget/ProviderNotFound/CapabilityUnsupported/PermanentProviderError`.
  Bounded `max_retries`; retry count recorded in stats + checkpoint.

## Checkpoints / resume

`sync_checkpoints` table (migration `0005`) stores sync id, case, provider +
version, target type/target, cursor+page, counts, retries, status, error,
timestamps. `bctx sync wallet <addr> --resume` reloads the cursor and refuses a
resume whose target/provider differs (`ErrSyncCheckpointConflict`). Statuses:
pending/running/completed/failed/cancelled. Ctrl+C leaves a resumable
checkpoint.

## Idempotency + enrichment

Transaction identity is `txid`; observations use a composite hash. Re-syncing a
wallet yields `0 new`. A locally **partial** transaction can be **enriched** by
a more complete acquired version (`mergeTransaction`): inputs/outputs taken from
the more complete side, non-zero scalar fields preferred, earliest source
timestamp kept, local known-good fields never lost to a provider gap.

## Local cache

Before fetching, the engine checks the local store; a complete local tx is not
refetched. A partial local tx is fetched to enrich it.

## Provenance

Each acquired record carries `schema.Provenance` (source type explorer/network,
provider identifier, retrieved-at, schema version, sync id). The checkpoint
records page/retry/discovered/acquired/persisted/duplicate/partial/rejected
counts. No secrets or raw responses are logged.

## Offline semantics

`--offline`, `--airgap`, or `network.mode = offline/airgap` makes any `sync`
command refuse with `ErrOfflineAcquisition` before any provider construction.
All local commands (`analyze`, `graph`, `dataset`, feature/ML/risk) remain fully
offline. `analyze wallet <addr>` never touches the network unless `--sync` is
passed AND acquisition is permitted.

## Configuration

```toml
[acquisition]
provider = "fake"
max_workers = 4
page_size = 100
batch_size = 500
request_timeout = "15s"
max_retries = 4
initial_backoff = "200ms"
max_backoff = "5s"
requests_per_second = 5
```

Credentials for a future authenticated provider come from the existing secure
config/environment mechanism and are never committed or printed.

## CLI

```
bctx provider list
bctx sync wallet <address> [--json] [--offline] [--resume]
bctx sync tx <txid>        [--json] [--offline]
bctx sync status <sync-id> [--json]
bctx sync list
bctx analyze wallet <address> --sync   # explicit acquire-then-analyze
```

## Typed errors

`ErrProviderUnavailable, ErrRateLimited, ErrProviderNotFound, ErrInvalidTarget,
ErrCapabilityUnsupported, ErrSyncCheckpointConflict, ErrOfflineAcquisition,
ErrPermanentProviderError, ErrAcquisitionCancelled, ErrTransport, ErrTimeout` —
branch with `errors.Is`, not string matching.

## Security

Provider responses are untrusted: txids/addresses/IPs/ports/timestamps/numbers
are validated through the shared validator before persistence; parameterized
repository APIs only; bounded page/queue/concurrency/retry/error sizes; no
credential persistence or logging.

## Limitations

- Only the fake provider is implemented; a real explorer adapter is future work
  (the interface and pipeline are ready).
- Resume re-streams from the stored cursor with dedup (correct + idempotent)
  rather than byte-seeking.
- `sent_to` edge semantics unchanged from Phase 3 (output-weighted analytical
  relationship, not exact fund matching).

## Test evidence

14 acquisition engine tests (multi-page, idempotency, tx/already-local,
not-found, retry/permanent/throttle, cancellation, capability, graph+features
integration, partial enrichment, empty wallet, checkpoint) + bounded-memory
large pagination + offline/no-call guard + network-import boundary guard. Full
regression (ML parity, graph, risk, ingestion) green; `-race`, `vet`, `gofmt`
clean.
