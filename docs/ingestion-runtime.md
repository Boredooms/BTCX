# BCTX Ingestion Runtime (Phase 4)

Production, streaming, offline data ingestion. Turns local CSV / JSON / NDJSON /
XML files into canonical BCTX records, feeds the graph and feature engine, and
records provenance + resumable checkpoints. No package in the file-ingestion
path imports an HTTP client.

## Supported formats

| format | parser | streaming |
|--------|--------|-----------|
| CSV | header-driven, column aliases | row-by-row (`encoding/csv`) |
| JSON | top-level array | `json.Decoder` object-by-object |
| NDJSON / JSONL | one object per line | `bufio.Scanner` |
| XML | `<dataset><transaction>…` | `xml.Decoder` token stream |

`--format auto` resolves by extension then content inspection (`<` → XML,
`[` → JSON, `{` → NDJSON) and never loads the whole file to decide.

## Canonical schema (unchanged single source of truth)

`pkg/schema.Transaction` now carries, in addition to inputs/outputs/fee:

- `FeeSats` (int64) and per-amount `AmountSats` — exact satoshi accounting
- `Size{BaseSize,TotalSize,Weight,VSize}` — BIP-141 (`weight=3*base+total`,
  `vsize=ceil(weight/4)`)
- `FeeRateSatVB` — canonical fee-rate unit **sat/vByte** = `fee_sats / vsize`
- `ScriptType` (canonical) + `OriginalScriptType` (raw source preserved)
- `Completeness` — `valid` | `partial` | `invalid`

Monetary math uses integer satoshis internally (`schema.BTCToSats` /
`SatsToBTC`); BTC floats remain for display/compatibility. Conservation
(`sum(in) == sum(out) + fee`) is checked in exact satoshis for complete txs.

Migrations `0003` (tx size/sats/completeness columns) and `0004` (dataset
provenance, `import_checkpoints`, `import_errors`) are additive — Phase 0–3 case
databases still open.

## Completeness

- `invalid` — no txid (dropped from persistence, counted as rejected)
- `partial` — missing inputs OR outputs (kept; conservation not verifiable)
- `valid` — both sides present and conservation holds

Partial records are **kept**, not silently dropped, so downstream analysis can
distinguish data quality.

## Network observations

Rows/objects with `src_ip`/`dst_ip` become `NetworkObservation`s. IPs are
validated (IPv4 **and** IPv6), ports 0–65535, timestamps parsed (RFC3339 +
fallbacks). Observation id is a deterministic composite hash of
`src_ip|src_port|dst_ip|dst_port|txid|timestamp` (dedup key — never timestamp
alone). When a `txid` is present the graph gets an `IP --observed_with--> TX`
edge; an unlinked observation is persisted without fabricating a txid.

## Deduplication (idempotent)

Transaction identity is `txid`; observation identity is the composite hash.
Re-importing the same file yields `0 new, N duplicates`. Verified by
`TestIdempotentImport` and the CLI (JSON then equivalent XML → 3 duplicates).

## Provenance

Every import records a `datasets` row: id (`ds-<sha12>`), source path + streamed
SHA-256, format, schema/parser/importer versions, records read/accepted/
rejected/duplicate/partial, tx/wallet/obs counts, status. `bctx dataset
inspect <id>` and `bctx dataset list` surface it.

## Checkpoints / resume

`import_checkpoints` persists dataset id, source path + hash, format, parser
version, records done, last batch, status (`pending|running|completed|failed|
cancelled`). `bctx dataset resume <id>` re-validates the source SHA-256 and
refuses to resume a changed file; dedup makes the re-run add only missing
records. Ctrl+C saves a resumable checkpoint.

## Batching + atomicity

Transactions are committed in atomic batches (`BatchSize` = 2000). A failed
batch rolls back; committed batches and the checkpoint survive. The graph is
updated incrementally per batch (`BuildIncremental`), never fully rebuilt.

## Errors

Up to `MaxErrorBuffer` (1000) per-record diagnostics (record no, field, reason,
bounded source fragment) are stored in `import_errors` (and are the basis for a
future `imports/errors.jsonl`). Full records are never dumped to logs.

## Feature-engine integration

Flow structural features now use **real** averaged `vsize` and `feerate_sat_vb`
from imported transactions (orchestrator `flowFeaturesFromTxs`); nominal
`180`/`12` are used only when a dataset supplies no size metadata. `feature-
schema-v1` semantics are unchanged and the flow/anomaly ONNX parity is intact
(no retraining).

## CLI

```
bctx dataset import <file> [--format auto|csv|json|ndjson|xml] [--json]
bctx dataset list
bctx dataset inspect <id>
bctx dataset resume <id>
```

## Offline

Entire ingestion + analysis path verified under `unshare -rn`: import a sized
CSV, build the graph, `analyze wallet A --offline` → identical to connected,
using real transaction-size data. No network, cloud, or Python runtime needed.

## Acquisition boundary

The importer depends only on local files + the repository + the graph builder.
Future provider adapters (Phase 5) will produce the same canonical records
through a `DataSource` adapter without changing storage, graph, features, ML or
risk — the ingestion normalizer/validator are reused.

## Limitations

- `sent_to` edge amount is output value (not matched input→output flow).
- A single flat CSV network row shares the `txid` column with transactions;
  richer multi-entity CSV layouts can be added as format adapters.
- Resume currently re-streams with dedup rather than seeking to `last_batch`
  (correct and idempotent, but re-reads the file).
