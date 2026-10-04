# BCTX — TASKS.md

## 0. Mission

Build **BCTX** as a Linux-native, terminal-first Bitcoin forensic intelligence platform with a strict offline-first architecture.

The final experience must feel like a serious terminal investigation workstation:

```bash
bctx
```

The implementation must support:

```text
Acquire relevant data while connected
        ↓
Persist it locally
        ↓
Build/search local evidence
        ↓
Construct transaction/entity graph
        ↓
Run local AI/ML
        ↓
Calculate risk
        ↓
Explain findings
        ↓
Generate reports
        ↓
Disconnect network
        ↓
Continue all intelligence/reporting work offline
```

The project must be distributable from GitHub Releases and installable on Linux through a simple `curl` command.

---

# 1. Non-Negotiable Product Rules

Before implementation, freeze these rules.

## 1.1 Runtime

The production runtime is:

```text
Go
```

The end user should run:

```bash
bctx
```

without needing:

```text
Node.js
Python
PostgreSQL
Redis
Docker
Kubernetes
```

for core functionality.

## 1.2 Primary UI

The product is:

```text
CLI + Interactive TUI
```

There is no required browser dashboard for the core product.

## 1.3 Offline Rule

Only the acquisition layer may need network access.

Everything below the acquisition boundary must work with:

```text
NETWORK = OFF
```

including:

```text
wallet lookup
transaction lookup
IP lookup against local data
graph traversal
feature extraction
ML inference
pattern detection
risk scoring
risk propagation
alerts
explainability
summaries
report generation
PDF/HTML/JSON export
TUI
```

## 1.4 Data Rule

BCTX cannot obtain genuinely new Bitcoin events while physically disconnected.

Therefore:

```text
CONNECTED
→ acquire/sync/monitor

OFFLINE
→ analyze everything already captured locally
```

The UI must never pretend that new network events are arriving while disconnected.

## 1.5 Evidence Rule

Every important finding must be traceable to stored evidence.

Never produce:

```text
"wallet is suspicious"
```

without:

```text
signal
+
source records
+
analysis path
+
confidence
```

## 1.6 Model Rule

The production system must use local model files.

No runtime model download.

Recommended:

```text
Python
→ train
→ evaluate
→ export ONNX

Go BCTX
→ ONNX Runtime
→ local model
```

## 1.7 Network Metadata Rule

Blockchain explorers provide blockchain/transaction information, not necessarily every network-layer observation requested by the SIH problem.

Therefore keep:

```text
Blockchain Acquisition
+
Network Metadata Acquisition
```

as separate adapters.

Never invent an IP-to-wallet relationship.

Use IP/port/timing only when that evidence is actually present in an imported/acquired telemetry dataset.

---

# 2. Updated Production Architecture

```text
                                  USER
                                   │
                                   ▼
                         ┌───────────────────┐
                         │    BCTX CLI/TUI   │
                         └─────────┬─────────┘
                                   │
                         ┌─────────▼─────────┐
                         │ Command Dispatcher│
                         └─────────┬─────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
                    ▼              ▼              ▼
             Acquisition       Investigation   Case Manager
                 Layer             Engine
                    │              │
                    │              ├──────────────┐
                    │              │              │
                    │              ▼              ▼
                    │           Graph         Features
                    │              │              │
                    └──────┐       └──────┬───────┘
                           │              │
                           ▼              ▼
                     LOCAL EVIDENCE STORE
                           │
                           ▼
                    ┌───────────────┐
                    │ Local ML      │
                    │ ONNX Runtime  │
                    └───────┬───────┘
                            ▼
                     ┌─────────────┐
                     │ Risk Engine │
                     └──────┬──────┘
                            ▼
                    ┌───────────────┐
                    │ Evidence      │
                    │ Engine        │
                    └──────┬────────┘
                           │
                ┌──────────┼───────────┐
                ▼          ▼           ▼
               TUI       Alerts      Reports
```

