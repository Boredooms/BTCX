# BCTX — Final Architecture & Product Requirements Document

**Document Type:** Product Requirements + System Architecture Specification  
**Product:** BCTX  
**Working Product Name:** BCTX — Bitcoin Forensic Intelligence Platform  
**Primary Runtime:** Linux  
**Primary Interface:** Terminal CLI + Interactive Terminal UI (TUI)  
**Architecture Principle:** Offline-first, local-first, evidence-driven  
**Document Status:** Final current vision / implementation baseline

---

# 1. Executive Summary

BCTX is an **offline-first Bitcoin forensic intelligence and investigation platform** delivered primarily as a single Linux command-line application with an interactive terminal UI.

The user installs BCTX, launches it from the terminal, and uses investigation commands such as:

```bash
bctx
bctx analyze wallet <WALLET_ID>
bctx monitor wallet <WALLET_ID>
bctx report wallet <WALLET_ID>
```

The core product workflow is:

```text
User
  ↓
BCTX Terminal / TUI
  ↓
Data Acquisition (when connected)
  ↓
Local Evidence Store
  ↓
Transaction + Entity Graph
  ↓
Feature Extraction
  ↓
Local AI/ML Models
  ↓
Risk Engine
  ↓
Explainability / Evidence Engine
  ↓
Investigation Report
  ↓
Terminal + PDF/JSON/HTML export
```

The most important product property is:

> **Once the relevant data, models, indexes, and auxiliary datasets have been acquired locally, BCTX can continue investigating and generating reports with the network completely disconnected.**

BCTX is therefore not simply a dashboard, wallet checker, or web application. It is intended to function like a **terminal-based investigation workstation**, conceptually similar to the interaction model of developer tools such as Claude Code, but specialized for Bitcoin transaction intelligence.

---

# 2. Source Problem Statement Alignment

The supplied SIH problem statement describes **AI-Powered Monitoring & Analysis of Bitcoin Transaction Traffic**.

The stated challenge requires a complete offline solution that can:

- ingest bulk Bitcoin transaction/network metadata;
- correlate network-layer observations such as IP, port, and timing with blockchain-layer data such as wallet, TXID, and amount;
- build an entity/transaction graph linking IPs, wallets, and transactions;
- implement an AI/ML detection use case with a working model;
- generate a ranked, explainable alert list including why an entity/transaction was flagged and a confidence score;
- present findings through dashboard or link-analysis visualization.

The document also names the suggested ML focus areas:

- Entity Clustering
- Anomaly Detection
- Peeling-Chain / Mixing Detection
- Risk Scoring

The dataset is specified as synthetic and modeled on real Bitcoin P2P/transaction fields. The stated expected solution is a workable offline Linux system with ingestion, correlation, AI/ML, technical explanation, and visualization/evidence.

**Source-derived requirement:** These elements are the non-negotiable requirements that BCTX must satisfy.

**Product design extension:** The terminal-first UX, local case store, explicit online/offline acquisition modes, local model packaging, SDK structure, and command-oriented investigation workflow are BCTX architectural decisions built to fulfill those requirements.

---

# 3. Product Vision

## 3.1 Vision Statement

> BCTX turns large Bitcoin transaction and network datasets into an offline, searchable, explainable investigation environment where analysts can discover anomalous activity, understand entity relationships, trace transaction flows, score risk, and generate evidence-backed reports.

## 3.2 Product Philosophy

BCTX should follow these principles:

### Offline by default after acquisition

The analysis engine, graph engine, risk engine, models, search, reports, and TUI must not require cloud services.

### Evidence before conclusion

Every risk score should have inspectable signals and supporting transaction/network evidence.

### Graph-native investigation

Wallets, transactions, entities, and network observations should be treated as connected data instead of isolated rows.

### Explainable AI

The ML layer should produce scores/features that can be surfaced to the analyst rather than a black-box "suspicious" label with no explanation.

### Terminal-first operation

The primary workflow should be possible without opening a browser.

### Controlled data acquisition

Internet-dependent acquisition should be a separate layer from offline investigation. Once data is persisted locally, the analysis layer must be able to operate without the network.

---

# 4. Primary Users

## 4.1 Primary User

**Bitcoin / digital-forensics / financial-intelligence analyst**

Typical tasks:

- investigate a wallet;
- inspect transaction history;
- trace transaction paths;
- discover connected wallets;
- investigate suspicious clusters;
- inspect network observations;
- identify anomalies;
- review risk scores;
- understand why something was flagged;
- generate an evidence report.

## 4.2 Organizational Context

The SIH problem statement names the **National Technical Research Organisation (NTRO)** and requires an offline Linux deployment.

Therefore BCTX is designed conceptually for environments where local processing and data isolation are important.

## 4.3 Secondary/Future Users

The core engine may later support:

- cryptocurrency compliance teams;
- exchange investigation teams;
- blockchain analytics teams;
- fraud/risk teams;
- incident-response and cyber-intelligence teams.

These are future product extensions and are not required to define the MVP.

---

# 5. What BCTX Is and Is Not

## BCTX IS

- an offline Bitcoin intelligence platform;
- a transaction-analysis engine;
- a graph investigation system;
- an AI/ML anomaly and pattern-analysis system;
- a risk-scoring engine;
- an evidence/explainability engine;
- a terminal CLI/TUI;
- a reporting tool;
- a local SDK/core library.

## BCTX IS NOT

- a Bitcoin wallet;
- a cryptocurrency exchange;
- a miner;
- the Bitcoin blockchain itself;
- a tool that can receive new blockchain transactions while physically disconnected from all networks;
- a system that proves the real-world identity of a wallet owner;
- an automatic declaration of criminal activity.

BCTX reports observations, relationships, signals, scores, and evidence from its available data.

---

# 6. Core User Journey

The product has two major operating workflows.

---

## 6.1 Workflow A — Analyze a Wallet

User:

```bash
bctx analyze wallet bc1q...
```

### Connected acquisition stage

If the requested data is not already available locally:

```text
Wallet ID
   ↓
Data Acquisition Layer
   ↓
Bitcoin Explorer / Blockchain Data Adapter
   ↓
Transaction History
   ↓
Relevant Transaction Details
   ↓
Local Evidence Store
```

The system should persist acquired data before analysis.

### Local analysis stage

```text
Local Data
   ↓
Normalization
   ↓
Indexes
   ↓
Graph Construction / Update
   ↓
Feature Extraction
   ↓
Local ML Inference
   ↓
Risk Calculation
   ↓
Evidence Generation
   ↓
Terminal Report
```

### Offline transition

The analyst may now disconnect the machine.

The same command:

```bash
bctx analyze wallet bc1q...
```

