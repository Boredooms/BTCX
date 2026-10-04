# BCTX Phase 7 — Forensic Reporting Runtime

The reporting runtime turns a completed investigation into a reproducible,
evidence-backed forensic report in four formats (JSON, Markdown, HTML, PDF),
plus a self-describing export bundle that can be independently verified. It
introduces **no new analysis**: a report is a frozen, deterministic view over an
already-produced `schema.InvestigationResult` and the local case repository.

Reporting is **network-forbidden**. Nothing in `reporting/` (or its
sub-packages) imports an HTTP client, an acquisition adapter, a provider SDK, or
the investigation orchestrator. Report generation composes only local evidence
and never opens a socket, even when the host is connected. The
`report generate`/`export` CLI commands run the orchestrator first (local
analysis only, honoring `--offline`/`--airgap`, never `--sync`), then render
from the frozen snapshot. See `docs/offline-architecture.md` for how the
boundary is enforced.

---

## 1. Report domain model

A report is an **investigation snapshot**, not just formatted text. Each report
carries a stable identity block:

| Field              | Source                                             |
|--------------------|----------------------------------------------------|
| `report_id`        | `rpt-<case>-<subject>-<first 12 hex of snapshot hash>` (no timestamp) |
| `case_id`          | investigation result                               |
| `investigation_id` | investigation result `id`                          |
| `subject` / `subject_type` | investigation result                       |
| `schema_version`   | `report-schema-v1`                                 |
| `report_version`   | generator version (`report-gen-v1`)                |
| `generated_at` / `generated_by` | wall-clock + build stamp (NOT hashed) |

### Report content sections

A forensic report can contain: case information, investigation subject,
acquisition summary, dataset / source provenance, transaction summary, flow
analysis, graph summary, related wallets, entity signals, network observations,
ML signals, risk assessment, risk deltas, alert history, evidence, timeline,
graph paths, limitations, data quality, offline/connected state, and generation
metadata. Sections with no data render as "not available" rather than being
fabricated.

### Evidence-truthful wording

Reports use the evidence engine's vocabulary — *observed*, *detected*,
*inferred*, *pattern match*, *anomaly*, *evidence indicates*. Pattern detectors
are named with a `-like` suffix (e.g. `peeling_chain_like`) to signal a pattern
match, not a proven classification. Entity clusters are described as an
"inferred relationship, not proven ownership." An anomaly is never turned into a
claim of criminal intent. These phrasings live in the `limitations` section and
in the per-item descriptions.

---

## 2. Snapshot + determinism contract

`Service.BuildSnapshot` captures a `models.ReportSnapshot`: a copy of every
local input the report depends on — the investigation result, subject
transactions, correlated network observations, subject alerts, evidence IDs,
redacted dataset provenance, the acquisition sync summary, monitoring session
state (events / risk deltas / monitor alerts), model versions, feature-schema
version + SHA, and the total-ordered traceability IDs (`tx_ids`, `alert_ids`,
`evidence_ids`). Every slice is stored in a **total order** so the serialized
snapshot — and therefore its hash — is reproducible.

An existing report never silently changes because the database changed later:
the snapshot is frozen at generation time, and `report inspect` recomputes the
hash from the stored snapshot JSON, so it is byte-reproducible.

### What the hash covers

`reporting.SnapshotHash` is the SHA-256 of the **canonical JSON** of the
snapshot. Canonical JSON disables HTML escaping and sorts object keys
recursively, so map iteration order and build differences cannot change the
bytes.

- **Excluded from the hash:** the report's own `generated_at` / `generated_by`.
  These have no snapshot field at all; they are added at render time.