### Offline boundary

```text
                  NETWORK
                     │
                     ▼
             ┌───────────────┐
             │ Acquisition   │
             │ Adapters ONLY │
             └───────┬───────┘
                     │
             LOCAL PERSISTENCE
                     │
          ───── OFFLINE BOUNDARY ─────
                     │
                     ▼
          Graph / ML / Risk / Evidence
                     │
                     ▼
                 TUI / CLI
                     │
                     ▼
                  Reports
```

---

# 3. Repository Bootstrap

## Task 001 — Create repository

Create:

```text
bctx/
├── cmd/
│   └── bctx/
│       └── main.go
├── internal/
│   ├── cli/
│   ├── tui/
│   ├── command/
│   ├── config/
│   ├── acquisition/
│   ├── ingestion/
│   ├── normalization/
│   ├── storage/
│   ├── indexing/
│   ├── graph/
│   ├── features/
│   ├── ml/
│   ├── risk/
│   ├── evidence/
│   ├── alerts/
│   ├── monitoring/
│   ├── reports/
│   ├── cases/
│   ├── audit/
│   └── security/
├── pkg/
│   ├── schema/
│   └── sdk/
├── models/
│   ├── manifests/
│   └── development/
├── migrations/
├── testdata/
├── tests/
├── scripts/
├── docs/
├── .github/
│   └── workflows/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── CLAUDE.md
├── AGENTS.md
├── TASKS.md
└── LICENSE
```

### Acceptance

```bash
go test ./...
go build ./cmd/bctx
```

both succeed.

---

# 4. Runtime Foundation

## Task 002 — Implement version command

```bash
bctx version
```

Return:

```text
BCTX
Version: x.y.z
Commit: ...
Build: ...
OS: linux
Arch: amd64
```

## Task 003 — Implement config loader

Use local TOML configuration.

Suggested location:

```text
~/.config/bctx/config.toml
```

## Task 004 — Implement runtime directories

Create:

```text
~/.bctx/
├── cases/
├── models/
├── geo/
├── cache/
├── logs/
└── tmp/
```

## Task 005 — Implement `bctx init`

It must initialize all local runtime state without network access.

### Acceptance

```bash
bctx init
bctx doctor
```

both work with the network disabled.

---

# 5. Local Case System

## Task 006 — Create case model

Support:

```text
case_id
name
created_at
updated_at
status
dataset list
model versions
```

## Task 007 — Implement:

```bash
bctx case create <name>
bctx case list
bctx case open <name>
bctx case close
```

## Task 008 — Case directory layout

```text
~/.bctx/cases/<case-id>/
├── case.db
├── imports/
├── evidence/
├── reports/
├── exports/
├── logs/
└── metadata.json
```

### Acceptance

Two cases must remain isolated.

---

# 6. SQLite Persistence

## Task 009 — Database schema

Create migrations for:

```text
cases
datasets
wallets
transactions
transaction_inputs
transaction_outputs
network_observations
entities
entity_members
graph_edges
features
model_predictions
risk_assessments
evidence_items
alerts
monitor_sessions
reports
audit_events
```

## Task 010 — Repository layer

Implement:

```text
WalletRepository
TransactionRepository
NetworkRepository
EntityRepository
GraphRepository
FeatureRepository
PredictionRepository
RiskRepository
EvidenceRepository
AlertRepository
ReportRepository
AuditRepository
```

## Task 011 — Database transactions

All multi-table ingestion operations must be transactional.

### Acceptance

Database survives process interruption without corrupting committed state.

---

# 7. Canonical Data Schema

## Task 012 — Define canonical transaction model

Support:

```text
timestamp
txid
inputs[]
outputs[]
fees
script type
```

## Task 013 — Define network observation model

Support:

```text
timestamp
src_ip
src_port
dst_ip
dst_port
```

## Task 014 — Define optional enrichment

Support:

```text
country
ASN
provider/source
```

Only when actually available.