must continue to work using local evidence.

The UI should explicitly show:

```text
NETWORK: OFFLINE
DATA SOURCE: LOCAL
MODEL: LOCAL
```

---

# 7. Core User Journey B — Monitor a Wallet

Command:

```bash
bctx monitor wallet bc1q...
```

While connected, BCTX can receive newly available transaction data through a pluggable acquisition adapter.

```text
External Bitcoin Data Source
          ↓
Monitor Ingestor
          ↓
Local Event Store
          ↓
Incremental Graph Update
          ↓
Incremental Feature Update
          ↓
Local ML Inference
          ↓
Risk Recalculation
          ↓
TUI Live View
```

Example:

```text
MONITORING W123

12:41:02  TX a81f...   +0.82 BTC
12:41:05  TX 92bc...   -0.41 BTC
12:41:11  TX 81ae...   -0.39 BTC

Previous Risk: 63
Current Risk:  71
Change:        +8
```

## Important offline constraint

A fully disconnected machine cannot receive genuinely new blockchain transactions.

Therefore:

```text
Connected mode:
  acquire new data + analyze

Disconnected mode:
  analyze everything already captured locally
```

When offline, the interface must not claim that newly generated network events are "live."

Recommended status:

```text
NETWORK: DISCONNECTED
LAST DATA SYNC: 14:05:18
NEW NETWORK DATA: PAUSED
LOCAL ANALYSIS: RUNNING
```

---

# 8. Command-Line Product Model

BCTX should behave as an investigation shell.

Example:

```bash
$ bctx
```

Interactive prompt:

```text
bctx>
```

Suggested commands:

```text
help
status
version

dataset list
dataset inspect
dataset import
dataset export

sync wallet <wallet>
sync transaction <txid>

analyze wallet <wallet>
analyze transaction <txid>
analyze entity <entity>

wallet <wallet>
transaction <txid>
ip <ip>

graph wallet <wallet>
graph transaction <txid>
graph cluster <cluster>
neighbors <id>
path <source> <destination>

alerts
anomalies
clusters
patterns

monitor wallet <wallet>

risk <wallet>
risk transaction <txid>

explain wallet <wallet>
explain transaction <txid>

report wallet <wallet>
report transaction <txid>

export report <id> --format pdf
export report <id> --format json
export report <id> --format html

case create <name>
case open <name>
case list

model status
model info

config
doctor

exit
```

The exact command syntax may evolve, but the architecture should preserve a command-oriented mental model.

---

# 9. Interactive TUI

The terminal should not merely print text. BCTX should contain a full interactive TUI for investigation.

Conceptual layout:

```text
┌─────────────────────────────────────────────────────────────────────┐
│ BCTX                         OFFLINE / CONNECTED                     │
├──────────────────┬──────────────────────────────────────────────────┤
│ MAIN MENU        │                                                  │
│                  │          TRANSACTION / ENTITY GRAPH              │
│ Dashboard        │                                                  │
│ Search           │                 W100                              │
│ Wallet           │                  │                               │
│ Transaction      │                  ▼                               │
│ Graph            │                TX901                             │
│ Alerts           │               /     \                            │
│ Anomalies        │             W928    W441                         │
│ Clusters         │               │                                  │
│ Monitoring       │               ▼                                  │
│ Reports          │              W821                                │
├──────────────────┴──────────────────────────────────────────────────┤
│ EVIDENCE                                                             │
│ [HIGH] Transaction anomaly                                           │
│ [HIGH] Peeling-chain pattern                                         │
│ [MED]  Cluster relationship                                          │
├─────────────────────────────────────────────────────────────────────┤
│ bctx> explain W928                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Navigation expectations

Examples:

```text
↑ ↓        navigate
Enter      select
Tab        panel
/          search
f          filter
g          graph
a          alerts
r          reports
m          map
e          explain
q          back/quit
```

Keyboard controls should be discoverable through a persistent footer/help screen.

---

# 10. High-Level Architecture

```text
                           ┌───────────────────────┐
                           │       BCTX CLI        │
                           │       + TUI           │
                           └───────────┬───────────┘
                                       │
                           ┌───────────▼───────────┐
                           │ Command / Workflow    │
                           │ Orchestrator           │
                           └───────────┬───────────┘
                                       │
          ┌────────────────────────────┼────────────────────────────┐
          │                            │                            │
          ▼                            ▼                            ▼
┌──────────────────┐        ┌──────────────────┐        ┌──────────────────┐
│ Data Acquisition │        │ Investigation     │        │ Case Management   │
│ Adapters         │        │ Engine            │        │                  │
└────────┬─────────┘        └────────┬─────────┘        └────────┬─────────┘
         │                           │                           │
         ▼                           ▼                           ▼
┌──────────────────┐        ┌──────────────────┐        ┌──────────────────┐
│ Normalization    │        │ Graph Engine     │        │ Local Case Store  │
└────────┬─────────┘        └────────┬─────────┘        └──────────────────┘
         │                           │
         └──────────────┬────────────┘
                        ▼
               ┌──────────────────┐
               │ Feature Engine   │
               └────────┬─────────┘
                        │
                        ▼
               ┌──────────────────┐
               │ Local ML Engine  │
               │ ONNX Runtime     │
               └────────┬─────────┘
                        │
                        ▼
               ┌──────────────────┐
               │ Risk Engine      │
               └────────┬─────────┘
                        │
                        ▼
               ┌──────────────────┐
               │ Evidence Engine  │
               └────────┬─────────┘
                        │
              ┌─────────┴─────────┐
              ▼                   ▼
      Terminal/TUI          Report Exporter
```

---

# 11. Recommended Technology Stack

## 11.1 Core Runtime — Go

**Recommendation: Go**

BCTX should use Go as the primary production runtime because it is well suited for:

- Linux-native tooling;
- static binary distribution;
- fast startup;
- straightforward concurrency;
- CLI applications;
- terminal applications;
- background workers;
- local storage integration;
- subprocess management;
- deployment as one executable.

Primary package structure:

```text
cmd/
internal/
pkg/
models/
migrations/
assets/
```

A single production entry point:

```bash
bctx
```

---

# 12. Terminal UI Technology

Recommended:

**Bubble Tea / Go TUI ecosystem**

Possible supporting components:

- Bubble Tea — application model/event loop;
- Lip Gloss — terminal styling/layout;
- Bubbles — reusable TUI components;
- terminal graph/rendering helpers or custom rendering;
- ANSI-compatible terminal rendering.

The final product should remain usable through plain CLI commands even if the TUI is unavailable.

Therefore:

```text
CLI
= mandatory

