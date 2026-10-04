# BCTX — Workflow

## 1. Purpose of This Document

This document provides a **deep, step-by-step breakdown of the major workflows in BCTX**. It explains how data, analysis, evidence, risk, monitoring, and reports move through the system across different investigation use cases.

The goal is to:

- make the system behavior crystal clear;
- define exact step-by-step flows for implementation;
- remove ambiguity between connected and offline operation;
- help during debugging and demonstration;
- ensure every result can be traced back to local evidence.

This document is the **operational workflow layer** of BCTX.

---

## 2. Core Workflow Philosophy

All BCTX workflows follow one core pattern:

> **Intent → Data Availability Check → Acquisition (if required) → Local Persistence → Graph/Feature Analysis → AI/ML → Risk Assessment → Evidence → Output**

For monitoring, the flow becomes:

> **Monitor Intent → Acquire New Events → Persist → Incremental Analysis → Risk Update → Alert/Explanation → Local State**

The most important architectural rule is:

> **External network access is used only to acquire/sync data. Once the required data exists locally, investigation and intelligence workflows operate entirely from local state.**

---

## 3. Global Components Involved in BCTX

Most workflows involve some combination of the following components.

### 3.1 Investigator / Analyst

- starts BCTX;
- loads or syncs data;
- runs investigation commands;
- inspects graphs, alerts, evidence, and reports;
- controls monitor sessions.

### 3.2 BCTX CLI

- parses commands;
- validates arguments;
- invokes the appropriate core service;
- renders command output.

### 3.3 BCTX TUI

- provides the interactive terminal workspace;
- displays alerts, graph, timeline, evidence, and system state;
- sends user actions to the core engine.

### 3.4 Data Acquisition Layer

- retrieves required blockchain/network metadata while connected;
- supports provider adapters;
- handles pagination, retries, rate limits, and synchronization.

### 3.5 Local Evidence Store

- stores raw/normalized data;
- stores investigation state;
- preserves provenance;
- becomes the source for offline analysis.

### 3.6 Graph Engine

- models wallets, transactions, IP observations, and entities;
- performs relationship and path analysis.

### 3.7 Feature Engine

- derives ML and analytical features from local data.

### 3.8 Local ML Engine

- executes packaged local models;
- performs anomaly/entity/flow analysis;
- does not require internet access.

### 3.9 Risk Engine

- combines ML outputs and analytical signals;
- produces risk score and confidence.

### 3.10 Evidence Engine

- records why an entity or transaction was flagged;
- links findings to source records and model signals.

### 3.11 Report Engine

- turns investigation results into terminal, Markdown, JSON, HTML, and PDF output.

---

# 4. BCTX Operating Modes

BCTX has three conceptual runtime states.

## 4.1 Connected Mode

Used for:

- initial data acquisition;
- wallet synchronization;
- transaction synchronization;
- monitoring new external events.

```text
NETWORK: CONNECTED
ACQUISITION: AVAILABLE
LOCAL ANALYSIS: AVAILABLE
```

---

## 4.2 Offline Mode

Used for:

- wallet lookup;
- transaction lookup;
- graph traversal;
- anomaly detection;
- clustering;
- suspicious-flow analysis;
- risk scoring;
- explainability;
- reports;
- local summarization.

```text
NETWORK: DISCONNECTED
ACQUISITION: UNAVAILABLE
LOCAL ANALYSIS: AVAILABLE
```

---

## 4.3 Air-Gapped Mode

Strict security mode where the application intentionally disables acquisition/network functionality.

```text
NETWORK: DISCONNECTED
ACQUISITION: DISABLED
LOCAL ANALYSIS: AVAILABLE
```

---

# 5. Application Startup Workflow

## Command

```bash
bctx
```

### Step 1: Binary Starts

```text
Linux
  ↓
bctx binary
```

### Step 2: Load Configuration

BCTX reads local configuration.

```text
~/.config/bctx/config.toml
```

### Step 3: Resolve Local Directories

```text
~/.bctx/
├── cases/
├── models/
├── geo/
├── logs/
└── cache/
```

### Step 4: Validate Runtime

Check:

- database;
- model availability;
- model checksums;
- schema version;
- indexes;
- report engine.

### Step 5: Detect Network State

Display:

```text
NETWORK: CONNECTED
```

or:

```text
NETWORK: OFFLINE
```

### Step 6: Load Current Case

