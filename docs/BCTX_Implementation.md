# BCTX — Implementation Plan

**Product:** BCTX — Bitcoin Forensic Intelligence Platform  
**Document:** Implementation Plan / Engineering Execution Specification  
**Target Platform:** Linux  
**Primary Runtime:** Go  
**Primary Interface:** CLI + Interactive TUI  
**ML Development:** Python  
**ML Runtime:** ONNX Runtime  
**Primary Storage:** SQLite  
**Optional Analytics Storage:** DuckDB  
**Architecture:** Offline-first, local-first, modular monolith

---

# 1. Purpose

This document translates the BCTX product and architecture decisions into an actionable engineering plan.

The goal is to define:

- what to build;
- in what order to build it;
- how the modules interact;
- what data structures are required;
- how the offline boundary is enforced;
- how AI/ML is trained and deployed;
- how monitoring works;
- how reports are generated;
- how the application is packaged;
- how the implementation is tested;
- what should be considered MVP versus later work.

The implementation should be incremental. We should not start by building the entire system simultaneously.

The recommended sequence is:

```text
Foundation
    ↓
Data Layer
    ↓
Graph Layer
    ↓
Feature Engine
    ↓
ML
    ↓
Risk + Evidence
    ↓
Investigation Commands
    ↓
TUI
    ↓
Monitoring
    ↓
Reports
    ↓
Offline Hardening
    ↓
Packaging
```

---

# 2. Implementation Principles

## 2.1 Build the core before the UI

The TUI is important, but the investigation engine must work independently.

Correct dependency:

```text
Core Engine
    ↓
CLI
    ↓
TUI
```

not:

```text
TUI
    ↓
business logic
```

This lets us test everything from commands before implementing the full interface.

---

## 2.2 Separate acquisition from intelligence

Network access must be isolated.

```text
                    NETWORK
                       │
                       ▼
              Acquisition Layer
                       │
                       ▼
                 Local Storage
                       │
            ─── OFFLINE BOUNDARY ───
                       │
                       ▼
          Graph + ML + Risk + Reports
```

Once data is local, the intelligence layer must not require external network access.

---

## 2.3 Store evidence before deriving intelligence

Raw/normalized evidence must be persisted before complex analysis.

```text
External record
      ↓
Normalize
      ↓
Persist
      ↓
Index
      ↓
Analyze
```

This ensures that analysis can be reproduced later.

---

## 2.4 Deterministic analysis is the source of truth

The risk score must come from:

```text
data
+
features
+
graph signals
+
ML outputs
+
risk logic
```

An optional local LLM may summarize those findings, but it must not invent the underlying evidence.

---

## 2.5 Everything should be incremental

Large data should not require complete recomputation whenever possible.

Prefer:

```text
new data
  ↓
incremental persistence
  ↓
incremental graph update
  ↓
incremental features
  ↓
targeted ML
  ↓
risk update
```

---

# 3. Implementation Phases

| Phase | Name | Primary Outcome |
|---|---|---|
| 0 | Project Foundation | Compilable BCTX binary |
| 1 | Storage & Schema | Local case/database layer |
| 2 | Data Ingestion | CSV/JSON/XML import |
| 3 | Indexing & Search | Fast wallet/TX/IP lookups |
| 4 | Graph Engine | Wallet/TX/IP/entity graph |
| 5 | Feature Engine | Investigation features |
| 6 | ML Pipeline | Local anomaly model |
| 7 | Entity & Pattern Detection | Clustering + flow detection |
| 8 | Risk & Evidence | Explainable alerts |
| 9 | CLI Investigation Shell | Command-driven investigation |
| 10 | TUI | Interactive terminal workstation |
| 11 | Monitoring | Incremental wallet monitoring |
| 12 | Reporting | PDF/HTML/JSON/Markdown reports |
| 13 | Offline Hardening | Verified no-network analysis |
| 14 | Packaging | Installable Linux binary |
| 15 | SIH Demo | End-to-end polished prototype |

---

# 4. Phase 0 — Project Foundation

## Objective

Create the initial Go project and make:

```bash
bctx
```

work.

## Repository

```text
bctx/
├── cmd/
│   └── bctx/
│       └── main.go
├── internal/
├── pkg/
├── models/
├── migrations/
├── scripts/
├── tests/
└── docs/
```

## Initial Go setup

```bash
go mod init github.com/<org>/bctx
```

Add the CLI/TUI dependencies.

## First command

```bash
bctx version
```

Output:

```text
BCTX v0.1.0
Platform: linux/amd64
Mode: offline
```

## Acceptance Criteria

- binary compiles;
- executable starts;
- `bctx help` works;
- version is shown;
- configuration directory can be initialized.

---

# 5. Phase 1 — Configuration and Runtime Initialization

Create:

```text
~/.bctx/
```

Structure:

```text
~/.bctx/
├── config/
├── cases/
├── models/
├── geo/
├── cache/
├── logs/
└── tmp/
```

Add:

```bash
bctx init
```

Expected:

```text
BCTX initialization complete.

Data directory: ~/.bctx
Models:         ~/.bctx/models
Cases:          ~/.bctx/cases
```

Implement:

```text
ConfigLoader
PathResolver
EnvironmentResolver
```