TUI
= primary interactive UX
```

---

# 13. Storage Architecture

BCTX should use local databases only.

## 13.1 Primary Storage

Recommended MVP approach:

**SQLite**

Advantages:

- zero-server database;
- embedded;
- one local file;
- transactional;
- easy Linux deployment;
- easy backups;
- suitable for case-based storage.

## 13.2 Analytics Storage

For large analytical workloads, **DuckDB** can be used where columnar analytical queries provide a benefit.

Recommended division:

```text
SQLite
  → operational metadata
  → cases
  → entities
  → alerts
  → reports
  → configuration

DuckDB
  → large-scale analytical queries
  → feature computation
  → bulk statistics
```

The architecture should not require both on day one. Start with SQLite and add DuckDB when dataset scale proves it necessary.

---

# 14. Data Model

Minimum logical entities:

```text
Case
Dataset
Wallet
Transaction
TransactionInput
TransactionOutput
NetworkObservation
EntityCluster
GraphEdge
FeatureVector
ModelPrediction
RiskAssessment
Alert
EvidenceItem
MonitorSession
Report
```

---

# 15. Transaction Record Model

The source challenge specifies fields including:

```text
timestamp
src_ip
src_port
dst_ip
dst_port
txid
input_addresses[]
output_addresses[]
input_amounts[]
output_amounts[]
fee
script_type
country / ASN
```

BCTX should normalize incoming formats into an internal schema.

Example:

```json
{
  "txid": "TX123",
  "timestamp": "2026-10-02T10:30:00Z",
  "inputs": [
    {
      "address": "W001",
      "amount_btc": 2.4
    }
  ],
  "outputs": [
    {
      "address": "W002",
      "amount_btc": 2.3
    }
  ],
  "fee_btc": 0.0001,
  "network_observations": [
    {
      "src_ip": "10.1.2.3",
      "src_port": 8333,
      "dst_ip": "10.1.2.4",
      "dst_port": 8333
    }
  ]
}
```

The canonical internal format should be independent of the source CSV/JSON/XML format.

---

# 16. Data Acquisition Layer

The acquisition layer is intentionally isolated from the offline analysis engine.

## 16.1 Adapter Interface

Conceptually:

```text
DataSourceAdapter
├── ExplorerAdapter
├── FileAdapter
├── SyntheticDatasetAdapter
└── FutureProviderAdapter
```

Methods conceptually include:

```text
getAddress()
getAddressTransactions()
getTransaction()
getRelatedData()
subscribeToNewEvents()
```

The implementation must be provider-agnostic.

This means BCTX can support:

```text
Provider A
Provider B
Local dataset
Synthetic dataset
```

without changing the analysis engine.

## 16.2 Acquisition Output

Every external response should immediately become:

```text
raw evidence
   ↓
normalized record
   ↓
local persistent record
   ↓
indexed record
```

The analysis layer should consume the local normalized representation.

---

# 17. The "Fully Offline" Architecture

BCTX should contain all required runtime components locally.

```text
BCTX Installation
│
├── bctx executable
│
├── models/
│   ├── anomaly.onnx
│   ├── entity.onnx
│   └── flow.onnx
│
├── assets/
│   ├── schemas/
│   └── model metadata
│
└── runtime configuration
```

User data lives separately:

```text
~/.bctx/
│
├── cases/
├── database/
├── indexes/
├── evidence/
├── reports/
├── cache/
└── logs/
```

After acquisition:

```text
Internet = OFF

BCTX
 ├── local database
 ├── local indexes
 ├── local graph
 ├── local ML models
 ├── local GeoIP data if enabled
 ├── local reports
 └── local case files
```

No component in the analysis path should make a network call.

---

# 18. Network Boundary Rule

This is a hard architectural rule:

> **Only the Data Acquisition Layer may require external network access.**

The following must never depend on the internet at runtime:

```text
CLI
TUI
Database
Graph Engine
Feature Engine
ML Engine
Risk Engine
Evidence Engine
Report Generator
Search
Wallet Lookup against local data
Transaction Lookup against local data
Graph Traversal
Historical Analysis
Model Inference
```

This boundary should be enforceable in code and testable.

---

# 19. Data Sync Model

BCTX should support an explicit sync workflow.

Example:

```bash
bctx sync wallet W123
```

Pipeline:

```text
External Source
      ↓
Fetch
      ↓
Validate
      ↓
Deduplicate
      ↓
Normalize
      ↓
Persist
      ↓
Index
      ↓
Graph Update
      ↓
Risk Recalculation
```

The user can then disconnect from the internet.

The local state becomes the investigation snapshot.

---

# 20. Dataset Import

Because the challenge explicitly requires bulk ingestion and because the dataset may be supplied as CSV/JSON/XML, BCTX must provide:

```bash
bctx dataset import dataset.csv
bctx dataset import dataset.json
bctx dataset import dataset.xml
```

The importer must:

1. detect/validate schema;
2. stream records rather than loading an entire huge file into RAM;
3. parse timestamps;
4. normalize wallet IDs;
5. normalize transaction IDs;
6. parse amount arrays;
7. validate network metadata;
8. deduplicate records;
9. persist records;
10. build indexes;
11. report ingestion statistics.

Example:

```text
Importing case01.csv

Records read:       2,481,392
Valid:              2,475,820
Rejected:               5,572
Transactions:       1,104,229
Wallets:              384,912
Network records:      723,441

Building indexes...
Building graph...

DONE
```

---

# 21. Local Indexing

Indexes are essential for interactive performance.

At minimum:

```text
txid → transaction
wallet → transaction list
ip → network observations
timestamp → events
entity → wallets
cluster → members
transaction → input/output wallets
```

Example:

```text
Wallet Index

W123
 ├── TX001
 ├── TX019
 ├── TX082
 └── TX901
```

This allows:

```bash
bctx wallet W123
```

without scanning every transaction.

---

# 22. Graph Engine

The graph is a first-class subsystem.

## Nodes

```text
Wallet
Transaction
IP
Entity / Cluster
```

## Edges

Examples:

```text
Wallet ──SENT_TO──> Wallet
Wallet ──INPUT_TO──> Transaction
Transaction ──OUTPUT_TO──> Wallet
IP ──OBSERVED_WITH──> Transaction
Wallet ──MEMBER_OF──> Entity
```

The graph engine must support:

```text
neighbors
path finding
N-hop traversal
subgraph extraction
cluster extraction
degree analysis
temporal graph queries
risk propagation
```

---

# 23. Entity Clustering

The product must support grouping wallets that appear behaviorally or structurally related.

Potential signals:

```text
common-input patterns
transaction co-occurrence
shared graph neighborhood
transaction timing
transaction behavior
graph embeddings
```

The challenge specifically suggests common-input ownership heuristics plus graph embeddings.

Important interpretation rule:

```text
Cluster = inferred relationship
NOT
Cluster = proven real-world ownership
```

The UI should phrase findings carefully.

Example:

```text
CLUSTER #18