If a default case exists:

```text
CASE: case-001
```

### Step 7: Start TUI / Prompt

```text
BCTX — Bitcoin Forensic Intelligence Platform

bctx>
```

or interactive TUI.

---

# 6. `bctx doctor` Workflow

## Command

```bash
bctx doctor
```

### Steps

1. Check executable.
2. Check configuration.
3. Check local directories.
4. Open database.
5. Validate schema.
6. Validate indexes.
7. Check graph subsystem.
8. Verify local models.
9. Verify model checksums.
10. Check report generation.
11. Check local GeoIP database if enabled.
12. Perform offline analysis test.
13. Report network dependency status.

### Example

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
[PASS] Reports
[PASS] Offline Analysis

BCTX READY
```

---

# 7. DATASET IMPORT WORKFLOW

## Command

```bash
bctx dataset import dataset.csv
```

Supported formats:

```text
CSV
JSON
XML
```

### Step 1: Validate Input

Check:

- file exists;
- extension/format;
- readable permissions;
- schema;
- required fields.

### Step 2: Streaming Parse

Do not load an entire large dataset into memory.

```text
File
 ↓
Streaming Parser
```

### Step 3: Normalize

Convert source format into BCTX canonical structures:

```text
Transaction
Wallet
NetworkObservation
```

### Step 4: Validate Records

Validate:

```text
timestamp
TXID
wallet addresses
amounts
IP
ports
```

### Step 5: Persist

```text
Normalized Data
      ↓
SQLite
```

### Step 6: Update Indexes

```text
wallet index
TXID index
IP index
timestamp index
```

### Step 7: Update Graph

Create or update:

```text
wallet nodes
transaction nodes
IP nodes
edges
```

### Step 8: Record Provenance

Store:

```text
source file
SHA-256
import timestamp
schema version
record counts
```

### Step 9: Produce Import Summary

```text
Records read: 2,481,392
Valid:        2,475,820
Rejected:         5,572

Transactions: 1,104,229
Wallets:        384,912
Network records:723,441
```

---

# 8. WALLET ANALYSIS WORKFLOW

## Command

```bash
bctx analyze wallet W123
```

This is the primary BCTX workflow.

---

## Stage A — Data Availability Check

BCTX first checks:

```text
Is W123 already present locally?
```

### Case 1: Data is complete locally

Continue directly:

```text
Local Data
 ↓
Analysis
```

### Case 2: Data is missing and acquisition is allowed

Continue to acquisition.

### Case 3: Data is missing and offline mode is forced

Stop gracefully:

```text
Required local data is unavailable.

Offline analysis cannot retrieve missing external data.
```

---

# 9. WALLET DATA ACQUISITION WORKFLOW

When connected and acquisition is permitted:

```text
Wallet ID
   ↓
Data Acquisition Adapter
   ↓
External Blockchain Data Source
```

The acquisition layer retrieves the relevant available information needed for the investigation.

Possible data:

```text
transaction history
transaction details
inputs
outputs
amounts
fees
timestamps
related records
available network metadata
```

### Important Rule

The acquisition layer must not pass remote data directly to ML.

Correct:

```text
External Source
 ↓
Normalize
 ↓
Persist Locally
 ↓
Analyze
```

---

# 10. DATA PAGINATION WORKFLOW

Historical transaction APIs may return data in pages.

The acquisition adapter should:

1. request first page;
2. store results;
3. identify continuation;
4. request next page;
5. deduplicate;
6. continue until the relevant history is exhausted or the requested limit is reached;
7. persist acquisition state.

```text
Page 1
 ↓
Local Store

Page 2
 ↓
Local Store

Page 3
 ↓
Local Store
...
```

If acquisition is interrupted:

```text
resume from last successful checkpoint
```

---

# 11. LOCAL PERSISTENCE WORKFLOW

Every acquired record becomes local evidence.

```text
External Record
      ↓
Canonical Record
      ↓
Database Transaction
      ↓
Indexes
      ↓
Graph
```

Store provenance such as:

```text
source
retrieved_at
source_identifier
schema_version
record_hash where applicable
```

---

# 12. GRAPH UPDATE WORKFLOW

After persistence:

```text
Transaction
   ↓
Input Wallets
   ↓
Output Wallets
   ↓
Network Observations
```

Graph entities are created or updated.

Example:

```text
IP-01
  │
  ▼