Configuration should support:

- data directory;
- model directory;
- current case;
- monitor settings;
- report defaults;
- acquisition settings.

---

# 6. Phase 2 — Storage and Schema

## Objective

Create the local data foundation.

Start with SQLite.

Database:

```text
case.db
```

## Core tables

Recommended initial tables:

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

---

# 7. Transaction Schema

Canonical transaction model:

```go
type Transaction struct {
    TXID        string
    Timestamp   time.Time
    Inputs      []TransactionInput
    Outputs     []TransactionOutput
    FeeBTC      float64
    ScriptType  string
}
```

Inputs:

```go
type TransactionInput struct {
    Address  string
    AmountBTC float64
}
```

Outputs:

```go
type TransactionOutput struct {
    Address  string
    AmountBTC float64
}
```

Network observation:

```go
type NetworkObservation struct {
    Timestamp time.Time
    SrcIP     string
    SrcPort   int
    DstIP     string
    DstPort   int
}
```

The source problem statement specifies transaction/network fields including timestamp, source/destination IPs and ports, TXID, input/output addresses, amounts, fee, script type, and country/ASN metadata.

---

# 8. Database Design Rules

Use:

```text
primary keys
foreign keys
indexes
transactions
```

Important indexes:

```sql
CREATE INDEX idx_transactions_txid
ON transactions(txid);

CREATE INDEX idx_inputs_address
ON transaction_inputs(address);

CREATE INDEX idx_outputs_address
ON transaction_outputs(address);

CREATE INDEX idx_network_src_ip
ON network_observations(src_ip);

CREATE INDEX idx_network_dst_ip
ON network_observations(dst_ip);

CREATE INDEX idx_transactions_timestamp
ON transactions(timestamp);
```

Additional compound indexes should be added after query profiling.

---

# 9. Phase 3 — Data Ingestion

## Objective

Support:

```bash
bctx dataset import data.csv
bctx dataset import data.json
bctx dataset import data.xml
```

## Import pipeline

```text
Input File
    ↓
Format Detection
    ↓
Streaming Parser
    ↓
Validation
    ↓
Normalization
    ↓
Batch Insert
    ↓
Index Update
    ↓
Graph Update
```

---

# 10. Streaming Ingestion

Do not load entire large datasets into memory.

For CSV:

```text
reader
  ↓
record
  ↓
normalize
  ↓
batch
  ↓
database
```

Use batches such as:

```text
1,000
5,000
10,000
```

and benchmark the appropriate size.

---

# 11. Import Error Handling

A malformed record should not necessarily terminate the whole import.

Maintain:

```text
valid_records
invalid_records
warnings
```

Example:

```text
Importing case.csv

Records read: 2,481,392
Valid:        2,475,820
Rejected:         5,572

Errors written to:
~/.bctx/cases/case-001/imports/errors.jsonl
```

---

# 12. Import Resumability

For large files, support resumable ingestion.

Persist:

```text
source file hash
byte offset or parser checkpoint
records processed
last successful commit
```

Command:

```bash
bctx dataset resume
```

This should be implemented after basic ingestion is stable.

---

# 13. Phase 4 — Indexing and Search

## Objective

Make common investigations interactive.

Required lookup paths:

```text
wallet → transactions
txid → transaction
IP → observations
entity → wallets
cluster → members
transaction → connected wallets
```

Commands:

```bash
bctx wallet W123
bctx transaction TX123
bctx ip 10.1.2.3
```

---

# 14. Search Engine

Start with database indexes rather than a separate search server.

Use SQL queries for:

```text
exact wallet lookup
exact TXID lookup
exact IP lookup
time range
amount range
risk range
cluster lookup
```

A separate full-text engine is not required for MVP.

---

# 15. Phase 5 — Graph Engine

## Objective

Represent investigation relationships as a graph.

Conceptual nodes:

```text
Wallet
Transaction
IP
Entity
```

Edges:

```text
Wallet → Transaction
Transaction → Wallet
IP → Transaction
Wallet → Entity
```

---

# 16. Graph Data Model

Example:

```text
W123
 │
 ├──TX001──→W456
 │
 ├──TX009──→W789
 │
 └──TX019──→W111
```

Persist graph edges locally.

Example:

```text
graph_edges
------------
source_type
source_id
edge_type
target_type
target_id
timestamp
weight
metadata
```

---

# 17. Graph Operations

Implement first:

```text
neighbors(id)
subgraph(id, depth)
path(source, destination)
degree(id)
transaction_count(id)
```

Then:

```text
connected_components
cluster_members
centrality
risk_propagation
temporal subgraphs
```

---

# 18. N-Hop Graph Traversal

Command:

```bash
bctx graph wallet W123 --depth 3
```

Implementation:

```text
start node
   ↓
BFS
   ↓
depth 1
   ↓
depth 2
   ↓
depth 3
```

Return a bounded subgraph.

Never expand the entire graph by default.

---

# 19. Graph Rendering for TUI

The TUI graph should work on a selected subgraph.

Pipeline:

```text
query graph
   ↓
nodes + edges
   ↓
layout
   ↓
terminal renderer
```

For large graphs:

```text
collapse low-value nodes
limit depth
limit node count
filter by risk
```

---

# 20. Phase 6 — Feature Engine