14 related wallet addresses

Confidence: 0.87

Basis:
• common-input pattern
• graph similarity
• repeated transaction behavior
```

---

# 24. Anomaly Detection

The system must include an actual ML-based anomaly detection component.

Potential features:

```text
transaction frequency
amount
amount variance
transaction timing
in/out ratio
wallet degree
fan-in
fan-out
number of hops
time between hops
counterparty diversity
cluster behavior
network-observation frequency
```

Output:

```text
anomaly_score ∈ [0, 1]
```

Example:

```text
Wallet W123
Anomaly Score: 0.93
```

The score should be accompanied by feature-level evidence.

---

# 25. Suspicious Flow / Pattern Detection

The product should implement detection for patterns suggested in the problem statement.

## Peeling chains

Conceptual signature:

```text
large balance
   ↓
repeated split
   ↓
small portion removed
   ↓
remaining funds continue
   ↓
repeat
```

## Mixing-like structures

Potential graph signatures may include:

```text
many inputs
many outputs
high participant count
near-balanced value movement
shared transaction structure
```

These should be described as **pattern matches**, not proof of illegal activity.

---

# 26. Risk Engine

The risk engine combines signals.

Possible inputs:

```text
anomaly score
cluster risk
pattern score
transaction velocity
graph proximity
seed risk
network correlation
historical behavior
```

Conceptually:

```text
Risk Score
   =
weighted combination of model outputs
+ graph signals
+ pattern signals
+ evidence confidence
```

The exact mathematical formula must be validated experimentally rather than hardcoded without evaluation.

Output:

```text
0–100 risk score
```

Example:

```text
Risk Score: 91/100
Confidence: 0.93
```

The distinction must remain:

```text
Risk score = investigative prioritization
NOT
Risk score = proof of wrongdoing
```

---

# 27. Risk Propagation

The challenge explicitly proposes propagating risk from seed illicit wallets.

BCTX can implement graph-based propagation.

Conceptual model:

```text
Seed Entity
Risk = 1.00
    │
    ├── directly connected → 0.70
    │
    ├── 2 hops → 0.45
    │
    └── 3 hops → 0.20
```

The actual decay and propagation algorithm must be configurable and validated.

Potential methods:

```text
personalized PageRank
label propagation
graph diffusion
distance-decay propagation
```

Risk propagation should always record:

```text
source entity
path
distance
propagation contribution
```

so the analyst can explain why a score changed.

---

# 28. Evidence Engine

Every alert should be backed by structured evidence.

Evidence examples:

```text
E1:
Transaction velocity > learned baseline

E2:
Peeling-chain structure detected

E3:
Wallet linked to cluster #18

E4:
Graph distance = 1 from seed entity

E5:
Three correlated network observations
```

Each evidence item should contain:

```text
type
description
source record(s)
timestamp(s)
feature/value
model contribution if applicable
confidence
```

This provides an audit trail.

---

# 29. Explainability

The key command:

```bash
bctx explain wallet W123
```

Example:

```text
WALLET W123
────────────────────────────────────────

RISK SCORE       91 / 100
CONFIDENCE       0.93

WHY FLAGGED
1. Transaction velocity is significantly
   above the learned baseline.

2. A peeling-chain pattern was detected
   across 6 hops.

3. Wallet belongs to Cluster #18.

4. Cluster has strong graph similarity with
   a high-risk seed entity.

5. New observed behavior increased the
   anomaly score by 0.11.

MODEL SIGNALS
anomaly            0.93
flow pattern       0.88
cluster similarity 0.81
network correlation 0.67
```

The output should be generated from stored evidence rather than a free-form language model inventing explanations.

---

# 30. Role of an Optional Local LLM

A local LLM may be useful for:

- summarizing already-computed evidence;
- converting structured evidence into a readable report;
- answering natural-language questions over the local case;
- explaining technical graph findings in simpler language.

However:

> **The local LLM must not be the source of truth for the risk score.**

The authoritative pipeline should be:

```text
Data
 ↓
Features
 ↓
ML / Graph Analysis
 ↓
Risk
 ↓
Evidence
 ↓
Optional local LLM summary
```

Not:

```text
Data
 ↓
LLM
 ↓
"Looks suspicious"
```

This preserves deterministic evidence and improves auditability.

---

# 31. Model Packaging

Models must be packaged for local inference.

Recommended lifecycle:

```text
Python Training Environment
        ↓
Train
        ↓
Evaluate
        ↓
Freeze model
        ↓
Export ONNX
        ↓
Model manifest
        ↓
Package with BCTX
        ↓
Local ONNX Runtime
```

Example:

```text
models/
├── anomaly.onnx
├── entity_cluster.onnx
├── flow_pattern.onnx
└── manifest.json
```

Manifest should include:

```json
{
  "model": "anomaly-detector",
  "version": "1.0.0",
  "input_schema": "v1",
  "training_dataset": "synthetic-v1",
  "feature_schema": "v1"
}
```

---

# 32. Model Update Strategy

Since offline execution is required, model updates cannot be required for normal operation.

Optional future mechanism:

```bash
bctx model update <local-package>
```

or:

```bash
bctx model import /media/usb/model-bundle.bctx
```

This allows an organization to transfer model updates through controlled offline media.

---

# 33. Geolocation / GeoIP

If the dataset includes IP metadata and geographic information is required, BCTX should use a **local GeoIP database** instead of making API calls during analysis.

Architecture:

```text
IP
 ↓
Local GeoIP Database
 ↓
Country / ASN / approximate region
 ↓
Graph / Evidence
```

The product should not imply exact physical location from an IP address.

---

# 34. Report Generation

Reports are a first-class feature.

Command:

```bash
bctx report wallet W123
```

Report should contain:

```text
Investigation ID
Case ID
Wallet / Entity
Risk score
Confidence
Transaction summary
Flow analysis
Entity cluster
Network observations
Anomaly findings
Pattern findings
Risk propagation
Evidence
Model metadata
Dataset snapshot
Analysis timestamp
```

Export formats:

```text
PDF
JSON
HTML
Markdown
```

JSON is important for machine-readable downstream processing.

---

# 35. Monitoring Architecture

The monitor should be an event-driven subsystem.

```text
External Data Stream
       ↓
Acquisition Adapter
       ↓
Local Event Queue
       ↓
Transaction Normalizer
       ↓
Graph Incremental Update
       ↓
Feature Update
       ↓
Local ML inference
       ↓
Risk Update
       ↓