TX-100
  │
  ├────→ W123
  │
  └────→ W456
```

---

# 13. FEATURE EXTRACTION WORKFLOW

For the requested wallet/entity:

```text
Local Evidence
      ↓
Transaction Features
      ↓
Temporal Features
      ↓
Graph Features
      ↓
Flow Features
      ↓
Network Features
      ↓
Feature Vector
```

Example:

```text
transaction_count = 231
outgoing_volume = 71.14
fan_out = 24
median_time_gap = 8.2 sec
graph_degree = 41
hop_count = 6
```

---

# 14. ML ANALYSIS WORKFLOW

The feature vector is passed to local ML models.

```text
Feature Vector
      ↓
Local ONNX Runtime
      ↓
Anomaly Model
      ↓
Anomaly Score
```

Additional models/detectors may then process the local graph/features:

```text
Entity Model
Flow Model
Pattern Detectors
```

No external ML API is called.

---

# 15. ENTITY CLUSTERING WORKFLOW

For wallet/entity clustering:

```text
Wallet Behavior
       ↓
Common-input heuristics
       ↓
Graph/behavior features
       ↓
Clustering / graph embeddings
       ↓
Cluster assignment
```

Output:

```text
Cluster #18
Members:
W123
W456
W789
W821

Confidence: 0.87
```

The result represents an inferred relationship, not proof of real-world ownership.

---

# 16. ANOMALY DETECTION WORKFLOW

```text
Wallet
 ↓
Feature Extraction
 ↓
Local Anomaly Model
 ↓
Anomaly Score
```

Example:

```text
Anomaly Score: 0.93
```

The system should retain:

```text
model version
feature schema
prediction
important signal values
timestamp
```

---

# 17. PEELING-CHAIN DETECTION WORKFLOW

```text
Wallet / Flow
      ↓
Transaction sequence
      ↓
Value distribution
      ↓
Repeated split/residual movement
      ↓
Timing analysis
      ↓
Chain scoring
```

Output:

```text
Pattern:
Peeling-chain-like behavior

Confidence:
0.88

Evidence:
6-hop transaction sequence
Repeated residual continuation
Rapid time intervals
```

---

# 18. MIXING-LIKE PATTERN WORKFLOW

```text
Transaction graph
      ↓
Input/output structure
      ↓
Participant count
      ↓
Value distribution
      ↓
Timing
      ↓
Graph pattern analysis
```

Output:

```text
Mixing-like pattern detected
Score: 0.81
```

Use wording such as:

```text
mixing-like
```

rather than treating pattern detection as proof of wrongdoing.

---

# 19. RISK CALCULATION WORKFLOW

After individual analyses:

```text
Anomaly
   +
Cluster
   +
Flow Pattern
   +
Graph Signals
   +
Network Correlation
   +
Risk Propagation
        ↓
   Risk Engine
        ↓
Risk Score
Confidence
Signal Breakdown
```

Example:

```text
Risk Score: 91/100
Confidence: 0.93
```

---

# 20. RISK PROPAGATION WORKFLOW

Suppose a seed entity has elevated risk.

```text
Seed Entity
Risk = 1.00
   │
   ├── W2
   │
   └── W3
```

The risk engine can calculate graph-based propagated risk.

Conceptually:

```text
distance 0 → 1.00
distance 1 → reduced
distance 2 → further reduced
distance 3 → further reduced
```

Every propagated score must record:

```text
seed source
path
distance
contribution
```

---

# 21. EVIDENCE GENERATION WORKFLOW

After risk calculation:

```text
Risk Signals
      ↓
Evidence Engine
      ↓
Evidence Items
```

Example:

```text
E1 — Transaction anomaly
E2 — Peeling-chain-like pattern
E3 — Cluster relationship
E4 — Graph proximity
E5 — Network correlation
```

Each evidence item references the underlying source records.

---

# 22. EXPLAIN WORKFLOW

## Command

```bash
bctx explain wallet W123
```

### Steps

1. Load local risk assessment.
2. Load signal breakdown.
3. Load evidence.
4. Load relevant source records.
5. Build explanation.
6. Render explanation.

Example:

```text
WHY FLAGGED

1. Transaction velocity is above the learned baseline.
2. Peeling-chain-like flow detected across 6 hops.
3. Wallet is a member of Cluster #18.
4. Connected to a high-risk seed through the graph.
5. Network observations correlate with relevant transactions.