## Task 015 — Version schemas

Create:

```text
schema-v1
```

Store schema version on imported/acquired data.

---

# 8. Data Ingestion

## Task 016 — CSV importer

```bash
bctx dataset import file.csv
```

Use streaming parsing.

## Task 017 — JSON importer

```bash
bctx dataset import file.json
```

Support both array and line-oriented representations where appropriate.

## Task 018 — XML importer

```bash
bctx dataset import file.xml
```

## Task 019 — Schema validation

Reject/record malformed records without killing the complete import.

## Task 020 — Batch persistence

Benchmark batch sizes.

## Task 021 — Import progress

Show:

```text
records read
records accepted
records rejected
transactions
wallets
network records
speed
ETA
```

## Task 022 — Import provenance

Persist:

```text
source file
SHA-256
import timestamp
schema version
tool version
```

## Task 023 — Import resumability

Persist checkpoints for large imports.

### Gate A

The ingestion system must import a synthetic multi-million-record dataset without loading the full dataset into RAM.

---

# 9. Data Acquisition

## Task 024 — Define provider interface

```go
type BlockchainSource interface {
    GetAddress(ctx context.Context, address string) (...)
    GetAddressTransactions(ctx context.Context, address string, cursor string) (...)
    GetTransaction(ctx context.Context, txid string) (...)
}
```

Exact interfaces may be refined.

## Task 025 — Explorer adapter

Implement one blockchain data provider adapter.

Responsibilities:

```text
address history
transaction details
pagination
retry
timeouts
rate limits
deduplication
```

## Task 026 — Network metadata adapter

Implement a separate interface for:

```text
network observation dataset
network telemetry feed
```

Do not mix blockchain and network evidence into one fake provider.

## Task 027 — Provider configuration

External credentials/endpoints live only in local config.

## Task 028 — Acquisition persistence

All external responses become local evidence before analysis.

## Task 029 — `sync`

Implement:

```bash
bctx sync wallet <wallet>
bctx sync transaction <txid>
```

## Task 030 — Strict offline flag

```bash
bctx analyze wallet <wallet> --offline
```

must never invoke acquisition.

---

# 10. Indexing

## Task 031 — Wallet index

```text
wallet → transactions
```

## Task 032 — TXID index

```text
txid → transaction
```

## Task 033 — IP index

```text
ip → network observations
```

## Task 034 — Timestamp index

Support efficient time-range queries.

## Task 035 — Cluster index

```text
cluster → members
```

## Task 036 — Query profiling

Measure:

```text
wallet lookup
TX lookup
IP lookup
time-range query
graph expansion
```

### Gate B

Indexed exact lookup must be interactive on the target demo dataset.

---

# 11. Search

## Task 037 — Exact search

```bash
bctx search W123
bctx search TX123
bctx search 10.1.2.3
```

## Task 038 — Filter search

Support:

```text
risk
amount
timestamp
transaction type
cluster
```

## Task 039 — TUI search

Implement `/`.

---

# 12. Graph Engine

## Task 040 — Define graph node types

```text
Wallet
Transaction
IP
Entity
```

## Task 041 — Define graph edge types

```text
Wallet -> Transaction
Transaction -> Wallet
IP -> Transaction
Wallet -> Entity
```

## Task 042 — Graph persistence

Persist graph relationships locally.

## Task 043 — N-hop traversal

```bash
bctx graph wallet W123 --depth 3
```

## Task 044 — Neighbor queries

```bash
bctx neighbors W123
```

## Task 045 — Path queries

```bash
bctx path W123 W900
```

## Task 046 — Graph filters

Support:

```text
time
risk
amount
node type
edge type
cluster
```

## Task 047 — Graph statistics

Calculate:

```text
degree
fan-in
fan-out
connected components
centrality where useful
```

### Gate C

A wallet can be expanded into a bounded local subgraph without requiring any network access.

---

# 13. Feature Engine

## Task 048 — Transaction features