Alert Manager
       ↓
TUI Event Stream
```

The monitor should support:

```bash
bctx monitor wallet W123
bctx monitor wallet W123 --interval 30s
bctx monitor wallet W123 --until 3h
```

Exact syntax can change.

---

# 36. Monitoring State

For each monitor session, persist:

```text
wallet
start time
last sync time
last seen transaction
transactions captured
risk before
risk after
alerts generated
session status
```

This allows:

```text
previous state
     +
new local events
     ↓
new analysis
     ↓
new state
```

---

# 37. Background Analysis

When the monitor receives new transactions:

```text
new transaction
      ↓
persist immediately
      ↓
lightweight feature update
      ↓
ML inference
      ↓
risk update
      ↓
UI refresh
```

Heavier operations may run asynchronously:

```text
full graph recomputation
deep cluster analysis
long-range path analysis
report generation
LLM summarization
```

The user should see task state:

```text
[RUNNING] anomaly analysis
[DONE]    graph update
[RUNNING] report generation
```

---

# 38. Case Management

BCTX should support multiple isolated investigations.

Example:

```bash
bctx case create case-001
bctx case open case-001
```

Filesystem:

```text
~/.bctx/cases/case-001/
├── case.db
├── evidence/
├── reports/
├── exports/
├── logs/
└── metadata.json
```

This provides a clean boundary between investigations.

---

# 39. Data Provenance

Every important record should preserve origin.

Example:

```text
source_type:
  explorer_api

source_identifier:
  provider-name

retrieved_at:
  timestamp

source_record_id:
  TX123

transformation_version:
  schema-v1
```

For imported files:

```text
source_type:
  csv

source_file:
  case01.csv

sha256:
  <hash>
```

The exact hash format is implementation-defined.

This is essential for reproducibility and auditability.

---

# 40. Offline Integrity

BCTX should provide:

```bash
bctx doctor
```

Example:

```text
BCTX SYSTEM CHECK

Executable              OK
Database                OK
Models                  OK
Model checksums          OK
GeoIP database           OK
Schema                   OK
Indexes                  OK

Network dependency test
  Analysis engine        PASS
  Graph engine           PASS
  ML engine              PASS
  Report engine          PASS

OFFLINE READY
```

This is a strong demo feature.

---

# 41. Security Architecture

Because the product is intended for potentially sensitive investigation datasets, security should be designed from the beginning.

## Requirements

- no external telemetry by default;
- no silent network calls from analysis components;
- local-only case data;
- permissions restricted to case directories;
- secrets/config kept separate from case data;
- model files checksum-verified;
- imported files tracked by hash;
- logs stored locally;
- report exports explicitly initiated;
- no automatic upload of investigation data.

The system should have an explicit "network capability" boundary.

---

# 42. No Hidden Internet Calls

BCTX should never secretly call external services.

Recommended:

```text
bctx doctor --network
```

to show:

```text
Acquisition providers:
  enabled / disabled

External endpoints:
  provider-A

Analysis network access:
  NONE
```

The offline analysis process should ideally be capable of running under a local OS network restriction for verification.

---

# 43. Installation Strategy

Primary installation:

```bash
curl -fsSL https://<official-domain>/install.sh | sh
```

The installer should:

1. identify Linux architecture;
2. download the correct BCTX release;
3. verify checksum/signature;
4. install binary;
5. install local model bundle;
6. initialize local directory;
7. run `bctx doctor`.

Then:

```bash
bctx
```

starts the application.

## Offline installation option

For truly isolated machines:

```text
Online machine
    ↓
Download BCTX release bundle
    ↓
USB / controlled transfer
    ↓
Offline Linux machine
    ↓
Local installer
```

This is important because an air-gapped machine obviously cannot execute an internet download itself.

---

# 44. Binary Distribution

The target should be:

```text
bctx-linux-amd64
bctx-linux-arm64
```

The production experience should feel like:

```bash
bctx
```

rather than:

```bash
python app.py
npm install
docker compose up
```

The user should not have to install:

```text
Python
Node.js
PostgreSQL
Redis
Docker
```

just to run the core product.

---

# 45. Internal Package Architecture

Suggested repository:

```text
bctx/
│
├── cmd/
│   └── bctx/
│       └── main.go
│
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
│   ├── config/
│   └── security/
│
├── pkg/
│   ├── schema/
│   ├── client/
│   └── sdk/
│
├── models/
│   ├── anomaly.onnx
│   ├── entity.onnx
│   └── flow.onnx
│
├── migrations/
│
├── scripts/
│
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── offline/
│   └── fixtures/
│
└── docs/
```

---

# 46. SDK Strategy

BCTX should be architected as a reusable engine rather than a CLI-only monolith.

Conceptually:

```go
engine := bctx.NewEngine(config)

case := engine.Cases.Open("case-001")

result := engine.InvestigateWallet("W123")

fmt.Println(result.RiskScore)
fmt.Println(result.Evidence)
```

The SDK should expose core capabilities such as:

```text
ingestion
storage
graph queries
feature extraction
model inference
risk scoring
evidence
reporting
```

The CLI/TUI becomes one client of the core engine.

This is important for future integrations.

---

# 47. Internal Service Boundaries

Do not begin with a microservice architecture.

Use modular boundaries inside one application.

Preferred:

```text
One BCTX Process
   ├── CLI/TUI
   ├── acquisition
   ├── storage
   ├── graph
   ├── ML
   ├── risk
   ├── reports
   └── monitor
```

Only split a component into a separate process if there is a concrete need such as:

- GPU model runtime isolation;
- long-running worker stability;
- resource isolation;
- separate language runtime.

The initial target is a single reliable Linux binary plus local data/model assets.

---

# 48. Python's Role

Python should be primarily a **training/research environment**, not the required end-user runtime.

Recommended:

```text
Python
├── dataset generation
├── preprocessing experiments
├── feature experiments
├── model training
├── evaluation
└── ONNX export

Go
├── CLI
├── TUI
├── storage
├── graph
├── orchestration
├── model inference
├── risk
└── reports
```

This keeps the production installation small and self-contained.

---

# 49. AI/ML Development Pipeline

```text
Synthetic Dataset
       ↓
Data Cleaning
       ↓
Feature Engineering
       ↓
Train / Validation / Test Split
       ↓
Model Training
       ↓
Evaluation
       ↓
Explainability Evaluation
       ↓
ONNX Export
       ↓
BCTX Integration
       ↓