Risk: 91
Confidence: 0.93
```

---

# 23. ALERT GENERATION WORKFLOW

After risk calculation:

```text
Risk Assessment
      ↓
Alert Threshold / Priority Logic
      ↓
Alert
```

Alert contains:

```text
entity
type
risk
confidence
reason
evidence
timestamp
case
status
```

---

# 24. ALERT RANKING WORKFLOW

```text
All Alerts
    ↓
Risk
    ↓
Confidence
    ↓
Recency / Priority
    ↓
Ranked Queue
```

Example:

```text
# TYPE          ENTITY     RISK
1 Pattern       W123       94
2 Anomaly       TX99       91
3 Cluster       W441       89
```

The ranking is for investigation prioritization.

---

# 25. WALLET LOOKUP WORKFLOW

## Command

```bash
bctx wallet W123
```

### Steps

```text
CLI
 ↓
Wallet Index
 ↓
Local Database
 ↓
Wallet Record
 ↓
Recent / Historical Transactions
 ↓
Summary
```

No graph or ML recomputation is necessary unless requested.

---

# 26. TRANSACTION LOOKUP WORKFLOW

## Command

```bash
bctx transaction TX123
```

### Steps

```text
TXID
 ↓
TX Index
 ↓
Local transaction record
 ↓
Inputs
 ↓
Outputs
 ↓
Network observations
 ↓
Related graph nodes
```

---

# 27. IP LOOKUP WORKFLOW

## Command

```bash
bctx ip 10.1.2.3
```

### Steps

```text
IP
 ↓
Local IP index
 ↓
Network observations
 ↓
Related transactions
 ↓
Related wallets/entities
 ↓
Timeline
```

If local GeoIP data is available:

```text
IP
 ↓
Local MMDB
 ↓
Country / ASN / approximate region
```

---

# 28. GRAPH INVESTIGATION WORKFLOW

## Command

```bash
bctx graph wallet W123 --depth 3
```

### Steps

```text
Wallet
 ↓
Graph Query
 ↓
N-hop traversal
 ↓
Subgraph extraction
 ↓
Filters
 ↓
TUI rendering
```

Possible filters:

```text
risk
time
amount
node type
edge type
cluster
```

---

# 29. PATH ANALYSIS WORKFLOW

## Command

```bash
bctx path W123 W900
```

### Steps

```text
Source
 ↓
Graph traversal
 ↓
Candidate paths
 ↓
Path scoring/filtering
 ↓
Result
```

Output example:

```text
W123
 ↓
W456
 ↓
W821
 ↓
W900

Hops: 3
```

---

# 30. CASE CREATION WORKFLOW

## Command

```bash
bctx case create case-001
```

### Steps

```text
Create case metadata
 ↓
Create database
 ↓
Create directories
 ↓
Set active case
```

Filesystem:

```text
~/.bctx/cases/case-001/
├── case.db
├── evidence/
├── imports/
├── reports/
├── exports/
└── logs/
```

---

# 31. CASE OPEN WORKFLOW

## Command

```bash
bctx case open case-001
```

### Steps

```text
Load case metadata
 ↓
Open database
 ↓
Validate schema
 ↓
Load indexes
 ↓
Load case context
 ↓
Set active case
```

---

# 32. REPORT GENERATION WORKFLOW

## Command

```bash
bctx report wallet W123
```

### Steps

```text
Wallet
 ↓
Investigation Result
 ↓
Risk
 ↓
Graph Summary
 ↓
Pattern Findings
 ↓
Network Findings
 ↓
Evidence
 ↓
Model Metadata
 ↓
Dataset Metadata
 ↓
Report Object
```

Then:

```text
Report Object
 ↓
Terminal renderer
```

or:

```text
Report Object
 ↓
JSON / Markdown / HTML / PDF exporter
```

---

# 33. REPORT EXPORT WORKFLOW

## Command

```bash
bctx export report W123 --format pdf
```

### Steps

```text
Load latest investigation report
 ↓
Render PDF representation
 ↓
Generate local PDF
 ↓
Save to requested path
 ↓
Record export audit event
```

No cloud service is required.

---

# 34. MONITOR WORKFLOW

## Command

```bash
bctx monitor wallet W123
```

### Stage 1: Initialize

```text
Wallet
 ↓
Load existing local state
 ↓
Determine last known transaction
 ↓