Implement:

```text
transaction count
incoming volume
outgoing volume
average amount
amount variance
fee statistics
```

## Task 049 — Temporal features

Implement:

```text
transactions/hour
transactions/minute
median time gap
burstiness
velocity
```

## Task 050 — Graph features

Implement:

```text
degree
fan-in
fan-out
neighbor count
graph depth
counterparty diversity
```

## Task 051 — Flow features

Implement:

```text
chain length
hop count
value decay
split ratio
merge ratio
```

## Task 052 — Network features

Implement only when network evidence exists:

```text
observation count
unique IP count
unique ASN count
timing correlation
```

## Task 053 — Feature schema versioning

All vectors must identify:

```text
feature_schema = v1
```

---

# 14. Synthetic Data Generator

## Task 054 — Build deterministic synthetic generator

Create:

```text
normal activity
```

and:

```text
anomaly scenarios
entity clusters
peeling-chain-like flows
mixing-like structures
high fan-out
high fan-in
rapid transaction flows
network correlation examples
```

## Task 055 — Ground truth labels

Store development-only labels.

## Task 056 — Reproducible seeds

Generator should support:

```bash
bctx-dev generate --seed 12345
```

or equivalent.

## Task 057 — Demo dataset

Produce a deterministic SIH demo dataset.

### Gate D

Every intended detector must have synthetic fixtures that can be evaluated automatically.

---

# 15. ML Training Pipeline

ML work happens outside the production runtime.

## Task 058 — Create `ml/` development workspace

```text
ml/
├── data/
├── features/
├── training/
├── evaluation/
├── export/
└── models/
```

## Task 059 — Baseline anomaly model

Start with a robust baseline such as:

```text
Isolation Forest
```

and compare alternatives later.

## Task 060 — Train/evaluation split

Ensure synthetic test data is not leaked into training.

## Task 061 — Evaluate

Measure:

```text
precision
recall
F1
PR-AUC
false-positive rate
false-negative rate
latency
```

## Task 062 — Entity model

Implement:

```text
behavior/graph feature clustering
```

then evaluate graph embedding approaches.

## Task 063 — Flow model/detector

Implement a validated flow detector.

It can be a hybrid of:

```text
ML
+
graph algorithms
+
domain features
```

provided the overall system still includes a real ML use case.

---

# 16. Model Export

## Task 064 — Export ONNX

Models:

```text
anomaly.onnx
entity.onnx
flow.onnx
```

## Task 065 — Model manifest

Include:

```text
name
version
feature schema
input schema
output schema
training metadata
checksum
```

## Task 066 — Model compatibility checker

Before inference:

```text
manifest
 ↓
checksum
 ↓
schema compatibility
 ↓
runtime compatibility
```

---

# 17. Local ML Runtime

## Task 067 — Integrate ONNX Runtime

Go must invoke models locally.

## Task 068 — Local inference API

```go
type MLModel interface {
    Predict(ctx context.Context, features FeatureVector) (Prediction, error)
}
```

## Task 069 — No-download guarantee

Inference must not download models.

## Task 070 — Model status

```bash
bctx model status
bctx model info anomaly
```

### Gate E

Disable networking and demonstrate successful ML inference.

---

# 18. Pattern Detection

## Task 071 — Peeling-chain-like detector

Detect:

```text
residual continuation
repeated split
amount decay
chain structure
timing
```

## Task 072 — Mixing-like detector

Analyze:

```text
input count
output count
participant count
value distribution
timing
graph structure
```

## Task 073 — High fan-out detector

## Task 074 — High fan-in detector

## Task 075 — Rapid-flow detector

Each detector returns structured evidence.

---

# 19. Entity Clustering

## Task 076 — Common-input heuristic

Implement as an evidence signal.

## Task 077 — Behavioral similarity

Calculate similarity using local feature vectors.

## Task 078 — Graph similarity

Use local graph structure.

## Task 079 — Cluster result

Return:

```text
cluster ID
members
confidence
signals
```

Never phrase inferred clustering as proven real-world ownership.

---

# 20. Risk Engine

## Task 080 — Risk assessment model

Implement:

```text
0–100 risk score
0–1 confidence
signal breakdown
```

## Task 081 — Signal aggregation

Inputs:

```text
anomaly
pattern
cluster
graph
network correlation
propagation
```

## Task 082 — Configurable weights

Store configuration locally.

## Task 083 — Risk delta

Support:

```text
previous risk
current risk
delta
contributing changes
```

---

# 21. Risk Propagation

## Task 084 — Seed risk model

Define seed entities and their local risk.

## Task 085 — Distance-decay propagation

Implement baseline.

## Task 086 — Store propagation provenance

Record:

```text
source seed
path
distance
contribution
```

## Task 087 — Evaluate advanced propagation later

Candidates:

```text
label propagation
graph diffusion
personalized PageRank
```

Only after the baseline works.

---

# 22. Evidence Engine

## Task 088 — Evidence schema

Evidence must include:

```text
ID
type
description
source record IDs
score
confidence
created_at
```

## Task 089 — Evidence linking

Every risk signal must point to evidence.

## Task 090 — Source drill-down

From:

```text
risk
 ↓
evidence
 ↓
transaction/network record
```

must work.

---

# 23. Alert Engine

## Task 091 — Alert generation

Create:

```text
alert_id
subject
risk
confidence
priority
reason
evidence
timestamp
status
```

## Task 092 — Ranked alerts

Sort using defined ranking logic.

## Task 093 — Alert states

```text
NEW
REVIEWING
DISMISSED
CONFIRMED
EXPORTED
```

"CONFIRMED" means analyst review state, not automatic legal determination.

---

# 24. Investigation Service

## Task 094 — Create high-level InvestigationService

```text
AnalyzeWallet()
AnalyzeTransaction()
AnalyzeEntity()
```

## Task 095 — Orchestration

```text
local data check
 ↓
optional acquisition
 ↓
persist
 ↓
graph
 ↓
features
 ↓
ML
 ↓
patterns
 ↓
cluster
 ↓
risk
 ↓
evidence
 ↓
alert
 ↓
result
```

## Task 096 — Structured InvestigationResult

All presentation layers consume the same result object.

### Gate F — Backend Complete

Before TUI work:

```text
dataset import
wallet lookup
TX lookup
graph
features
ML
patterns
clustering
risk
evidence
alerts
explain
report object
```

must work entirely through services/tests.

---

# 25. CLI — Backend-First Interface

## Task 097 — Implement:

```bash
bctx status
bctx wallet <id>
bctx transaction <id>
bctx ip <id>
bctx analyze wallet <id>
bctx graph wallet <id>
bctx neighbors <id>
bctx path <source> <destination>
bctx alerts
bctx explain wallet <id>
bctx report wallet <id>
```

## Task 098 — CLI should be useful before TUI

Every major backend capability must be testable from CLI commands.

## Task 099 — JSON output

Add:

```bash
--json
```

where useful.

This makes automation and tests easier.

---

# 26. Terminal Frontend Phase

Only after Gate F passes.

## Task 100 — TUI shell

Build:

```text
header
sidebar
main panel
evidence panel
footer
```

## Task 101 — Home / landing view

Home should show:

```text
BCTX
current case
dataset
record counts
model state
graph state
network state
alerts
recent investigations
```

## Task 102 — Search view

## Task 103 — Alert view

## Task 104 — Wallet view

## Task 105 — Transaction view

## Task 106 — Graph view

## Task 107 — Timeline view

## Task 108 — Evidence view

## Task 109 — Monitoring view

## Task 110 — Reports view

---

# 27. TUI UX Rules

The TUI must feel like a native investigation workstation.

Do not build:

```text
web page squeezed into terminal
```

Build:

```text
investigation cockpit
```

Persistent footer should show:

```text
↑↓ Navigate
Enter Open
Tab Panel
/ Search
G Graph
E Explain
R Report
Q Back
```

---

# 28. Graph TUI

## Task 111 — Bounded subgraph renderer

Do not render huge graphs directly.

## Task 112 — Node selection

Press:

```text
Enter
```

to inspect a node.

## Task 113 — Graph filters

Support:

```text
risk
time
amount
node type
cluster
```

## Task 114 — Graph focus

Allow:

```text
focus selected node
expand one hop
expand N hops
collapse
```

---

# 29. Evidence-Driven UX

Every investigation page should answer:

```text
WHAT?
WHY?
HOW CONFIDENT?
WHERE DID THIS COME FROM?
```

Example:

```text
W123
Risk: 91

WHY FLAGGED
[HIGH] transaction anomaly
[HIGH] peeling-chain-like pattern
[MED] cluster relation

EVIDENCE
TX901
TX902
TX903
...
```

---

# 30. Monitoring

## Task 115 — Monitor session

```bash
bctx monitor wallet <id>
```

## Task 116 — Acquisition stream adapter

Support:

```text
HTTP polling
WebSocket/provider stream
```

where the selected provider supports it.

## Task 117 — Persist events immediately

New data must be saved before expensive analysis.

## Task 118 — Incremental graph update

## Task 119 — Incremental features

## Task 120 — Incremental ML

## Task 121 — Risk delta

## Task 122 — Live TUI updates

---

# 31. Monitoring Disconnect Workflow

When network disappears:

```text
acquisition stops
local state remains valid
local analysis remains available
TUI remains usable
```

Display:

```text
NETWORK: DISCONNECTED
ACQUISITION: PAUSED
LOCAL ANALYSIS: AVAILABLE
LAST EVENT: ...
```

Do not show fake live events.

---

# 32. Monitoring Reconnect Workflow

When network returns:

```text
reconnect
 ↓
find last local event
 ↓
request missing data
 ↓
deduplicate
 ↓
persist
 ↓
graph update
 ↓
analysis update
 ↓
risk update
```

---

# 33. Summarization

## Task 123 — Deterministic summary

Create summary from structured evidence first.

## Task 124 — Optional local LLM

Only after deterministic summaries work.

Possible local LLM uses:

```text
report prose
monitor summary
natural-language investigation
```

The LLM must not invent risk evidence.

---

# 34. Reports

## Task 125 — Report object

Implement:

```text
InvestigationReport
```

including:

```text
case
subject
risk
confidence
transactions
graph summary
patterns
cluster
network evidence
model metadata
dataset metadata
evidence
```

## Task 126 — JSON

## Task 127 — Markdown

## Task 128 — HTML

## Task 129 — PDF

## Task 130 — Report versioning

Store:

```text
v1
v2
v3
```

when monitoring changes state.

---

# 35. GeoIP

## Task 131 — Local MMDB support

If enabled:

```text
IP
 ↓
local database
 ↓
country
ASN
approximate region
```

No online lookup during analysis.

---

# 36. Offline Security

## Task 132 — Network boundary enforcement

Only acquisition packages may import network clients.

## Task 133 — Network call audit

Provide mechanisms to prove:

```text
analysis path made no external requests
```

## Task 134 — Airgap mode

Implement:

```bash
bctx --airgap
```

or equivalent.

In airgap mode:

```text
all acquisition disabled
```

## Task 135 — Model checksum verification

## Task 136 — Dataset hash tracking

## Task 137 — Safe import paths

## Task 138 — Secure temporary files

---

# 37. `bctx doctor`

## Task 139 — Full health checker

Check:

```text
binary
config
case
database
schema
indexes
models
graph
report engine
GeoIP
offline mode
```

Expected:

```text
OFFLINE READY
```

---

# 38. Distribution Architecture

## Task 140 — Release targets

Build:

```text
Linux amd64
Linux arm64
```

## Task 141 — GitHub Releases

