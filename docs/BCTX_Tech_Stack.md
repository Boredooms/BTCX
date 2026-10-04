# BCTX — Technology Stack

**Product:** BCTX — Bitcoin Forensic Intelligence Platform  
**Architecture:** Offline-first, local-first, terminal-centric  
**Primary OS:** Linux  
**Primary Runtime:** Go  
**Primary UX:** CLI + Interactive TUI  
**ML Training:** Python  
**ML Deployment:** ONNX Runtime  
**Storage:** SQLite + optional DuckDB  
**Distribution:** Native Linux binary + local model/data bundles

---

# 1. Technology Stack Philosophy

BCTX is not being designed as a conventional web application.

The core product must behave like a native terminal investigation tool:

```text
$ bctx
```

The architecture therefore prioritizes:

- offline execution;
- fast startup;
- native Linux deployment;
- low operational complexity;
- local data ownership;
- deterministic analysis;
- explainable ML;
- strong graph analytics;
- easy binary distribution;
- modular extensibility.

The end-user should not need to install a full application platform such as Node.js, Python, PostgreSQL, Redis, Docker, or Kubernetes just to run BCTX.

---

# 2. Stack at a Glance

| Layer | Technology | Purpose |
|---|---|---|
| Core application | Go | Main runtime and orchestration |
| CLI | Go + Cobra | Command-line interface |
| TUI | Go + Bubble Tea ecosystem | Interactive terminal experience |
| Terminal styling | Lip Gloss | Layout and styling |
| TUI components | Bubbles | Lists, tables, spinners, inputs, etc. |
| Storage | SQLite | Embedded operational database |
| Analytics | DuckDB | Optional large analytical queries |
| Query layer | SQL | Local structured querying |
| Graph engine | Custom Go graph subsystem | Wallet/TX/IP/entity relationships |
| Graph persistence | SQLite/DuckDB tables + indexes | Durable local graph representation |
| ML research | Python | Training and experimentation |
| ML models | PyTorch / scikit-learn | Model development |
| ML deployment | ONNX | Portable model format |
| Local inference | ONNX Runtime | Offline inference |
| Data ingestion | Go streaming parsers | CSV/JSON/XML ingestion |
| Data validation | Go schema/validation layer | Input integrity |
| Configuration | TOML | Local configuration |
| Serialization | JSON | APIs, exports, model metadata |
| Reports | Go templates + rendering pipeline | Local report generation |
| PDF | Local PDF generation tool/library | Offline PDF export |
| HTML | Go templates | Offline HTML reports |
| Logging | Go structured logging | Local diagnostics/audit |
| Testing | Go test + Python test tooling | Unit/integration/model testing |
| Packaging | Go build / release pipeline | Native binary releases |
| Integrity | SHA-256/signatures | Binary/model/data verification |
| GeoIP | Local MMDB-compatible database | Offline IP metadata lookup |
| Version control | Git | Source control |
| CI/CD | GitHub Actions or equivalent | Build/test/release automation |

---

# 3. Core Runtime — Go

## Why Go?

Go is the recommended primary implementation language for BCTX.

It fits the product because BCTX is fundamentally a systems-oriented terminal application rather than a browser application.

### Required characteristics

```text
Fast startup
Static/portable binaries
Linux support
Concurrency
Streaming data processing
Filesystem access
Database integration
Process management
Networking for acquisition adapters
CLI tooling
TUI tooling
```

### Go owns

```text
CLI
TUI
Command dispatcher
Workflow orchestration
Data ingestion
Normalization
Local storage
Indexes
Graph engine
Feature extraction
ML inference integration
Risk engine
Evidence engine
Monitoring
Reporting
Case management
Configuration
Security controls
```

---

# 4. CLI — Cobra

Recommended:

**Cobra**

Purpose:

```text
bctx
├── analyze
├── monitor
├── wallet
├── transaction
├── graph
├── alerts
├── explain
├── report
├── dataset
├── case
├── model
├── sync
├── status
└── doctor
```

Example:

```bash
bctx analyze wallet W123
```

Cobra should provide command parsing, flags, help, subcommands, and completion.

---

# 5. Interactive TUI — Bubble Tea

Recommended:

**Bubble Tea**

BCTX's main interactive environment should be a terminal UI rather than a browser dashboard.

Concept:

```text
CLI command
     ↓
investigation state
     ↓
Bubble Tea TUI
```

The TUI should provide:

```text
Dashboard
Wallet details
Transaction details
Alert queue
Graph view
Timeline
Network/IP activity
Risk information
Evidence
Reports
Monitoring status
```

Example:

```text
┌─────────────────────────────────────────────────────────────┐
│ BCTX                         OFFLINE                         │
├─────────────────┬───────────────────────────────────────────┤
│ ALERTS          │            TRANSACTION GRAPH              │
│                 │                                           │
│ W123  94        │              W101                         │
│ TX91  91        │                │                          │
│ W441  89        │               TX1                         │
│                 │              /  \                         │
│                 │            W123 W441                      │
├─────────────────┴───────────────────────────────────────────┤
│ EVIDENCE                                                   │
└─────────────────────────────────────────────────────────────┘
```

---

# 6. Terminal Styling — Lip Gloss

Recommended:

**Lip Gloss**

Used for:

- panel styling;
- borders;
- status indicators;
- selected states;
- tables;
- alert presentation;
- consistent visual hierarchy.

BCTX should support:

```text
normal
warning
high-risk
error
success
offline
connected
```

without requiring a browser.

---

# 7. TUI Components — Bubbles

Recommended components where applicable:

```text
list
table
viewport
spinner
text input
progress
key bindings
pagination
```

Custom components can be built for:

```text
graph visualization
risk meter
timeline
evidence tree
investigation panels
```

---

# 8. Storage — SQLite

## Primary database

**SQLite**

This is the default BCTX operational database.

Why:

```text
Embedded
No database server
Single local file
Transaction support
Reliable
Easy backup
Excellent Linux fit
Offline by design
```

Example:

```text
~/.bctx/cases/case-001/case.db
```

The user should not need:

```text
PostgreSQL
MySQL
MongoDB
Redis
```

for the core application.

---

# 9. Optional Analytics Engine — DuckDB

**DuckDB** should be introduced when large analytical operations benefit from a columnar analytical database.

Use it for:

```text
large aggregations
feature engineering
dataset profiling
statistical analysis
bulk historical queries
model evaluation
```

Possible architecture:

```text
SQLite
↓
operational metadata

DuckDB
↓
large analytical workloads
```

DuckDB is optional for the first MVP.

---

# 10. Why Not PostgreSQL?

A server-based PostgreSQL deployment would introduce:

```text
database server
connection management
credentials
service lifecycle
installation
configuration
```

That works for enterprise deployments but conflicts with the initial product goal of:

```text
download binary
↓
run bctx
↓
offline
```

SQLite removes that operational burden.

A future enterprise deployment can expose a different storage backend without changing the core domain interfaces.

---

# 11. Why Not Redis in the MVP?

Redis is not required for the offline-first single-machine architecture.

Potential future uses:

```text
distributed monitoring
multi-user deployments
high-speed shared cache
distributed job coordination
```

For the MVP:

```text
Go memory
+
SQLite
+
local queues
```

are sufficient.

---

# 12. Graph Technology

## Recommended approach

Do not introduce Neo4j as a mandatory dependency for the MVP.

Build a **local graph subsystem in Go** over persisted relational data.

Logical graph:

```text
Wallet
Transaction
IP
Entity
```

Relationships:

```text
Wallet → Transaction
Transaction → Wallet
IP → Transaction
Wallet → Entity
```

Store durable graph relationships in tables/indexes such as:

```text
nodes
edges
wallet_transactions
transaction_outputs
network_observations
entity_members
```

Then use Go algorithms for:

```text
BFS
DFS
N-hop traversal
shortest path
component discovery
risk propagation
subgraph extraction
degree analysis
```

---

# 13. Why Not Neo4j Initially?

Neo4j is a valid future option but would make the offline product more complex because it introduces another server/runtime dependency.

The MVP objective is:

```text
bctx
↓
one local application
↓
one local case store
```

A native graph layer provides more deployment control.

A graph backend interface should still be defined so Neo4j or another graph engine could be added later.

---

# 14. Data Ingestion Stack

Supported source formats:

```text
CSV
JSON
XML
```

The ingestion layer should be implemented in Go.

Design:

```text
File
 ↓
Streaming parser
 ↓
Validation
 ↓
Normalization
 ↓
Batch persistence
 ↓
Index update
 ↓
Graph update
```

Avoid:

```go
os.ReadFile(...)
```

for multi-gigabyte datasets.

Use streaming readers and batched database operations.

---

# 15. Data Schema

The internal normalized schema should support the challenge fields:

```text
timestamp
src_ip
src_port
dst_ip
dst_port
txid
input_addresses
output_addresses
input_amounts
output_amounts
fee
script_type
country
ASN
```

The external input format should not leak into internal domain logic.

Example:

```text
CSV
JSON
XML
  ↓
Normalizer
  ↓
Canonical Transaction model
```

This allows different providers and datasets to feed the same analysis engine.

---

# 16. Data Validation

Use native Go validation logic.

Validation should cover:

```text
required fields
valid timestamps
valid transaction identifiers
wallet/address format where applicable
amount consistency
array lengths
IP address syntax
port ranges
duplicate records
malformed records
```

Malformed data must be isolated rather than crashing an entire import.

---

# 17. Data Acquisition Stack

The acquisition layer is the only part of BCTX that may require network access.

Use an adapter pattern:

```text
DataSource
├── ExplorerAdapter
├── FileAdapter
├── SyntheticDatasetAdapter
└── FutureAdapter
```

The analysis engine must never directly call provider APIs.

Correct:

```text
Provider API
 ↓
Acquisition Adapter
 ↓
Local canonical data
 ↓
Analysis
```

Incorrect:

```text
Risk Engine
 ↓
Provider API
```

---

# 18. HTTP Client

Go's native HTTP client should be sufficient for acquisition adapters initially.

Requirements:

```text
timeouts
retries
pagination
rate limiting
context cancellation
HTTP status handling
response validation
```

External provider credentials, where required, must be stored in local configuration and never committed to Git.

---

# 19. Live Monitoring Transport

For future/current monitoring providers, the acquisition layer may support:

```text
HTTP polling
WebSocket
provider-specific streaming APIs
```

The transport remains isolated from:

```text
graph
risk
ML
TUI
```

So the monitor pipeline is:

```text
transport
 ↓
normalized event
 ↓
local event store
 ↓
analysis
```

---

# 20. Python ML Stack

Python is the recommended language for ML research.

Python owns:

```text
dataset preparation
feature experimentation
model training
model evaluation
visual analysis
model export
```

Recommended libraries:

```text
pandas
NumPy
scikit-learn
PyTorch
NetworkX
Jupyter for experimentation
ONNX / ONNX export tooling
```

Not all libraries need to ship with BCTX.

They primarily belong to the model development environment.

---

# 21. ML Model Strategy

BCTX should start with one strong model and expand.

Recommended progression:

```text
Phase 1
Anomaly detection

Phase 2
Entity clustering

Phase 3
Flow/pattern detection

Phase 4
Advanced graph representation learning
```

Possible techniques:

### Anomaly detection

```text
Isolation Forest
Autoencoder
One-Class models
other validated anomaly approaches
```

### Entity clustering

```text
feature-based clustering
graph embeddings
community detection
```

### Graph models

Potential future:

```text
Node2Vec
GraphSAGE
GNN
```

The exact model should be selected based on evaluation rather than technology branding.

---

# 22. Model Deployment — ONNX

Recommended production model format:

**ONNX**

Training:

```text
Python
 ↓
train model
 ↓
evaluate
 ↓
export .onnx
```

Runtime:

```text
BCTX
 ↓
ONNX Runtime
 ↓
local model
```

Advantages:

```text
No Python dependency
Portable
Local
Fast inference
Versionable
Easy packaging
```

---

# 23. ONNX Runtime

The Go runtime should call the local inference engine.

Conceptually:

```text
Go feature vector
       ↓
ONNX Runtime
       ↓
anomaly.onnx
       ↓
score
```

The production binary must not download a model.

Models are packaged with or explicitly imported into the local BCTX installation.

---

# 24. Optional Local LLM

A local LLM is an optional advanced component.

Potential purpose:

```text
evidence summarization
report prose
natural-language investigation
question answering over local evidence
```

It should never replace the primary analytical pipeline.

Correct:

```text
graph/model/risk
       ↓
structured evidence
       ↓
local LLM
       ↓
summary
```

Incorrect:

```text
raw transactions
       ↓
LLM
       ↓
risk score
```

The risk score must come from the deterministic/ML analysis layer.

---

# 25. Feature Engineering

Features are generated locally.

Candidate features:

```text
transaction count
incoming count
outgoing count
incoming volume
outgoing volume
average transaction value
value variance
transaction frequency
time gaps
fan-in
fan-out
wallet degree
counterparty diversity
hop count
path length
cluster membership
graph centrality
network observation count
```

The feature schema should be versioned.

Example:

```text
feature-schema-v1
feature-schema-v2
```

This prevents model/feature incompatibility.

---

# 26. Risk Engine

The risk engine is implemented in Go.

Inputs:

```text
anomaly score
pattern score
cluster score
graph signals
risk propagation
network correlation
historical behavior
```

Outputs:

```text
risk_score
confidence
signal breakdown
```

Example:

```json
{
  "risk_score": 91,
  "confidence": 0.93,
  "signals": [
    {
      "name": "transaction_anomaly",
      "score": 0.93
    },
    {
      "name": "peeling_pattern",
      "score": 0.88
    }
  ]
}
```

---

# 27. Evidence Engine

Implemented in Go.

Its responsibility:

```text
collect evidence
normalize evidence
link evidence to source records
link evidence to model outputs
produce explanations
```

Example evidence:

```text
TX123
↓
anomalous velocity

TX124
↓
part of peeling chain

W123
↓
cluster #18
```

The evidence engine makes the report auditable.

---

# 28. Reporting Stack

## Markdown

Generated directly by Go templates.

## JSON

Generated using Go's standard JSON tooling.

## HTML

Generated using Go `html/template`.

## PDF

Use a local PDF generation library/toolchain.

Important:

```text
report generation
=
100% local
```

No SaaS PDF generation service.

---

# 29. GeoIP Stack

If offline IP geolocation is required:

```text
IP
 ↓
local MMDB database
 ↓
country / ASN metadata
```

Use a locally bundled or locally imported GeoIP database.

Possible implementation:

```text
MaxMind DB compatible reader
```

The database should be separately versioned and checksum-verified.

---

# 30. Configuration Stack

Recommended format:

**TOML**

Example:

```toml
[app]
data_dir = "~/.bctx"
default_case = "case-001"

[models]
directory = "~/.bctx/models"

[monitor]
default_interval = "30s"

[reports]
default_format = "pdf"

[network]
acquisition_enabled = true
```

Configuration should be loaded locally.

---

# 31. Logging

Use structured local logging.

Example:

```text
2026-10-02T12:05:02Z INFO  wallet.analysis started wallet=W123
2026-10-02T12:05:03Z INFO  graph.expansion completed nodes=842 edges=1921
2026-10-02T12:05:04Z INFO  model.anomaly score=0.93
2026-10-02T12:05:04Z INFO  risk.updated previous=81 current=91
```

Logs belong in:

```text
~/.bctx/logs/
```

No telemetry should be required.

---

# 32. Audit Logging

Separate application logs from investigation audit events.

Audit events:

```text
DATASET_IMPORTED
WALLET_ANALYZED
TRANSACTION_ANALYZED
GRAPH_EXPANDED
MODEL_EXECUTED
RISK_UPDATED
ALERT_CREATED
REPORT_GENERATED
REPORT_EXPORTED
MONITOR_STARTED
MONITOR_STOPPED
```

Each event should contain:

```text
timestamp
case
action
subject
operator/session
result
```

---

# 33. Process Architecture

Initial deployment should be one primary BCTX process.

```text
bctx
├── CLI
├── TUI
├── command engine
├── storage
├── graph
├── ML
├── risk
├── evidence
├── monitor
└── reports
```

Optional workers can run inside the same process.

Separate subprocesses should be introduced only where they solve a real problem such as ML resource isolation.

---

# 34. Background Worker Architecture

Use Go goroutines and channels for:

```text
data ingestion
monitor events
graph updates
ML tasks
report generation
TUI updates
```

Conceptually:

```text
Event Queue
   ↓
Worker
   ↓
Analysis
   ↓
Result Event
   ↓
TUI
```

Long-running operations must not freeze the TUI.

---

# 35. Local Event Bus

A lightweight in-process event system can be used.

Events:

```text
TransactionReceived
DatasetImported
GraphUpdated
RiskUpdated
AlertCreated
AnalysisStarted
AnalysisCompleted
ReportGenerated
MonitorStateChanged
```

This allows the TUI and background workers to stay decoupled.

---

# 36. Case Storage Architecture

Each investigation should have isolated local state.

```text
~/.bctx/
└── cases/
    └── case-001/
        ├── case.db
        ├── evidence/
        ├── reports/
        ├── exports/
        ├── imports/
        ├── logs/
        └── metadata.json
```

This supports multiple independent investigations.

---

# 37. Model Storage

Local model directory:

```text
~/.bctx/models/
├── anomaly/
│   ├── model.onnx
│   └── manifest.json
├── entity/
│   ├── model.onnx
│   └── manifest.json
└── flow/
    ├── model.onnx
    └── manifest.json
```

BCTX should validate:

```text
model version
feature schema
input schema
checksum
runtime compatibility
```

before use.

---

# 38. Offline Boundary

The most important technology boundary is:

```text
                 NETWORK
                    │
                    ▼
             Acquisition Layer
                    │
                    ▼
           ─── OFFLINE BOUNDARY ───
                    │
                    ▼
              Local Data
                    │
           ┌────────┼────────┐
           ▼        ▼        ▼
         Graph     ML      Storage
           │        │        │
           └────────┼────────┘
                    ▼
                 Risk
                    │
                 Evidence
                    │
              TUI / Reports
```

The components below the boundary must not require network access.

---

# 39. Network Security Model

Acquisition network access should be explicit.

When no acquisition is requested:

```text
Analysis
Graph
ML
Risk
Reports
```

must continue without making external network calls.

Possible runtime state:

```text
CONNECTED
DISCONNECTED
AIRGAPPED
```

The UI should expose the current state.

---

# 40. Installation Stack

The release should be distributed as a native binary.

Example:

```bash
curl -fsSL https://<domain>/install.sh | sh
```

Installer responsibilities:

```text
detect architecture
download binary
download/verify model bundle
verify checksums/signatures
install executable
initialize directories
run doctor
```

Target result:

```bash
bctx
```

No Node.js or Python setup required.

---

# 41. Fully Offline Installation

For truly air-gapped machines:

```text
Internet-connected machine
        ↓
download BCTX release bundle
        ↓
USB / controlled transfer
        ↓
offline Linux system
        ↓
local installer
```

The offline machine receives:

```text
binary
models
checksums
required auxiliary datasets
optional demo dataset
```

---

# 42. Build Toolchain

Development:

```text
Go
Go modules
Make or task runner
Git
Python virtual environment
```

Example:

```bash
make build
make test
make lint
make package
```

Production:

```bash
go build
```

with appropriate release flags.

---

# 43. Testing Stack

## Go

Use:

```text
go test
go vet
race detector
benchmarks
```

Test categories:

```text
unit
integration
storage
graph
CLI
TUI
offline
security
```

## Python

Use:

```text
pytest
```

for:

```text
feature engineering
model validation
data generation
model evaluation
```

---

# 44. Offline Test Stack

A dedicated test suite must verify:

```text
network disabled
       ↓
BCTX starts
       ↓
database works
       ↓
graph works
       ↓
ML works
       ↓
risk works
       ↓
explanation works
       ↓
report works
```

A successful offline test is a release requirement.

---

# 45. CI/CD

Use GitHub Actions or an equivalent CI system.

Pipeline:

```text
Commit
 ↓
Go formatting
 ↓
Go tests
 ↓
Static analysis
 ↓
Python tests
 ↓
ML validation
 ↓
Build Linux binaries
 ↓
Package model bundle
 ↓
Checksum/sign
 ↓
Release artifacts
```

CI itself is not part of the installed product.

---

# 46. Cross Compilation

Primary target:

```text
Linux amd64
```

Secondary:

```text
Linux arm64
```

Potential future:

```text
Windows
macOS
```

Linux remains the primary supported platform because the challenge requires a Linux solution.

---

# 47. Dependency Philosophy

Minimize runtime dependencies.

Ideal production installation:

```text
bctx binary
+
model bundle
+
optional local GeoIP DB
+
user datasets
```

Avoid requiring:

```text
Docker
Kubernetes
PostgreSQL
Redis
Node.js
Python
cloud runtime
```

for core operation.

---

# 48. Data Import and Export Formats

### Import

```text
.csv
.json
.xml
```

### Internal

```text
SQLite
DuckDB
JSON metadata
```

### Export

```text
.json
.md
.html
.pdf
```

---

# 49. API / SDK Architecture

The core Go packages should be usable independently of the CLI.

Conceptual:

```go
engine := bctx.NewEngine(config)

result, err := engine.InvestigateWallet(ctx, "W123")
```

SDK modules:

```text
Data
Graph
Features
ML
Risk
Evidence
Reports
Cases
```

This allows:

```text
CLI
TUI
future API
future integrations
```

to share the same core.

---

# 50. Recommended Repository Stack