## Objective

Convert raw graph/data information into ML-ready features.

Create:

```text
internal/features/
```

Example:

```go
type WalletFeatures struct {
    TxCount             float64
    IncomingVolume      float64
    OutgoingVolume      float64
    TxVelocity          float64
    FanIn               float64
    FanOut              float64
    CounterpartyCount   float64
    AverageTimeGap      float64
    Degree              float64
}
```

---

# 21. Feature Categories

## Transaction Features

```text
count
volume
fee
average amount
amount variance
input/output ratio
```

## Temporal Features

```text
transactions per minute
transactions per hour
median time gap
burstiness
```

## Graph Features

```text
degree
fan-in
fan-out
depth
neighbor count
centrality
cluster membership
```

## Flow Features

```text
number of hops
value decay
split ratio
merge ratio
chain length
```

## Network Features

```text
network observation count
unique IP count
unique ASN count
time correlation
```

---

# 22. Feature Versioning

Each feature vector must identify its schema version.

Example:

```text
feature_schema = v1
```

Store this alongside model predictions.

This avoids running:

```text
model trained on v1
```

against:

```text
features v3
```

without compatibility checks.

---

# 23. Phase 7 — ML Development

## Objective

Create a real ML pipeline, not just rule-based thresholds.

Recommended progression:

```text
Anomaly Detection
      ↓
Entity Clustering
      ↓
Flow/Pattern Detection
```

The challenge requires a working AI/ML detection use case rather than rules alone.

---

# 24. ML Development Environment

Python:

```text
python
pandas
numpy
scikit-learn
PyTorch
NetworkX
ONNX
```

Project:

```text
ml/
├── data/
├── features/
├── notebooks/
├── training/
├── evaluation/
├── export/
└── models/
```

---

# 25. Synthetic Dataset Generator

Create controlled development data.

Example:

```text
normal wallets
normal transactions
normal flows
```

plus synthetic patterns:

```text
rapid movement
fan-out
peeling chains
mixing-like structures
clustered wallets
unusual transaction timing
```

Each synthetic scenario should have development-only ground truth.

Example:

```json
{
  "scenario": "peeling_chain",
  "wallet": "W9001",
  "label": "pattern_1"
}
```

---

# 26. Anomaly Detection Model

Start with a simple robust baseline.

Candidate:

```text
Isolation Forest
```

Pipeline:

```text
wallet data
   ↓
feature vector
   ↓
scaler/transform if required
   ↓
Isolation Forest
   ↓
anomaly score
```

Output normalized to:

```text
0.0 → 1.0
```

Do not assume the first model is final. Benchmark alternatives.

---

# 27. Model Evaluation

Measure:

```text
precision
recall
F1
PR-AUC
ROC-AUC where appropriate
false positives
false negatives
inference latency
```

For synthetic imbalanced data, do not rely on accuracy alone.

Store evaluation artifacts.

---

# 28. Model Export

Export trained models:

```text
model.onnx
```

along with:

```text
manifest.json
```

Manifest:

```json
{
  "name": "anomaly-detector",
  "version": "1.0.0",
  "feature_schema": "v1",
  "runtime": "onnx"
}
```

---

# 29. Phase 8 — Local ML Runtime

The production BCTX application must run ML locally.

Pipeline:

```text
Go
 ↓
feature vector
 ↓
ONNX Runtime
 ↓
model.onnx
 ↓
prediction
```

No runtime download.

No remote inference.

No Python installation requirement.

---

# 30. Model Registry

BCTX should maintain a local model registry.

Command:

```bash
bctx model status
```

Example:

```text
MODELS

anomaly-detector    1.0.0    READY
entity-model       1.0.0    READY
flow-model         1.0.0    READY
```

Model compatibility should validate:

```text
model version
feature schema
runtime version
checksum
```

---

# 31. Phase 9 — Entity Clustering

## Objective

Group wallets that exhibit evidence of relationship.

Signals:

```text
common-input heuristic
behavior similarity
transaction timing
graph neighborhood
graph embeddings
```

MVP:

```text
heuristic + feature clustering
```

Later:

```text
graph embeddings
community detection
GNN
```

---

# 32. Cluster Output

Example:

```text
Cluster #18

Members:
W123
W456
W789
W821

Confidence:
0.87

Signals:
common-input relationship
graph similarity
behavior similarity
```

The wording should indicate inferred linkage, not proof of ownership.

---

# 33. Phase 10 — Suspicious Flow Detection

Implement a dedicated flow-analysis module.

```text
internal/patterns/
```

Initial detectors:

```text
PeelingChainDetector
FanOutDetector
FanInDetector
MixingLikeDetector
RapidFlowDetector
```

Each detector should return structured evidence:

```go
type PatternResult struct {
    Pattern     string
    Score       float64
    Confidence  float64
    EvidenceIDs []string
}
```

---

# 34. Peeling Chain Detection

Implementation concept:

```text
identify transaction chain
       ↓
measure repeated value split
       ↓
measure residual continuation
       ↓
measure time gap
       ↓
score pattern
```

Do not rely on one threshold.

Use multiple features.

---

# 35. Mixing-Like Detection

Identify graph signatures that resemble mixing structures.

Potential features:

```text
input count
output count
value distribution
participant count
transaction structure
timing
graph convergence/divergence
```

The detector should report:

```text
"mixing-like pattern detected"
```

rather than asserting criminal intent.

---

# 36. Phase 11 — Risk Engine

Implement:

```text
internal/risk/
```

Input:

```text
anomaly
cluster
pattern
graph
network
propagation
```

Output:

```go
type RiskAssessment struct {
    Score       float64
    Confidence  float64
    Signals     []RiskSignal
}
```

Normalize:

```text
score = 0..100
confidence = 0..1
```

---

# 37. Risk Signal Weighting

Initial formula can be configurable.

Conceptual:

```text
risk
 =
 anomaly contribution
 + pattern contribution
 + cluster contribution
 + graph contribution
 + propagation contribution
```

Weights should be stored in configuration/model metadata.

Do not present arbitrary weights as scientifically validated until evaluated.

---

# 38. Risk Propagation

Implement graph-based risk diffusion.

Start with a simple distance-decay approach.

Example:

```text
seed = 1.00

distance 1 = 0.70
distance 2 = 0.45
distance 3 = 0.25
```

Later evaluate:

```text
label propagation
personalized PageRank
graph diffusion
```

Every propagated result must retain the source path.

---

# 39. Phase 12 — Evidence Engine

Create:

```text
internal/evidence/
```

Evidence model:

```go
type EvidenceItem struct {
    ID          string
    Type        string
    Description string
    SourceIDs   []string
    Score       float64
    Confidence  float64
    CreatedAt   time.Time
}
```

Evidence types:

```text
transaction
graph
network
anomaly
pattern
cluster
risk_propagation
```

---

# 40. Explainability Pipeline

Command:

```bash
bctx explain wallet W123
```

Pipeline:

```text
Wallet
 ↓
retrieve assessment
 ↓
retrieve contributing signals
 ↓
retrieve evidence
 ↓
retrieve source transactions
 ↓
render explanation
```

The explanation must be traceable to actual data.

---

# 41. Phase 13 — Alert Manager

Create:

```text
internal/alerts/
```

Alert structure:

```text
alert_id
case_id
subject_type
subject_id
risk_score
confidence
priority
reason
evidence_ids
created_at
status
```

Generate alerts from risk thresholds.

---

# 42. Alert Ranking

Sort using:

```text
risk score
confidence
priority
recency
```

The exact ranking algorithm should be explicitly documented.

Example terminal output:

```text
#  TYPE          ENTITY      RISK
1  Pattern       W123        94
2  Anomaly       TX99        91
3  Cluster       W441        89
```

---

# 43. Phase 14 — CLI Investigation Shell

Add commands in this order:

```text
status
dataset
wallet
transaction
ip
graph
analyze
alerts
explain
report
export
case
model
```

Do not build every command at once.

---

# 44. CLI Command Architecture

Each command should call a core service.

Example:

```text
cmd/analyze
      ↓
InvestigationService
      ↓
GraphService
      ↓
FeatureService
      ↓
MLService
      ↓
RiskService
      ↓
EvidenceService
```

This prevents CLI code from becoming business logic.

---

# 45. `analyze wallet`

Command:

```bash
bctx analyze wallet W123
```

Execution:

```text
1. Check local data.
2. If acquisition enabled and required data is missing:
      call Data Acquisition.
3. Persist normalized data.
4. Update indexes.
5. Update graph.
6. Calculate features.
7. Run local models.
8. Run pattern detectors.
9. Calculate risk.
10. Build evidence.
11. Generate report object.
12. Render terminal result.
```

---

# 46. Offline Analyze Behavior

Command:

```bash
bctx analyze wallet W123 --offline
```

must:

```text
never access external provider
```

It should fail gracefully if required local evidence does not exist.

Example:

```text
Wallet W123 is not available in local evidence.

Offline analysis cannot retrieve missing data.

Use a connected acquisition step first.
```

---

# 47. Acquisition Flag

Recommended model:

```bash
bctx analyze wallet W123
```

Default behavior:

```text
use local data
+
acquire missing data when connected
```

Explicit:

```bash
bctx analyze wallet W123 --offline
```

Strict local-only.

Explicit acquisition:

```bash
bctx sync wallet W123
```

This separation makes behavior understandable.

---

# 48. Phase 15 — TUI

Build after the CLI/core works.

TUI screens:

```text
Home
Search
Wallet
Transaction
Graph
Alerts
Anomalies
Clusters
Monitoring
Reports
Settings
Help
```

---

# 49. TUI State Model

Maintain a central application state:

```go
type AppState struct {
    Mode           Mode
    CurrentCase    string
    SelectedEntity string
    Alerts         []Alert
    Graph          GraphView
    NetworkState   NetworkState
    Jobs           []Job
}
```

Use event messages to update state.

---

# 50. TUI Job System

Long tasks must not block the terminal.

Jobs:

```text
IMPORT_DATASET
BUILD_GRAPH
RUN_ANALYSIS
GENERATE_REPORT
SYNC_DATA
MONITOR
```

Status:

```text
QUEUED
RUNNING
COMPLETED
FAILED
CANCELLED
```

---

# 51. Graph TUI Implementation

Implement a bounded graph renderer.

Inputs:

```text
nodes
edges
selected node
depth
filters
```

Controls:

```text
g = graph
+/- = zoom/depth
f = filter
Enter = inspect
Esc = back
```

For large graphs, avoid attempting to render thousands of nodes simultaneously.

---

# 52. Evidence Panel

Every investigation view should provide an evidence pane.

Example:

```text
EVIDENCE

[1] Transaction anomaly
    Score: 0.93

[2] Peeling chain
    Confidence: 0.88

[3] Cluster relation
    Confidence: 0.81
```

Selecting an evidence item should reveal its source records.

---

# 53. Phase 16 — Monitoring

Implement:

```text
internal/monitoring/
```

Command:

```bash
bctx monitor wallet W123
```

Architecture:

```text
Provider
  ↓
Event Adapter
  ↓
Local Event Queue
  ↓
Persistence
  ↓
Incremental Analysis
  ↓
Risk Update
  ↓
TUI Event
```

---

# 54. Monitor Session State

Store:

```text
session_id
wallet
started_at
last_event_at
last_seen_tx
previous_risk
current_risk
status
```

This enables risk delta reporting.

---

# 55. Risk Delta

Whenever new events change the analysis:

```text
Previous: 63
Current:  71
Change:   +8
```

Show contributing changes:

```text
New anomaly score: +0.11
New graph connection: +0.04
New flow-pattern score: +0.06
```

The exact score decomposition depends on the risk implementation.

---

# 56. Offline Monitoring Behavior

When network disappears:

```text
monitoring acquisition = paused
local analysis = available
```

The monitor must not claim to receive new external transactions while disconnected.

The UI should display:

```text
NETWORK: DISCONNECTED
ACQUISITION: PAUSED
LOCAL ANALYSIS: AVAILABLE
LAST EVENT: 14:05:18
```

---

# 57. Phase 17 — Reporting

Implement:

```text
internal/reports/
```

Report generation should consume a structured report model.

```go
type InvestigationReport struct {
    Case
    Subject
    Risk
    Transactions
    GraphSummary
    Patterns
    Cluster
    NetworkObservations
    Evidence
    ModelMetadata
    DatasetMetadata
}
```

---

# 58. Report Formats

Implement in this order:

```text
JSON
Markdown
HTML
PDF
```

Why:

```text
JSON
↓
easy testing and machine-readable source

Markdown
↓
easy debugging

HTML
↓
visual report

PDF
↓
final deliverable
```

---

# 59. PDF Generation

PDF generation must remain local.

Pipeline:

```text
InvestigationReport
       ↓
HTML/print representation
       ↓
local PDF renderer
       ↓
report.pdf
```

Do not require a cloud PDF service.

---

# 60. Report Versioning

When monitoring changes risk:

```text
report-v1
report-v2
report-v3
```

Store:

```text
dataset snapshot
model versions
risk score
generation timestamp
```

This allows old/new comparison.

---

# 61. Phase 18 — Case Management

Commands:

```bash
bctx case create case-001
bctx case list
bctx case open case-001
bctx case close case-001
```

Every analysis should execute within a case context.

Case isolation:

```text
case-001
case-002
case-003
```

must not accidentally mix evidence.

---

# 62. Phase 19 — Provenance

For every imported/acquired record, store:

```text
source type
source identifier
retrieved/imported timestamp
source file hash or provider reference
transform/schema version
```

For files:

```text
SHA-256
```

should be stored.

This creates reproducibility.

---

# 63. Phase 20 — Audit Trail

Write local audit events for:

```text
dataset import
wallet analysis
transaction analysis
monitor start
monitor stop
risk update
alert creation
report creation
report export
model execution
```

Audit records should be immutable from the application's normal UI.

---

# 64. Phase 21 — Optional Local LLM

This should be implemented only after the deterministic pipeline works.

Possible use:

```text
summarize evidence
summarize monitor session
generate human-readable investigation narrative
natural-language command interpretation
```

The architecture:

```text
Evidence Engine
     ↓
Structured JSON
     ↓
Local LLM
     ↓
Summary
```

The LLM should not have unrestricted access to arbitrary external resources.

---

# 65. Natural-Language Agent Layer

Future command:

```text
bctx> investigate everything suspicious around W123
```

Agent orchestrates existing commands internally:

```text
search
graph
features
anomaly
patterns
risk
evidence
report
```

The agent is a workflow orchestrator.

It should not replace specialized engines.

---

# 66. Phase 22 — GeoIP

If geographic analysis is enabled:

```text
IP
 ↓
local MMDB
 ↓
country
ASN
approximate region
```

Data should be bundled/imported locally.

The system should record the GeoIP database version.

---

# 67. Phase 23 — Security

Implement security controls before final release.

## Required

```text
no analytics telemetry
no hidden network calls
case directory isolation
file permission checks
input validation
safe export paths
model checksum verification
dataset checksum
secure temporary-file handling
```

---

# 68. Network Dependency Enforcement

The code architecture must make it difficult for offline modules to access network APIs.

Suggested package rule:

```text
internal/acquisition
```

may import network clients.

Other core packages should not directly depend on acquisition providers.

Conceptually:

```text
CLI
 ↓
Acquisition interface
 ↓
local repository
 ↓
analysis
```