Every release should publish:

```text
bctx-linux-amd64.tar.gz
bctx-linux-arm64.tar.gz
checksums.txt
model-bundle.tar.zst or equivalent
```

Exact archive format can be chosen during implementation.

## Task 142 — Release manifest

Publish machine-readable release metadata.

## Task 143 — Signed/checksummed artifacts

Verify artifacts before installation.

---

# 39. Install Script

## Task 144 — Official installer

The target UX:

```bash
curl -fsSL https://raw.githubusercontent.com/<ORG>/bctx/main/scripts/install.sh | sh
```

The installer must:

```text
detect Linux
detect architecture
resolve latest compatible release
download binary package
download required model bundle
verify checksums/signatures
install bctx
install models
initialize directories
run bctx doctor
```

## Task 145 — Safer explicit download form

Also document:

```bash
curl -fsSL https://raw.githubusercontent.com/<ORG>/bctx/main/scripts/install.sh \
  -o /tmp/bctx-install.sh && sh /tmp/bctx-install.sh
```

## Task 146 — Offline/local installer

Provide:

```bash
./install-local.sh
```

for air-gapped machines receiving a release bundle through controlled media.

---

# 40. Release Layout

Recommended:

```text
GitHub Release
├── bctx-linux-amd64.tar.gz
├── bctx-linux-arm64.tar.gz
├── bctx-models-vX.Y.Z.tar.zst
├── bctx-demo-vX.Y.Z.tar.zst
├── checksums.txt
└── release-manifest.json
```

The executable remains:

```text
bctx
```

The installed application should feel like a single native tool even though models/data are packaged as companion assets.

---

# 41. CI/CD

## Task 147 — CI

Run:

```text
go fmt
go vet
go test
race tests
Python tests
model validation
build
```

## Task 148 — Release CI

On tag:

```text
build binaries
package assets
generate checksums
publish GitHub Release
```

## Task 149 — Clean Linux smoke test

CI should install the release into a clean Linux environment and execute:

```bash
bctx doctor
bctx dataset import demo.csv
bctx analyze wallet W123
```

---

# 42. Offline CI

## Task 150 — No-network integration test

Use a CI environment with blocked network access.

Verify:

```text
bctx starts
database opens
models load
wallet lookup works
graph works
risk works
reports work
```

## Task 151 — Network call assertion

The offline path should fail the test if it attempts an external connection.

---

# 43. End-to-End Scenario Matrix

All scenarios below must be tested.

## Scenario A — Fresh install, connected

```text
install
 ↓
init
 ↓
sync wallet
 ↓
analyze
 ↓
report
```

## Scenario B — Fresh install, offline

```text
install from local bundle
 ↓
doctor
 ↓
import local dataset
 ↓
analyze --offline
 ↓
report
```

## Scenario C — Analyze missing wallet while connected

```text
analyze
 ↓
missing locally
 ↓
acquire
 ↓
persist
 ↓
analyze
```

## Scenario D — Analyze missing wallet while offline

```text
analyze --offline
 ↓
missing data
 ↓
clear error
```

No network attempt.

## Scenario E — Re-analyze fully local wallet

```text
analyze
 ↓
local data
 ↓
local graph
 ↓
local ML
```

No network.

## Scenario F — Live monitor

```text
monitor
 ↓
receive event
 ↓
persist
 ↓
analyze
 ↓
risk delta
```

## Scenario G — Monitor loses network

```text
network off
 ↓
acquisition paused
 ↓
local UI remains functional
```

## Scenario H — Monitor reconnects

```text
network on
 ↓
catch up
 ↓
deduplicate
 ↓
persist
 ↓
reanalyze
```

## Scenario I — Corrupt model

```text
model check
 ↓
checksum/schema failure
 ↓
inference blocked
```

## Scenario J — Corrupt dataset record

```text
bad record
 ↓
record rejected
 ↓
import continues
```

## Scenario K — Large dataset