```text
bctx/
├── cmd/
│   └── bctx/
├── internal/
│   ├── cli/
│   ├── tui/
│   ├── command/
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
│   ├── monitoring/
│   ├── reports/
│   ├── cases/
│   ├── audit/
│   └── security/
├── pkg/
│   ├── schema/
│   └── sdk/
├── models/
├── migrations/
├── scripts/
├── tests/
└── docs/
```

---

# 51. Technology-to-Feature Mapping

| Product Feature | Main Technology |
|---|---|
| `bctx` executable | Go |
| CLI commands | Cobra |
| Interactive terminal dashboard | Bubble Tea |
| Terminal styling | Lip Gloss |
| Large input ingestion | Go streaming I/O |
| CSV parsing | Go CSV package |
| JSON parsing | Go JSON |
| XML parsing | Go XML |
| Local database | SQLite |
| Analytical processing | DuckDB |
| Wallet indexing | SQLite indexes / custom indexes |
| Graph traversal | Go |
| Entity clustering | Python ML + graph algorithms |
| Anomaly detection | Python-trained model + ONNX |
| Flow detection | Python ML/algorithm + ONNX where appropriate |
| Risk scoring | Go |
| Risk propagation | Go graph algorithms |
| Explainability | Go Evidence Engine |
| Local inference | ONNX Runtime |
| Local GeoIP | MMDB |
| Monitoring | Go goroutines + adapter layer |
| Local reports | Go templates + PDF library |
| Config | TOML |
| Logs | Go structured logger |
| Audit trail | SQLite |
| Binary distribution | Go release build |
| Offline install | local bundle |
| CI/CD | GitHub Actions/equivalent |
| Testing | Go test + pytest |

---

# 52. What the End User Actually Installs

The ideal user experience is:

```text
BCTX RELEASE

bctx
models/
geo/
checksums
```

After installation:

```bash
bctx
```

starts the complete application.

The user should not see:

```text
npm install
pip install
docker compose
postgres setup
redis setup
```

as required product steps.

---

# 53. Final Recommended Stack

## Core

```text
Go
```

## CLI

```text
Cobra
```

## TUI

```text
Bubble Tea
Bubbles
Lip Gloss
```

## Data

```text
SQLite
DuckDB (optional)
```

## Graph

```text
Custom Go graph engine
SQLite/DuckDB-backed graph data
```

## ML

```text
Python
scikit-learn
PyTorch
NumPy
pandas
```

## ML deployment

```text
ONNX
ONNX Runtime
```

## Acquisition

```text
Go HTTP client
WebSocket support where required
adapter architecture
```

## GeoIP

```text
Local MMDB database
```

## Reports

```text
Go templates
JSON
HTML
Markdown
Local PDF renderer
```

## Configuration

```text
TOML
```

## Testing

```text
Go test
pytest
integration tests
offline tests
benchmark tests
```

## Distribution

```text
Native Linux binary
Model bundle
Optional data/GeoIP bundles
Checksums/signatures
```

---

# 54. Final Architecture Stack

```text
                         BCTX
                          │
             ┌────────────┴────────────┐
             │                         │
          Cobra                     Bubble Tea
           CLI                         TUI
             │                         │
             └────────────┬────────────┘
                          ▼
                  Go Core / SDK
                          │
      ┌───────────────────┼───────────────────┐
      ▼                   ▼                   ▼
 Acquisition          Investigation       Case Mgmt
      │                   │                   │
      ▼                   ▼                   ▼
 Normalizer           Graph Engine        SQLite
      │                   │                   │
      └───────────────────┼───────────────────┘
                          ▼
                   Feature Engine
                          │
                          ▼
                   ONNX Runtime
                          │
                    Local Models
                          │
                          ▼
                     Risk Engine
                          │
                          ▼
                   Evidence Engine
                          │
                ┌─────────┴─────────┐
                ▼                   ▼
              TUI                Reports
                                  │
                         ┌────────┼────────┐
                         ▼        ▼        ▼
                        PDF      HTML     JSON
```

---

# 55. Final Technology Principle

The final BCTX stack should follow this rule:

> **Use Go for the product, Python for model development, ONNX for portable local inference, SQLite for embedded persistence, and a terminal-native TUI for the investigator experience.**

The architecture should remain:

```text
simple to install
simple to run
simple to isolate
difficult to accidentally depend on the internet
```

The final runtime target is:

```bash
$ bctx

BCTX — Bitcoin Forensic Intelligence Platform

Network: OFFLINE
Database: READY
Models: READY
Graph: READY

bctx>
```

with the core investigation path operating entirely from local data and local computation.