Offline Inference Tests
```

Models should be versioned.

Example:

```text
anomaly-v1
anomaly-v2
entity-v1
flow-v1
```

---

# 50. Synthetic Dataset Strategy

The challenge states that the provided working dataset is synthetic and modeled on real Bitcoin P2P/transaction fields.

For development, BCTX should generate a controlled synthetic corpus containing:

## Normal behavior

Examples:

```text
ordinary transaction frequencies
typical amount distributions
ordinary wallet connectivity
normal time gaps
```

## Suspicious-like behavior patterns

Examples:

```text
high fan-out
rapid chains
peeling chains
mixing-like structures
unusual transaction velocity
clustered wallet behavior
cross-network correlation
```

Each synthetic event should have a ground-truth tag during development.

This enables:

```text
known pattern
     ↓
model
     ↓
prediction
     ↓
compare against ground truth
```

Ground-truth labels should not be exposed as investigator truth in the final UI unless they represent known synthetic test conditions.

---

# 51. Performance Requirements

Initial targets should be configurable based on benchmark hardware.

The engineering goals are:

### Startup

```text
BCTX startup should feel near-instant for a CLI tool.
```

### Wallet lookup

```text
indexed local wallet lookup should be interactive.
```

### Transaction lookup

```text
indexed TXID lookup should be interactive.
```

### Graph traversal

```text
small/medium subgraph traversal should be interactive.
```

### ML inference

```text
single wallet inference should complete locally
without cloud latency.
```

### Large ingestion

```text
bulk ingestion should be streaming and resumable.
```

Exact throughput targets should be decided after benchmarking the target machine.

---

# 52. Memory Strategy

Avoid loading a complete multi-million-row dataset into RAM.

Use:

```text
streaming ingestion
batched inserts
disk-backed indexes
incremental graph construction
lazy graph expansion
```

For investigation:

```text
query target
   ↓
load relevant neighborhood
   ↓
analyze subgraph
```

rather than:

```text
load entire graph
```

---

# 53. Monitoring Resource Strategy

Monitoring should not re-run expensive global analysis after every transaction.

Prefer:

```text
new event
  ↓
incremental features
  ↓
targeted model inference
  ↓
local risk update
```

and periodically:

```text
background deep analysis
```

This provides responsive terminal UX.

---

# 54. Alert System

Alerts should be ranked.

Example:

```text
ALERTS

#   TYPE             ENTITY       RISK
1   Peeling chain    W92831       94
2   Anomaly          TX98213      91
3   Cluster          W44129       89
4   Fan-out          W88211       83
```

Each alert should include:

```text
priority
risk score
confidence
reason
evidence references
created_at
case
status
```

Statuses:

```text
NEW
REVIEWING
DISMISSED
CONFIRMED
EXPORTED
```

"Confirmed" should mean investigator review state, not automatic legal determination.

---

# 55. Investigation View

Selecting an alert:

```bash
bctx alerts
```

then:

```text
Enter → Investigate
```

should open:

```text
ENTITY
├── Summary
├── Transactions
├── Graph
├── IP Activity
├── Timeline
├── Risk
├── Evidence
└── Report
```

This mirrors the visual concept shown in the current product sketches.

---

# 56. Timeline View

For any wallet/entity:

```text
TIME

10:31  TX001 +2.3 BTC
10:33  TX002 -0.4 BTC
10:34  TX003 -0.5 BTC
10:35  TX004 -0.5 BTC
10:36  TX005 -0.7 BTC
```

The timeline should allow filtering by:

```text
time range
transaction type
amount
risk
counterparty
IP
pattern
```

---

# 57. Graph View

The graph should support:

```text
depth
node types
edge types
risk filters
time filters
amount filters
cluster highlighting
```

Example:

```text
W123
 ├── TX001
 │    ├── W456
 │    └── W789
 │
 └── TX009
      └── W891
```

High-risk nodes should be visually distinguishable.

The TUI may use compact node representations in constrained terminal dimensions.

---

# 58. Network View

The network investigation view should provide:

```text
IP
Port
Timestamp
Related TX
Related Wallet
Country/ASN (if available locally)
```

Example:

```text
IP 10.1.2.3

Observed transactions: 34
Associated wallets: 8

Timeline
10:22 → TX001
10:24 → TX009
10:25 → TX012
```

Network correlation must always be based on the supplied/acquired metadata available to BCTX.

BCTX must not imply that every wallet has a reliably identifiable IP.

---

# 59. Search

Global search:

```bash
bctx search W123
bctx search TX123
bctx search 10.1.2.3
```

TUI search:

```text
/
```

Search indexes:

```text
wallet
TXID
IP
cluster
case
alert
```

---

# 60. Natural Language Investigation — Optional Future Layer

A future local AI agent could support:

```text
bctx> investigate everything around W123 that changed in the last 3 hours
```

The agent would translate this into deterministic tool calls:

```text
search wallet
→ fetch local transactions
→ expand graph
→ run anomaly
→ inspect patterns
→ calculate risk
→ produce evidence-backed summary
```

The agent should not bypass the underlying evidence engine.

The product should feel agentic while maintaining deterministic underlying analysis.

---

# 61. Report Update Workflow

For a monitoring session:

```text
OLD ANALYSIS
     ↓
NEW TRANSACTIONS
     ↓
INCREMENTAL ANALYSIS
     ↓
NEW RISK
     ↓
CHANGE EXPLANATION
     ↓
UPDATED REPORT
```

Example:

```text
Previous Risk: 63
Current Risk:  71
Change:        +8
```

Explanation:

```text
Risk increased because:
+ new high-velocity transaction pattern
+ additional graph connection
+ anomaly score changed from 0.72 → 0.84
```

---

# 62. Report Versioning

Every report should contain:

```text
report_id
case_id
entity
generated_at
dataset_snapshot
model_versions
risk_score
evidence_version
```

When new evidence arrives:

```text
report v1
report v2
report v3
```

This allows investigators to compare state over time.

---

# 63. Auditability

Important actions should generate local audit events:

```text
dataset imported
wallet analyzed
graph expanded
model executed
risk changed
report created
report exported
monitor started
monitor stopped
```

Audit logs must be local.

No external telemetry should be required.

---

# 64. Configuration

Configuration should be local:

```text
~/.config/bctx/config.toml
```

Potential settings:

```text
default case
data directory
model directory
provider configuration
monitor interval
risk thresholds
TUI preferences
report format
logging level
```

---

# 65. Configuration Profiles

Possible future profiles:

```text
offline
connected
airgap
development
demo
```

Example:

```bash
bctx --profile airgap
```

An `airgap` profile can explicitly disable all acquisition modules.

---

# 66. Failure Handling

BCTX must fail safely.

Examples:

### Network unavailable

```text
Acquisition failed.

Network unavailable.

Attempting local data...

LOCAL DATA AVAILABLE
```

### Data missing

```text
Wallet W123 is not available in the local case.