Create monitor session
```

### Stage 2: Acquire New Data

While connected:

```text
External Data Source
 ↓
Monitor Adapter
 ↓
New Transaction/Event
```

### Stage 3: Persist

```text
New Event
 ↓
Normalize
 ↓
SQLite
```

### Stage 4: Update Graph

```text
New Transaction
 ↓
Graph Increment
```

### Stage 5: Incremental Analysis

```text
New Event
 ↓
Feature Update
 ↓
Local ML
 ↓
Pattern Detector
```

### Stage 6: Risk Update

```text
Previous Risk
      +
New Signals
      ↓
Current Risk
```

### Stage 7: Alert

If the new state produces an alert:

```text
Risk Update
 ↓
Alert Manager
 ↓
TUI
```

---

# 35. MONITOR TERMINAL VIEW

Example:

```text
MONITORING W123

12:41:02  TX a81f...   +0.82 BTC
12:41:05  TX 92bc...   -0.41 BTC
12:41:11  TX 81ae...   -0.39 BTC

Previous Risk: 63
Current Risk:  71
Change:        +8

Status:
NETWORK: CONNECTED
LOCAL STORE: SYNCED
ANALYSIS: RUNNING
```

---

# 36. MONITOR DISCONNECT WORKFLOW

If the machine loses network access:

```text
Network unavailable
       ↓
Stop acquisition
       ↓
Preserve local data
       ↓
Continue local analysis
```

UI:

```text
NETWORK: DISCONNECTED
ACQUISITION: PAUSED
LOCAL ANALYSIS: AVAILABLE
LAST EVENT: 14:05:18
```

Do not claim that new external transactions are being received.

---

# 37. MONITOR RECONNECT WORKFLOW

When connectivity returns:

```text
Network restored
       ↓
Find last persisted event
       ↓
Request missing/new data
       ↓
Deduplicate
       ↓
Persist
       ↓
Update graph
       ↓
Run incremental analysis
       ↓
Update risk
```

The monitor should recover from transient network failures without rebuilding the entire case.

---

# 38. MONITOR SUMMARY WORKFLOW

## Command

```bash
bctx summarize monitor
```

### Steps

```text
Monitor Session
 ↓
Local transaction events
 ↓
Risk changes
 ↓
Detected patterns
 ↓
Alerts
 ↓
Evidence
 ↓
Summary
```

Optional future stage:

```text
Structured Evidence
 ↓
Local LLM
 ↓
Narrative Summary
```

---

# 39. OLD + NEW REPORT WORKFLOW

When monitoring produces new evidence:

```text
Previous Report
      ↓
New Local Events
      ↓
Re-analysis
      ↓
New Risk
      ↓
Risk Delta
      ↓
Updated Evidence
      ↓
New Report Version
```

Example:

```text
Risk:
63 → 71

Change:
+8

New signals:
• higher transaction velocity
• new graph relationship
• increased anomaly score
```

---

# 40. OFFLINE ANALYSIS WORKFLOW

## Command

```bash
bctx analyze wallet W123 --offline
```

The command must execute:

```text
CLI
 ↓
Local Repository
 ↓
Local Graph
 ↓
Local Feature Engine
 ↓
Local ML Models
 ↓
Local Risk Engine
 ↓
Local Evidence
 ↓
Terminal Report
```

No external data acquisition occurs.

---

# 41. OFFLINE EXPLAIN WORKFLOW

```bash
bctx explain wallet W123
```

The system reads only:

```text
local database
local graph
local model outputs
local evidence
local configuration
```

and produces the explanation.

---

# 42. OFFLINE REPORT WORKFLOW

```bash
bctx report wallet W123
```

Everything is local:

```text
Local case
 ↓
Investigation result
 ↓
Evidence
 ↓
Report generator
 ↓
PDF/HTML/JSON/Markdown
```

---

# 43. OFFLINE SEARCH WORKFLOW

```bash
bctx search W123
```

Uses:

```text
local indexes
local SQLite/DuckDB
```

No remote search provider is required.

---

# 44. OFFLINE MODEL WORKFLOW

```text
Feature Vector
      ↓
Local ONNX Runtime
      ↓
Local .onnx model
      ↓
Prediction
```

The system does not:

```text
download model
call cloud inference
query hosted AI
```

---

# 45. OPTIONAL LOCAL LLM WORKFLOW

Future command:

```bash
bctx summarize wallet W123
```

Pipeline:

```text
Local Investigation
 ↓