not:

```text
analysis → HTTP client
```

---

# 69. Offline Test Harness

Create:

```text
tests/offline/
```

The test harness should:

1. prepare a local case;
2. run analysis once;
3. disable network access;
4. rerun analysis;
5. compare outputs;
6. verify no external connection attempt.

Test commands:

```bash
make test-offline
```

---

# 70. Deterministic Offline Test

Test:

```bash
bctx analyze wallet W123
```

with network unavailable.

Verify:

```text
same local dataset
same model version
same configuration
```

results in equivalent:

```text
risk score
model score
evidence IDs
graph result
```

within defined numerical tolerance.

---

# 71. Phase 24 — Performance Testing

Create benchmarks for:

```text
CSV ingestion
JSON ingestion
SQLite insertion
wallet lookup
TX lookup
IP lookup
graph expansion
feature generation
ML inference
risk calculation
report generation
```

Go benchmark examples:

```bash
go test -bench=.
```

Record:

```text
records/sec
latency
memory
database size
```

---

# 72. Large Dataset Strategy

For multi-million-record datasets:

```text
streaming ingestion
batch commits
indexed lookup
lazy graph expansion
incremental feature calculation
```

Avoid:

```text
full database load into RAM
full graph rendering
full graph traversal for every query
```

---

# 73. Phase 25 — Error Recovery

Every major operation needs recoverable failure.

Examples:

```text
database unavailable
corrupt dataset
missing model
invalid wallet
provider timeout
partial sync
report export failure
```

The system should preserve already-committed state.

---

# 74. `bctx doctor`

Implement a complete health-check command.

Example:

```bash
bctx doctor
```

Output:

```text
BCTX SYSTEM CHECK

[PASS] Binary
[PASS] Configuration
[PASS] Database
[PASS] Schema
[PASS] Indexes
[PASS] Graph
[PASS] Anomaly Model
[PASS] Entity Model
[PASS] Flow Model
[PASS] Report Engine
[PASS] GeoIP

Offline Analysis:
[PASS]

Network Calls During Analysis:
[NONE]

BCTX READY
```

---

# 75. Phase 26 — Model Integrity

Before loading a model:

```text
read manifest
 ↓
verify checksum
 ↓
verify schema
 ↓
verify supported runtime
 ↓
load model
```

If verification fails:

```text
MODEL INTEGRITY ERROR
```

and inference must not start.

---

# 76. Phase 27 — Packaging

Final package:

```text
bctx
models/
checksums.txt
model-manifest.json
optional geo database
LICENSE
```

Build:

```bash
make release
```

Outputs:

```text
bctx-linux-amd64.tar.gz
bctx-linux-arm64.tar.gz
```

---

# 77. Installer

Online installer:

```bash
curl -fsSL https://<official-domain>/install.sh | sh
```

Steps:

```text
detect architecture
download package
verify checksum/signature
install binary
install models
initialize directories
run doctor
```

---

# 78. Air-Gapped Installation

Prepare the release bundle on another machine:

```text
BCTX bundle
+
model bundle
+
optional GeoIP
+
demo dataset
```

Transfer through approved removable media.

Then:

```bash
./install-local.sh
```

No internet connection required.

---

# 79. Demo Dataset

Create:

```text
demo/synthetic-case.bctxdata
```

The demo must contain:

```text
normal activity
anomalous activity
entity clusters
peeling chain
mixing-like structure
network observations
```

Also include known expected findings for testing.

---

# 80. Demo Command Sequence

The final SIH demonstration should follow:

```bash
bctx
```

then:

```bash
bctx dataset import demo.csv
```

then:

```bash
bctx analyze wallet W1042
```

then:

```bash
bctx explain wallet W1042
```

then:

```bash
bctx report wallet W1042
```

then:

```bash
bctx export report W1042 --format pdf
```

Then disconnect the machine.

Run:

```bash
bctx analyze wallet W1042 --offline
bctx explain wallet W1042
bctx report wallet W1042
```

All should succeed.

---

# 81. Monitor Demonstration

While connected:

```bash
bctx monitor wallet W1042
```

Show:

```text
new transaction
new transaction
new transaction
```

Then:

```text
Risk:
63 → 68 → 74
```

Disconnect.

Show:

```text
NETWORK: DISCONNECTED
ACQUISITION: PAUSED
LOCAL ANALYSIS: AVAILABLE
```

Then:

```bash
bctx summarize monitor
bctx report wallet W1042
```

---

# 82. Implementation Order Inside the Team

Recommended parallelization:

## Member / Track A — Core

```text
Go foundation
CLI
config
case management
```

## Member / Track B — Data

```text
schemas
ingestion
SQLite
indexes
```

## Member / Track C — Graph

```text
graph model
traversal
graph analytics
TUI graph renderer
```

## Member / Track D — ML

```text
synthetic generator
features
anomaly model
clustering
flow detection
ONNX export
```

## Member / Track E — UX/Reports

```text
TUI
alerts
evidence
reports
PDF/HTML
```

These tracks should merge through interfaces rather than directly sharing internal implementation details.

---

# 83. Interface Contracts

Define interfaces early.

Example:

```go
type DataSource interface {
    GetTransactions(ctx context.Context, wallet string) ([]Transaction, error)
}

type Repository interface {
    GetWallet(ctx context.Context, id string) (*Wallet, error)
}

type GraphService interface {
    Expand(ctx context.Context, id string, depth int) (Subgraph, error)
}

type MLModel interface {
    Predict(ctx context.Context, features FeatureVector) (Prediction, error)
}

type RiskService interface {
    Assess(ctx context.Context, subject Subject) (RiskAssessment, error)
}
```

These are conceptual and can be refined during implementation.

---

# 84. Investigation Service

Create one high-level service responsible for orchestration.

Conceptual:

```go
type InvestigationService struct {
    Data      Repository
    Graph     GraphService
    Features  FeatureService
    ML        MLService
    Risk      RiskService
    Evidence  EvidenceService
}
```

Method:

```go
InvestigateWallet(ctx, walletID)
```

This becomes the primary path used by:

```text
CLI
TUI
monitor
SDK
agent
```

---

# 85. Investigation Result Object

Use a structured result.

```go
type InvestigationResult struct {
    Subject       Subject
    Transactions  []TransactionSummary
    GraphSummary  GraphSummary
    Anomalies     []Prediction
    Patterns      []PatternResult
    Cluster       ClusterResult
    Risk          RiskAssessment
    Evidence      []EvidenceItem
}
```

The presentation layer should render this rather than recompute data.

---

# 86. Monitor Service

Create:

```go
type MonitorService struct {
    Source      DataSource
    Repository  Repository
    Investigator InvestigationService
}
```

Flow:

```text
new event
 ↓
persist
 ↓
update graph
 ↓
investigate affected subject
 ↓
calculate risk delta
 ↓
emit MonitorEvent
```

---

# 87. Event Architecture

Events:

```text
TransactionReceived
DatasetImported
GraphUpdated
AnalysisStarted
AnalysisCompleted
RiskUpdated
AlertCreated
ReportGenerated
MonitorDisconnected
MonitorConnected
```

The TUI subscribes to these events.

---

# 88. Report Service

Create:

```go
type ReportService interface {
    Generate(ctx context.Context, result InvestigationResult) (Report, error)
    Export(ctx context.Context, report Report, format string, path string) error
}
```

This keeps formats separate from analysis.

---

# 89. Local Repository Pattern

All analysis should query repositories.

Example:

```text
WalletRepository
TransactionRepository
NetworkRepository
EntityRepository
EvidenceRepository
AlertRepository
ReportRepository
```

This makes persistence replaceable and testable.

---

# 90. Testing Architecture

## Unit Tests

Test independently:

```text
parser
normalizer
repository
graph
feature calculations
risk calculations
pattern detectors
report rendering
```

## Integration Tests

Test:

```text
import
database
graph
ML
risk
report
```

together.

## End-to-End Tests

Test:

```text
dataset
→ import
→ analyze
→ evidence
→ report
```

---

# 91. Test Fixtures

Store compact deterministic datasets:

```text
tests/fixtures/
├── normal.csv
├── peeling.csv
├── mixing.csv
├── clustering.csv
└── network.csv
```

Expected outputs:

```text
tests/expected/
```

This makes regression testing possible.

---

# 92. Security Testing

Test:

```text
malformed CSV
malformed JSON
malformed XML
huge field
invalid IP
invalid port
path traversal
symlink handling
corrupt SQLite
tampered model
```

---

# 93. Resource Limits

Protect the application against pathological input.

Configurable limits:

```text
maximum graph depth
maximum nodes rendered
maximum imported record size
maximum concurrent jobs
maximum report size
maximum monitor events queued
```

These prevent accidental resource exhaustion.

---

# 94. Logging Strategy

Use structured local logs.

Separate:

```text
application.log
audit.log
import errors
monitor events
```

Do not put sensitive raw datasets into debug logs by default.

---

# 95. Telemetry Policy

Default:

```text
NO TELEMETRY
```

No anonymous usage reporting should be required for operation.

Any future telemetry mode should be explicit and disabled by default in sensitive environments.

---

# 96. Offline-First Acceptance Requirements

BCTX is not considered production-ready until all of these work without network:

```text
startup
database access
wallet lookup
transaction lookup
IP lookup against local data
graph queries
anomaly inference
entity analysis
pattern detection
risk scoring
evidence generation
alerts
report generation
PDF export
JSON export
TUI
```

The only excluded functionality is acquisition of data that does not already exist locally.

---

# 97. Important Runtime Distinction

## Connected mode

```text
Acquire
Sync
Monitor
Persist
Analyze
```

## Offline mode

```text
Search
Analyze
Trace
Cluster
Detect
Score
Explain
Summarize
Report
Export
```

## Disconnected limitation

```text
New external Bitcoin data cannot arrive.
```

The UI must reflect this honestly.

---

# 98. MVP Definition

The first working product should include:

```text
Go binary
CLI
basic TUI
SQLite
CSV import
wallet lookup
TX lookup
transaction graph
wallet features
anomaly model
basic entity clustering
one flow detector
risk score
evidence
alerts
explain command
report
offline analysis
```

Do not delay the working demo for advanced features.

---

# 99. Post-MVP

After the MVP is stable:

```text
JSON/XML import
DuckDB analytics
WebSocket monitor
incremental risk
advanced graph algorithms
GeoIP
HTML/PDF improvements
local LLM
natural-language agent
model update bundles
```

---

# 100. Final Build Sequence

The implementation should proceed in this exact practical order:

```text
1. Go binary
   ↓
2. CLI commands + config
   ↓
3. SQLite + migrations
   ↓
4. CSV importer
   ↓
5. Wallet/TX/IP indexes
   ↓
6. Wallet/TX lookup
   ↓
7. Graph construction
   ↓
8. Graph traversal
   ↓
9. Feature engine
   ↓
10. Synthetic data generator
   ↓
11. Anomaly model
   ↓
12. ONNX local inference
   ↓
13. Entity clustering
   ↓
14. Peeling/flow detection
   ↓
15. Risk engine
   ↓
16. Evidence engine
   ↓
17. Alerts
   ↓
18. analyze command
   ↓
19. explain command
   ↓
20. TUI
   ↓
21. Monitoring
   ↓
22. Reports
   ↓
23. Offline hardening
   ↓
24. Installer
   ↓
25. SIH demo
```

---

# 101. Final End-to-End Runtime

When an analyst runs:

```bash
bctx analyze wallet W123
```

the implementation should effectively execute:

```text
CLI
 ↓
InvestigationService
 ↓
Local repository lookup
 ↓
Missing-data check
 ↓
Optional AcquisitionAdapter
 ↓
Local persistence
 ↓
Index update
 ↓
Graph expansion
 ↓
Feature extraction
 ↓
Anomaly model
 ↓
Entity/cluster analysis
 ↓
Pattern detectors
 ↓
Risk engine
 ↓
Evidence engine
 ↓
InvestigationResult
 ↓
CLI/TUI renderer
```

When the analyst then disconnects the machine:

```text
NETWORK = OFF

Local repository
 ↓
Graph
 ↓
Features
 ↓
ONNX model
 ↓
Risk
 ↓
Evidence
 ↓
Report
```

continues to operate.

---

# 102. Final Engineering Goal

The engineering target is not simply:

```text
"make a dashboard"
```

It is:

```text
Build a self-contained local intelligence engine
that happens to have a terminal interface.
```

The architecture should therefore prioritize:

```text
data correctness
graph correctness
model correctness
evidence traceability
offline reliability
reproducibility
performance
security
```

The terminal UI is the investigator's interface to that engine.

---

# 103. Definition of Done

BCTX is ready for the SIH prototype demonstration when:

```text
[✓] Linux binary launches directly
[✓] Dataset can be imported
[✓] Wallets can be searched
[✓] Transactions can be searched
[✓] IP metadata can be queried from local data
[✓] Graph is constructed
[✓] Graph can be traversed
[✓] ML model runs locally
[✓] Anomaly score is generated
[✓] Entity clustering works
[✓] Suspicious flow detector works
[✓] Risk score is generated
[✓] Evidence is attached
[✓] Alerts are ranked
[✓] Explain command works
[✓] TUI works
[✓] Report is generated
[✓] PDF/JSON export works
[✓] Monitoring works while connected
[✓] Captured monitor data persists locally
[✓] Analysis works after network disconnect
[✓] No analysis-time external calls are required
[✓] `bctx doctor` validates installation
[✓] Demo works on a clean Linux environment
```

---

# 104. Final Architecture in One Diagram

```text
                           USER
                            │
                            ▼
                    ┌──────────────┐
                    │  BCTX CLI/TUI│
                    └──────┬───────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │ Investigation     │
                  │ Orchestrator      │
                  └────────┬─────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
         ▼                 ▼                 ▼
   Acquisition         Local Data        Case Manager
   (network)             Store
         │                 │
         │                 ▼
         │             Indexes
         │                 │
         └────────────┐    │
                      ▼    ▼
                  ┌───────────────┐
                  │ Graph Engine  │
                  └───────┬───────┘
                          ▼
                  ┌───────────────┐
                  │ Feature Engine│
                  └───────┬───────┘
                          ▼
                  ┌───────────────┐
                  │ Local ML      │
                  │ ONNX Runtime  │
                  └───────┬───────┘
                          ▼
                  ┌───────────────┐
                  │ Risk Engine   │
                  └───────┬───────┘
                          ▼
                  ┌───────────────┐
                  │ Evidence      │
                  └───────┬───────┘
                          │
              ┌───────────┼────────────┐
              ▼           ▼            ▼
             TUI        Alerts       Reports
                                      │
                              ┌───────┼───────┐
                              ▼       ▼       ▼
                             PDF     HTML     JSON


             ───────── OFFLINE BOUNDARY ─────────

                  Everything below this line
                   operates from local state.
```

---

# 105. Final Implementation Principle

**BCTX should be built as a native, modular, offline intelligence engine first and a terminal interface second.**

The acquisition layer provides data.

The local evidence store owns that data.

The graph engine explains relationships.

The feature engine turns evidence into measurable signals.

The local ML runtime detects unusual behavior.

The risk engine prioritizes investigations.

The evidence engine explains why.

The CLI/TUI lets the analyst operate the entire system.

The reporting engine preserves the result.

And once the necessary data is local:

```text
internet disconnected
        ↓
BCTX remains fully usable
```