Run:
bctx sync wallet W123
```

### Model missing

```text
Required model anomaly-v1 is unavailable.

Analysis cannot continue with this model.

Run:
bctx model status
```

### Corrupt database

```text
Database integrity check FAILED.

Case data was not modified.

Recovery instructions:
bctx doctor
```

---

# 67. Offline Test Plan

Offline mode must be a formally tested feature.

Test:

```text
1. Install BCTX.
2. Acquire data.
3. Disable network interfaces.
4. Run:
     bctx wallet W123
     bctx transaction TX123
     bctx analyze wallet W123
     bctx alerts
     bctx graph wallet W123
     bctx explain wallet W123
     bctx report wallet W123
5. Verify every operation succeeds without network access.
```

Additionally run process-level network monitoring to verify the analysis path does not attempt external connections.

---

# 68. Reproducibility Test

Given:

```text
same local dataset
same model version
same configuration
```

the core analysis should produce reproducible results within defined numerical tolerance.

This is important for forensic workflows.

---

# 69. Model Evaluation Requirements

Do not evaluate the AI layer solely through a screenshot.

Measure:

```text
precision
recall
F1
ROC-AUC / PR-AUC where appropriate
false-positive rate
false-negative rate
latency
memory use
```

The best metric depends on the detection problem.

For highly imbalanced synthetic anomaly datasets, precision/recall and PR-AUC should be considered alongside other metrics.

---

# 70. Explainability Evaluation

For each prediction, verify:

```text
prediction
+
top contributing features
+
supporting evidence
+
model version
```

No explanation should reference information that was not actually used/generated by the analysis pipeline.

---

# 71. Security and Privacy Test Plan

Test for:

```text
unexpected external connections
secret leakage
case isolation
file permissions
path traversal in imports
malformed input handling
database corruption recovery
model file tampering
report export path validation
```

---

# 72. Packaging / Release Architecture

Release artifact:

```text
BCTX Linux Bundle
│
├── bctx
├── models/
├── checksums.txt
├── model-manifest.json
├── LICENSE
└── README
```

Installer:

```bash
curl -fsSL https://... | sh
```

should install:

```text
/usr/local/bin/bctx
```

or a user-local equivalent.

No Docker is required for the end-user installation.

---

# 73. Demo Dataset Package

For SIH demonstration, ship a local synthetic dataset package.

Example:

```text
demo/
├── synthetic-case.bctxdata
├── dataset-manifest.json
└── expected-findings.json
```

Demo mode:

```bash
bctx demo
```

should work without internet.

This allows judges to experience the entire workflow in a disconnected environment.

---

# 74. Demo Flow

The strongest demonstration sequence should be:

```bash
$ bctx
```

Then:

```text
BCTX
Offline Bitcoin Intelligence Platform

DATASET: demo-case
TRANSACTIONS: 1,248,732
WALLETS: 431,982
MODEL: READY
NETWORK: CONNECTED
```

Run:

```bash
bctx analyze wallet W1042
```

System:

```text
Fetching relevant data...
Persisting locally...
Building graph...
Running anomaly model...
Checking flow patterns...
Calculating risk...
Generating evidence...
```

Then:

```text
RISK SCORE: 93
CONFIDENCE: 0.94
```

Now turn Wi-Fi off.

Run:

```bash
bctx explain wallet W1042
```

It still works.

Then:

```bash
bctx report wallet W1042
```

It still works.

Then export:

```bash
bctx export report W1042 --format pdf
```

This demonstrates the offline claim clearly.

---

# 75. Monitor Demo Flow

Start while connected:

```bash
bctx monitor wallet W1042
```

New events appear:

```text
NEW TX
NEW TX
NEW TX
```

Risk changes:

```text
63 → 68 → 74
```

Then disconnect.

The monitor should show:

```text
NETWORK: DISCONNECTED
NEW INGESTION: PAUSED
LOCAL ANALYSIS: ACTIVE
```

The analyst can still run:

```bash
bctx explain wallet W1042
bctx report wallet W1042
bctx summarize monitor
```

The summary is generated from locally captured events.

---

# 76. Product UX Principle: "Connected is for Acquisition, Offline is for Intelligence"

This should be a major conceptual message.

```text
CONNECTED
──────────
Acquire
Sync
Monitor
Update

OFFLINE
───────
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

This is technically precise and avoids claiming that an air-gapped system can receive new blockchain events.

---

# 77. Required Components

## Mandatory MVP

### Runtime

- Go
- Linux support
- CLI
- TUI

### Storage

- SQLite
- local case filesystem

### Data

- CSV/JSON/XML ingestion
- normalized internal schema
- indexes

### Graph

- wallet/transaction graph
- basic IP/network relationship graph
- path traversal
- neighbors
- cluster representation

### AI/ML

- at least one working ML model;
- anomaly detection;
- model packaging locally;
- inference without internet.

### Detection

- anomaly detection;
- entity clustering;
- at least one suspicious flow/pattern detector;
- risk scoring.

### Explainability

- ranked alerts;
- confidence;
- reasons;
- evidence references.

### Reporting

- terminal report;
- JSON export;
- PDF/HTML export.

### Offline

- complete offline analysis;
- no analysis-time external API calls;
- offline smoke-test suite.

---

# 78. Strong MVP vs Future Features

## MVP

```text
bctx
dataset import
wallet lookup
transaction lookup
graph
anomaly detection
entity clustering
one flow detector
risk scoring
explain
alerts
report
offline mode
```

## Phase 2

```text
live monitor
incremental graph updates
risk delta
timeline
GeoIP
advanced graph analytics
HTML reports
```

## Phase 3

```text
local LLM
natural-language investigation
agentic orchestration
USB model/data bundles
multi-case management
advanced graph embeddings
```

---

# 79. Suggested Development Phases

## Phase 0 — Repository & Runtime

Build:

```text
Go project
CLI skeleton
TUI skeleton
config
logging
doctor
```

Deliverable:

```bash
bctx
```

works.

---

## Phase 1 — Data Engine

Build:

```text
schema
CSV parser
JSON parser
XML parser
SQLite
indexes
```

Deliverable:

```bash
bctx dataset import demo.csv
```

---

## Phase 2 — Transaction Graph

Build:

```text
wallet nodes
transaction nodes
edges
neighbor queries
path queries
```

Deliverable:

```bash
bctx graph wallet W123
```

---

## Phase 3 — Feature Engine

Calculate:

```text
frequency
volume
velocity
fan-in
fan-out
degree
time gaps
counterparty count
hop features
```

Deliverable:

```text
feature vectors
```

---

## Phase 4 — ML

Build one robust anomaly model first.

Then add:

```text
entity clustering
flow/pattern model
```

Export:

```text
ONNX
```

Deliverable:

```bash
bctx analyze wallet W123
```

---

## Phase 5 — Risk + Evidence

Build:

```text
risk engine
evidence model
explain engine
alert ranking
```

Deliverable:

```bash
bctx explain wallet W123
bctx alerts
```

---

## Phase 6 — TUI

Build:

```text
dashboard
graph view
wallet view
alerts
timeline
evidence panel
```

Deliverable:

```bash
bctx
```

feels like a full investigation workstation.

---

## Phase 7 — Monitoring

Build:

```text
provider adapter
live event ingestion
local event store
incremental risk
risk delta
summary
```

Deliverable:

```bash
bctx monitor wallet W123
```

---

## Phase 8 — Reporting

Build:

```text
Markdown
JSON
HTML
PDF
```

Deliverable:

```bash
bctx report wallet W123
```

---

## Phase 9 — Offline Hardening

Test:

```text
network disabled
all analysis functions
all model functions
all graph functions
all report functions
```

Deliverable:

> Proven offline operation.

---

## Phase 10 — Distribution

Build:

```text
single binary
model bundle
installer
checksums
demo dataset
```

Deliverable:

```bash
curl ... | sh
bctx
```

---

# 80. Example End-to-End Architecture

```text
                          INTERNET
                             │
                 ┌───────────▼───────────┐
                 │ DATA ACQUISITION ONLY │
                 │                       │
                 │ Explorer Adapter      │
                 │ Monitor Adapter       │
                 └───────────┬───────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Local Evidence  │
                    │ Store           │
                    └────────┬────────┘
                             │
                    NORMALIZE / INDEX
                             │
            ┌────────────────┼────────────────┐
            ▼                ▼                ▼
       Transactions        Wallets       Network Data
            │                │                │
            └────────────────┼────────────────┘
                             ▼
                    ┌─────────────────┐
                    │ Graph Engine    │
                    └────────┬────────┘
                             ▼
                    ┌─────────────────┐
                    │ Feature Engine  │
                    └────────┬────────┘
                             ▼
                    ┌─────────────────┐
                    │ Local ML        │
                    │ ONNX Runtime    │
                    └────────┬────────┘
                             ▼
                    ┌─────────────────┐
                    │ Risk Engine     │
                    └────────┬────────┘
                             ▼
                    ┌─────────────────┐
                    │ Evidence Engine │
                    └────────┬────────┘
                             │
                 ┌───────────┼───────────┐
                 ▼           ▼           ▼
                CLI         TUI        Reports
```

At this point:

```text
INTERNET = OFF
```

and everything below the acquisition boundary continues to function.

---

# 81. Final Product Architecture Rule

The single most important architectural rule is:

> **Never let the offline intelligence layer depend directly on an external service.**

Bad:

```text
Risk Engine
   ↓
Internet API
```

Good:

```text
Risk Engine
   ↓
Local Database
   ↓
Local Graph
   ↓
Local Models
```

Bad:

```text
Report
 ↓
Cloud LLM
```

Good:

```text
Report
 ↓
Evidence Engine
 ↓
Local report generator
```

Optional:

```text
Evidence
 ↓
Local LLM
 ↓
Human-readable summary
```

---

# 82. Final Product Definition

## BCTX

**BCTX is an offline-first, terminal-operated Bitcoin forensic intelligence platform.**

A user can:

```text
1. Install BCTX as a Linux binary.
2. Start it directly from the terminal.
3. Acquire relevant Bitcoin/network data while connected.
4. Persist that evidence locally.
5. Build a wallet/transaction/network graph locally.
6. Run AI/ML models locally.
7. Detect anomalies and suspicious transaction patterns.
8. Cluster related entities.
9. Calculate risk scores.
10. Inspect explainable evidence.
11. Monitor wallets while connected.
12. Store newly observed transactions locally.
13. Disconnect the machine.
14. Continue analyzing captured data completely offline.
15. Generate updated summaries and reports.
16. Export reports locally as PDF/HTML/JSON/Markdown.
```

The fundamental architecture is:

```text
              BCTX
               │
      ┌────────┴─────────┐
      │                  │
  ACQUISITION        INTELLIGENCE
  (network)            (offline)
      │                  │
      ▼                  ▼
 external data       local data
      │                  │
      └────────┬─────────┘
               ▼
          local evidence
               │
               ▼
             graph
               │
               ▼
              ML
               │
               ▼
             risk
               │
               ▼
           evidence
               │
        ┌──────┴──────┐
        ▼             ▼
       TUI          Report
```

### Final product goal

> **Turn raw Bitcoin transaction and network metadata into an offline, explainable investigation environment that helps an analyst discover suspicious patterns, understand relationships, prioritize cases, and preserve evidence-backed reports.**

---

# 83. Implementation Decision Summary

| Area | Decision |
|---|---|
| Product | BCTX |
| Product type | Offline-first Bitcoin intelligence/investigation workbench |
| Primary UX | Terminal CLI + TUI |
| Primary runtime | Go |
| ML training | Python |
| ML deployment | ONNX Runtime |
| Operational database | SQLite |
| Analytical database | DuckDB when needed |
| Graph | Local graph subsystem over persisted relational data/indexes |
| Input | CSV / JSON / XML + provider adapters |
| External network | Acquisition layer only |
| Offline inference | Mandatory |
| Local data | Mandatory |
| Local models | Mandatory |
| Reports | PDF / HTML / JSON / Markdown |
| Monitoring | Connected acquisition + local persistence + local analysis |
| Air-gapped analysis | Mandatory |
| SDK | Core engine exposed independently from CLI/TUI |
| Cloud dependency | None for offline intelligence |
| Container dependency | None for end-user runtime |
| Target OS | Linux |
| Model source of truth | Deterministic ML/graph/risk pipeline |
| Optional LLM | Local summarization / natural-language interface |
| Primary user | Investigation / intelligence analyst |

---

# 84. Definition of Done for the SIH Prototype

BCTX will be considered functionally complete when a fresh Linux environment can:

```bash
bctx
```

then:

```bash
bctx dataset import demo.csv
```

then:

```bash
bctx analyze wallet W123
```

and produce:

```text
risk score
confidence
transaction summary
graph relationships
anomaly result
pattern result
cluster result
evidence
```

Then the machine's network can be disabled and the following still work:

```bash
bctx wallet W123
bctx transaction TX123
bctx graph wallet W123
bctx explain wallet W123
bctx alerts
bctx report wallet W123
bctx export report W123 --format pdf
```

That offline demonstration is a central proof point for the product because the challenge explicitly requires a complete offline Linux solution.