Structured Evidence
 ↓
Local LLM
 ↓
Summary
```

The local LLM is a presentation/summarization layer.

The authoritative source remains:

```text
Graph
ML
Risk
Evidence
```

---

# 46. DATA SYNC WORKFLOW

## Command

```bash
bctx sync wallet W123
```

### Steps

```text
Check network
 ↓
Load provider adapter
 ↓
Request new/missing data
 ↓
Paginate
 ↓
Deduplicate
 ↓
Validate
 ↓
Normalize
 ↓
Persist
 ↓
Update indexes
 ↓
Update graph
 ↓
Recalculate affected analysis
```

---

# 47. MODEL UPDATE WORKFLOW

Models should not need internet at runtime.

Optional local update:

```bash
bctx model import /media/usb/model-bundle
```

### Steps

```text
Read package
 ↓
Verify checksum/signature
 ↓
Verify model manifest
 ↓
Verify feature schema compatibility
 ↓
Install model
 ↓
Run validation
 ↓
Activate version
```

---

# 48. GEOIP WORKFLOW

When an IP needs enrichment:

```text
IP
 ↓
Local MMDB
 ↓
Country
ASN
Approximate region
 ↓
Evidence / Graph
```

No online GeoIP lookup is required.

---

# 49. AUDIT WORKFLOW

Important operations create local audit events.

Example:

```text
DATASET_IMPORTED
WALLET_ANALYZED
GRAPH_EXPANDED
MODEL_EXECUTED
RISK_UPDATED
ALERT_CREATED
REPORT_GENERATED
REPORT_EXPORTED
MONITOR_STARTED
MONITOR_STOPPED
```

Each event records:

```text
timestamp
case
action
subject
result
```

---

# 50. ERROR WORKFLOW — DATA MISSING

Example:

```bash
bctx analyze wallet W999
```

If offline and absent locally:

```text
W999 is not available in local evidence.

Offline analysis cannot acquire missing data.

Suggested action:
sync W999 while connected.
```

---

# 51. ERROR WORKFLOW — NETWORK FAILURE

During acquisition:

```text
Provider request failed
       ↓
Retry
       ↓
If still unavailable:
       ↓
Save completed data
       ↓
Preserve checkpoint
       ↓
Return partial-sync state
```

Example:

```text
Sync incomplete.

Transactions acquired: 8,241
Checkpoint saved.

Resume with:
bctx sync resume case-001
```

---

# 52. ERROR WORKFLOW — MODEL FAILURE

If the model is missing/incompatible:

```text
Feature preparation
 ↓
Model validation
 ↓
FAIL
```

Output:

```text
MODEL ERROR

Required model:
anomaly-detector v1.0.0

Status:
MISSING / INCOMPATIBLE

No risk score was generated.
```

Do not silently fall back to an unvalidated model.

---

# 53. ERROR WORKFLOW — DATABASE FAILURE

```text
Database open
 ↓
Integrity check
 ↓
FAIL
```

BCTX should:

- prevent destructive operations;
- preserve existing data;
- report recovery information;
- allow a backup/restore workflow.

---

# 54. SECURITY WORKFLOW

At runtime:

```text
User Command
 ↓
Input Validation
 ↓
Authorization to local case path
 ↓
Operation
 ↓
Audit Event
```

File imports should be checked for:

```text
path traversal
symlinks where unsafe
oversized input
malformed structure
```

---

# 55. FULL `ANALYZE` WORKFLOW

The complete workflow is:

```text
User
 ↓
bctx analyze wallet W123
 ↓
Check Local Data
 ↓
Missing?
 ├── NO → continue
 └── YES → Acquisition if permitted
                ↓
          Normalize
                ↓
          Persist Locally
                ↓
          Index
                ↓
          Graph Update
                ↓
          Feature Extraction
                ↓
          Anomaly Model
                ↓
          Entity Analysis
                ↓
          Flow Detection
                ↓
          Risk Propagation
                ↓
          Risk Engine
                ↓
          Evidence Engine
                ↓
          Alert Manager
                ↓
          Investigation Result
                ↓
        Terminal/TUI Output
                ↓
             Report
```

---

# 56. FULL `MONITOR` WORKFLOW

```text
User
 ↓
bctx monitor wallet W123
 ↓
Load local state
 ↓