- **Included in the hash (deliberate):** the investigation result's
  `created_at`. Report identity is tied to the *specific* investigation
  snapshot, so the same subject investigated at two different times yields two
  distinct report IDs. This decision is locked by
  `reporting/tests/snapshot_test.go:TestSnapshotCreatedAtInHash` (review NIT #3).

### Determinism in practice (what the e2e proof asserts)

`scripts/phase7_offline_e2e.sh` proves determinism at two levels, matching what
the system actually guarantees:

1. **Persisted-snapshot hash reproducibility.** The hash is defined over the
   frozen snapshot. `report generate` persists a snapshot; `report inspect`
   recomputes its hash. Recomputing it repeatedly yields the identical hash —
   this is the authoritative "the snapshot hash is stable" guarantee.
2. **Report-content reproducibility across independent re-generations.** Two
   fully independent `report generate` runs over identical local evidence
   produce byte-identical reports after normalizing (a) the documented
   wall-clock stamps — `generated_at`, and the `snapshot_sha256` / `report_id`
   suffix that derive from the per-run `result.created_at` the orchestrator
   restamps — and (b) ~1e-7 cross-run float jitter in the lowest digits of the
   ONNX anomaly score (the ML parity target is ~1e-7; see the README). After
   that normalization every substantive field — transactions, graph, flow,
   risk signals/weights, evidence, timeline — is identical.

> A fresh `report generate` re-runs the orchestrator, so its `created_at` (and
> hence the raw snapshot hash) advances with the wall clock by design. For a
> byte-identical hash across calls, render from the same persisted snapshot
> (`report inspect`) rather than re-analyzing.

---

## 3. Report versioning policy

Two opaque, equality-compared version tokens are stamped on every report and
snapshot (`reporting/version.go`):

- `report-schema-v1` — the report / snapshot **data contract**.
- `report-gen-v1` — the **generator** (rendering + snapshot-assembly rules).

They are **not** semantic version numbers to be parsed or range-compared. Any
change to the meaning, shape, or wording of a report — or to how a snapshot is
built or hashed — requires minting a **new** string. An existing token is never
silently reused for a changed meaning, so a consumer that sees an identical
token can trust the contract is identical. `report inspect` rejects any
`schema_version` other than `report-schema-v1` with
`unsupported report schema %q` rather than silently reinterpreting it.

---

## 4. The four formats and their offline guarantees

| Format   | Package              | Offline guarantee                                   |
|----------|----------------------|-----------------------------------------------------|
| JSON     | `reporting/json`     | Deterministic canonical JSON: version markers first, all other keys sorted recursively, HTML escaping off, numbers preserved exactly, trailing newline. Machine-readable, source-linked (IDs trace every conclusion to local evidence). No secrets. |
| Markdown | `reporting/markdown` | Pure local text. No external assets, no remote CSS/JS/image, no web fonts. |
| HTML     | `reporting/html`     | Single self-contained file: inline CSS only, no `<script src>`, no CDN, no remote font, no remote image, no `@import`. Opens fully offline on an air-gapped machine. |
| PDF      | `reporting/pdf`      | Native pure-Go PDF writer. No Python, no headless browser, no network, no cloud renderer. Output begins with the `%PDF-` magic. Contains title, case, subject, risk, confidence, key findings, timeline, transaction summary, graph summary, evidence, alerts, provenance and limitations. |

The HTML "no external asset" and PDF `%PDF-` guarantees are asserted by
`scripts/phase7_offline_e2e.sh` and by `reporting/tests` (markdown/HTML
self-contained checks, PDF structural checks).

### No secret leakage

Dataset provenance is redacted (`reporting/models` `RedactDatasets`) to an
allow-list of nine fields: `id`, `source_file`, `sha256`, `schema_version`,
`tool_version`, `format`, `parser_version`, `importer_version`, `imported_at`.
Sync/monitor provenance is projected to counts + status only — never transport
or credential fields. `reporting/tests/secret_leak_test.go` renders all four
formats and asserts no `api_key` / `secret` / `token` / `password` /
`authorization` / `bearer` substring appears.

---

## 5. Export bundle layout + verify

`Service.ExportBundle(snap, formats, outDir)` writes a self-describing
`report/` directory under `outDir`. The bundle is atomic (temp-file + rename
per file; full rollback on any write error) and deterministic:

```
<outDir>/report/
  report.json      report.md      report.html      report.pdf   # requested formats
  evidence.json    # snapshot evidence, referentially intact, ordered by ID
  timeline.json    # deterministic merged timeline
  manifest.json    # report_id, case_id, schema/generator version,
                   # snapshot_sha256, generated_at/by, per-file {name,bytes,sha256}
  checksums.sha256 # coreutils "<sha256>  <name>" lines, sorted by name
```

`Service.Verify(bundleDir)` recomputes the SHA-256 of every file listed in
`manifest.json` and `checksums.sha256` and compares them. A mismatch or missing
file is reported as **data** (`VerifyResult.OK = false` with a per-file
`OK` / `MISMATCH` / `MISSING` status), not a Go error; only I/O failures reading
the manifest/checksum files surface as errors. The `report verify` CLI exits
non-zero when `OK` is false.

---

## 6. Report CLI

All five subcommands run with `--offline` and `--airgap`; reporting makes no
network call even when the config is online. `list` / `inspect` / `verify` do
zero analysis and are always offline.

```
bctx report generate <subject> [--format json|md|html|pdf] [--out PATH]
bctx report export   <subject> [--formats json,md,html,pdf] [--out DIR]
bctx report list
bctx report inspect  <report-id>
bctx report verify   <bundle-dir>
```

- **generate** — runs the orchestrator (local analysis; `offline = --offline ||
  --airgap || !AcquisitionAllowed()`; no `--sync`), builds the snapshot, renders
  one file, persists the snapshot, appends `report_generated` to the audit log.
  Default format from `[reports] default_format` (`pdf`). `--out`: absolute path
  used as-is; a relative path resolves under the case dir and may not escape it
  (`ErrUnsafePath`). Empty `--out` → `<case>/reports/<report_id>/report.<ext>`.
- **export** — orchestrator → snapshot → `ExportBundle` into `<out>/report/`;
  validates each format token; appends `report_exported`.
- **list** — persisted reports in the active case; legacy rows (pre-snapshot)
  shown as `legacy (no snapshot)`.
- **inspect** — recomputes and prints the snapshot hash + present sections;
  rejects any schema other than `report-schema-v1`.
- **verify** — verifies a `report/` bundle directory; non-zero exit on failure.

Human output surfaces the honest NETWORK / ACQUISITION / LOCAL ANALYSIS banner
(AGENTS.md §17): NETWORK is read from the configured mode without probing,
ACQUISITION is always "not performed (reporting never acquires)", and LOCAL
ANALYSIS shows OFFLINE / CONNECTED from the orchestrator flag. `--json` output
is stable (sorted keys).

---

## 7. Acceptance scripts

| Script                              | Proves                                                                 |
|-------------------------------------|------------------------------------------------------------------------|
| `scripts/phase7_offline_e2e.sh`     | The whole post-acquisition chain (analyze → graph → neighbors → monitor → report generate all four formats → export → verify) runs under `unshare -rn`; persisted-snapshot hash reproducible; report content stable across re-runs; HTML has no external asset; PDF starts `%PDF-`. |
| `scripts/phase7_airgap_bundle.sh`   | `generate` + `export` + `verify` need only the binary + a local case DB + local ONNX models — Go/Python/Node/Docker are poisoned on PATH and never invoked; no internet. |
| `scripts/phase6_offline_proof.sh`   | Unchanged Phase-6 offline hard-gate (no regression).                   |

Both Phase-7 scripts fall back from `unshare -rn` to `--airgap` + `BCTX_OFFLINE`
with a printed note when the network namespace is unavailable, and exit non-zero
on any failure.
