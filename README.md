# BTCX — Bitcoin Forensic Intelligence Platform

BTCX is a **terminal-first, offline-first** Bitcoin forensic intelligence
workstation. It acquires Bitcoin + network metadata while connected, owns the
data locally, builds a transaction/entity graph, applies local AI/ML and graph
analysis, calculates explainable risk, and generates evidence-backed reports —
then keeps working with the network fully disconnected.

```
Acquire while connected.
Own the data locally.
Analyze locally.
Disconnect the network.
Keep investigating.
```

This is **not** a web dashboard, wallet, exchange, or explorer clone. It is a
native Linux CLI + interactive TUI over a local intelligence engine. Only the
acquisition layer ever touches the network; every analysis path is local.

---

## Highlights

- **Offline-first analysis**: features → local ML (pure-Go tree inference) →
  risk → evidence → report, all with no network required.
- **Interactive TUI** (Bubble Tea): dashboard, analysis, wallet/entity,
  transaction, graph, network/IP, geo map, detection, alerts, live monitoring,
  reports, extensions, settings.
- **Explainable risk**: a 0–100 score with a per-signal breakdown and linked
  evidence; a corroboration gate keeps saturated ML signals honest.
- **Local-file data source**: analyze from a dataset you already have on disk,
  completely offline (see below).
- **Report engine**: deterministic snapshots rendered to JSON / Markdown /
  HTML / PDF, viewable in the TUI and exportable to disk.
- **Extension / knowledge manager**: load local knowledge bases, model packs,
  and demo-example datasets.

---

## Install (Linux)

**One-liner (downloads the latest release, verifies its checksum, installs
`bctx` + `btcx` onto your PATH):**

```bash
curl -fsSL https://raw.githubusercontent.com/Boredooms/BTCX/main/install.sh | sh
```

- Installs to `/usr/local/bin` when writable or `sudo` is available, otherwise
  `~/.local/bin`. Override with `PREFIX=…` or `NO_SUDO=1`.
- Pin a version: `… | VERSION=v0.1.0 sh`.
- The installer uses the network **only to download** the release; BTCX itself
  runs fully offline once installed.

```bash
# system-wide (prompts for sudo if needed):
curl -fsSL https://raw.githubusercontent.com/Boredooms/BTCX/main/install.sh | sudo sh
# user-local, no sudo:
curl -fsSL https://raw.githubusercontent.com/Boredooms/BTCX/main/install.sh | NO_SUDO=1 sh
```

**Build from source** (requires Go 1.23+ and CGO for the SQLite storage layer):

```bash
git clone https://github.com/Boredooms/BTCX.git && cd BTCX
CGO_ENABLED=1 go build -o bin/bctx ./apps/bctx
bash scripts/install_btcx.sh      # install bctx + btcx onto your PATH
```

**Run & verify:**

```bash
btcx            # launch the interactive TUI
bctx --help     # CLI
bctx doctor     # diagnose offline readiness (binary, config, db, models, ...)
```

---

## Analyze a wallet

```bash
# Analyze from the active case's already-local evidence.
bctx analyze wallet <WALLET>

# Acquire missing data from the provider first (requires network).
bctx analyze wallet <WALLET> --sync
```

### Local-file data source (offline-only)

If you already have the Bitcoin/account/bulk metadata on disk, point the
analyze command at it directly. The file is the single source of truth for that
request — **no network access occurs**, and a missing/invalid/irrelevant file is
a clear error (never a silent fallback to an external provider):

```bash
bctx analyze wallet <WALLET> --file /path/to/data.csv
```

- Supported formats reuse the existing ingestion pipeline: **CSV / JSON /
  NDJSON / XML**.
- Bulk datasets (many wallets) are supported — BTCX streams the import and then
  scopes the investigation to the target wallet.
- The data flows through the **same** pipeline as every other mode:
  `local file → parse → validate → normalize → local store → graph → features →
  local ML → risk → evidence → report`.
- Provenance (source file, SHA-256, record counts) is recorded in the case.

### From the TUI (offline)

The same local-file import is available inside the interactive TUI — no CLI
needed:

1. `btcx` → open or create a case.
2. Go to **Data / Ingestion** (`d`), press **f**, type a dataset path
   (CSV/JSON/NDJSON/XML), and press **Enter**. The file is imported offline
   through the existing ingestion pipeline and the graph is rebuilt.
3. Go to **Analysis** (`2`) or **Wallet / Entity** (`3`) and open the wallet —
   it now resolves from the imported local evidence, fully offline.

---

## Offline guarantee

The analysis path is local-only. A dedicated boundary test asserts that no
analysis package pulls in a network transport, and the `--airgap` /`--offline`
flags hard-disable acquisition. With `--file`, acquisition is disabled for that
request regardless of other flags.

```bash
bctx --airgap analyze wallet <WALLET> --file /path/to/data.csv
```

---

## Layout

```
apps/bctx        native entrypoint (CLI + TUI)
cli/commands     Cobra command tree
tui/             Bubble Tea TUI (screens, components, theme)
ingestion/       streaming CSV/JSON/NDJSON/XML import + normalization
blockchain/      acquisition, parsing, normalization
graph/           transaction/entity graph build + queries
ml/, models/     local feature engine + tree-model inference
risk/            explainable risk engine
evidence/        evidence assembly
reporting/       deterministic report snapshots + renderers
storage/sqlite   local case storage + migrations
pkg/schema       canonical domain schema
```

---

## License

See [LICENSE](LICENSE).