Start monitor session
 ↓
Connected?
 ├── YES → acquire events
 │           ↓
 │       persist locally
 │           ↓
 │       update graph
 │           ↓
 │       incremental features
 │           ↓
 │       local ML
 │           ↓
 │       risk update
 │           ↓
 │       evidence
 │           ↓
 │       alerts
 │           ↓
 │       TUI
 │
 └── NO → local analysis of captured data
```

---

# 57. FULL OFFLINE WORKFLOW

```text
Machine
  ↓
NETWORK OFF
  ↓
bctx
  ↓
Local Case
  ↓
Local Database
  ↓
Local Index
  ↓
Local Graph
  ↓
Local Feature Engine
  ↓
Local ONNX Models
  ↓
Risk Engine
  ↓
Evidence Engine
  ↓
TUI / Reports
```

No external service is required for this path.

---

# 58. FULL REPORT UPDATE WORKFLOW

```text
Existing Investigation
       ↓
New Captured Transactions
       ↓
Local Persistence
       ↓
Graph Increment
       ↓
Feature Delta
       ↓
ML Delta
       ↓
Risk Delta
       ↓
New Evidence
       ↓
Updated Report
       ↓
Versioned Export
```

---

# 59. FINAL DEMO WORKFLOW

The preferred SIH demonstration sequence:

### Step 1

```bash
bctx
```

Show:

```text
BCTX
Dataset: demo-case
Models: READY
Graph: READY
```

### Step 2

```bash
bctx analyze wallet W1042
```

Show:

```text
Fetching relevant data...
Persisting...
Building graph...
Running ML...
Calculating risk...
```

### Step 3

Show:

```text
Risk Score: 93
Confidence: 0.94
```

### Step 4

```bash
bctx explain wallet W1042
```

Show evidence.

### Step 5

```bash
bctx report wallet W1042
```

Generate the investigation report.

### Step 6

```bash
bctx export report W1042 --format pdf
```

Export locally.

### Step 7

Disconnect the network.

### Step 8

```bash
bctx analyze wallet W1042 --offline
```

Show that analysis still works.

### Step 9

```bash
bctx explain wallet W1042
bctx report wallet W1042
```

Show that explanation/reporting still work.

### Step 10

Reconnect if required.

```bash
bctx monitor wallet W1042
```

Show new events and risk changes.

---

# 60. Final Workflow Architecture

```text
                          USER
                           │
                           ▼
                    ┌─────────────┐
                    │ BCTX CLI/TUI│
                    └──────┬──────┘
                           │
                  ┌────────▼────────┐
                  │ Command Engine  │
                  └────────┬────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
       Acquisition      Local Data      Monitor
       (connected)       Store           Events
            │              │              │
            └──────────────┼──────────────┘
                           ▼
                     Normalization
                           │
                           ▼
                       Indexing
                           │
                           ▼
                     Graph Engine
                           │
                           ▼
                     Feature Engine
                           │
                           ▼
                    Local ML Engine
                           │
                           ▼
                      Risk Engine
                           │
                           ▼
                    Evidence Engine
                           │
                 ┌─────────┴─────────┐
                 ▼                   ▼
               Alerts              Reports
                 │                   │
                 ▼                   ▼
                TUI             PDF/HTML/JSON
```

---

# 61. One-Line Workflow Summary

> **BCTX acquires relevant Bitcoin/network data when connected, stores it locally, transforms it into a searchable transaction/entity graph, runs local AI/ML and risk analysis, produces evidence-backed alerts and reports, and continues all intelligence and reporting workflows offline after the required data has been captured.**

---

# 62. Core Rule to Remember

The entire BCTX system can be understood as two boundaries:

```text
                 CONNECTED
                     │
          ┌──────────▼──────────┐
          │  DATA ACQUISITION   │
          │  Sync / Monitoring  │
          └──────────┬──────────┘
                     │
               LOCAL EVIDENCE
                     │
          ─── OFFLINE BOUNDARY ───
                     │
          ┌──────────▼──────────┐
          │     INTELLIGENCE    │
          │                     │
          │ Graph               │
          │ Features            │
          │ AI/ML               │
          │ Risk                │
          │ Evidence            │
          │ Alerts              │
          │ Summaries           │
          │ Reports             │
          └─────────────────────┘
```

> **Data comes in through a controlled acquisition path. Intelligence happens locally.**