```text
multi-million rows
 ↓
streaming import
 ↓
index
 ↓
graph
```

RAM must remain bounded.

## Scenario L — Export report offline

```text
offline
 ↓
report
 ↓
PDF
```

Must work.

## Scenario M — New report version

```text
existing investigation
 ↓
new local events
 ↓
risk change
 ↓
report v2
```

## Scenario N — Case isolation

```text
case-A
case-B
```

must never mix evidence.

---

# 44. Performance Gates

## Gate G1 — Startup

BCTX should start fast enough to feel like a native CLI tool.

## Gate G2 — Exact lookup

Wallet/TX lookup must be interactive.

## Gate G3 — Streaming ingestion

Large imports must not scale RAM linearly with file size.

## Gate G4 — Graph

Bounded N-hop traversal must be interactive.

## Gate G5 — ML

Single-entity local inference must be low-latency.

## Gate G6 — Monitoring

TUI must remain responsive while background analysis runs.

---

# 45. Final Backend Gate

Do not begin visual polish until this passes:

```text
[PASS] storage
[PASS] ingestion
[PASS] indexes
[PASS] graph
[PASS] features
[PASS] anomaly ML
[PASS] clustering
[PASS] flow detection
[PASS] risk
[PASS] evidence
[PASS] alerts
[PASS] explain
[PASS] report object
[PASS] offline inference
```

---

# 46. Final TUI Gate

TUI is complete when:

```text
[PASS] startup
[PASS] navigation
[PASS] search
[PASS] wallet
[PASS] transaction
[PASS] graph
[PASS] timeline
[PASS] alerts
[PASS] evidence
[PASS] monitoring
[PASS] reports
[PASS] offline state
[PASS] keyboard help
```

---

# 47. Final Distribution Gate

A clean Linux machine must be able to:

```bash
curl -fsSL https://raw.githubusercontent.com/<ORG>/bctx/main/scripts/install.sh | sh
```

then:

```bash
bctx
```

and:

```bash
bctx doctor
```

must pass.

After disabling networking:

```bash
bctx analyze wallet W123 --offline
bctx explain wallet W123
bctx report wallet W123
```

must still work for available local evidence.

---

# 48. Final Definition of Done

BCTX is done when an analyst can perform this complete lifecycle:

```text
INSTALL
  ↓
START
  ↓
CREATE CASE
  ↓
IMPORT / SYNC DATA
  ↓
ANALYZE WALLET
  ↓
BUILD GRAPH
  ↓
RUN LOCAL ML
  ↓
DETECT PATTERNS
  ↓
CALCULATE RISK
  ↓
GENERATE EVIDENCE
  ↓
VIEW ALERT
  ↓
EXPLAIN
  ↓
MONITOR WHILE CONNECTED
  ↓
PERSIST NEW EVENTS
  ↓
DISCONNECT NETWORK
  ↓
CONTINUE LOCAL ANALYSIS
  ↓
SUMMARIZE
  ↓
GENERATE REPORT
  ↓
EXPORT PDF/JSON/HTML
```

---

# 49. Priority Rules for Agents

When implementing tasks:

```text
P0 — correctness and backend
P1 — offline guarantees
P2 — evidence and explainability
P3 — CLI usability
P4 — TUI
P5 — advanced ML
P6 — visual polish
```

Never sacrifice:

```text
data correctness
evidence traceability
offline behavior
testability
```

for visual features.

---

# 50. Final Build Philosophy

BCTX should be developed as:

```text
A serious local intelligence engine
        ↓
exposed through a terminal
        ↓
distributed as a native Linux tool
        ↓
with optional connected acquisition
        ↓
and complete offline intelligence
```

The terminal is the front door.

The local evidence store is the memory.

The graph is the relationship map.

The ML engine is the pattern detector.

The risk engine is the prioritization layer.

The evidence engine is the explanation layer.

The report engine is the output layer.

The offline boundary is the defining system property.
