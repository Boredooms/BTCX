# BCTX — Idea

## 1. What Is BCTX?

**BCTX** is an **offline-first Bitcoin forensic intelligence and investigation platform** designed to run primarily from the terminal.

The product is inspired by the interaction model of tools such as Claude Code: instead of forcing an investigator through a traditional web dashboard, BCTX provides a command-driven investigation environment with an interactive terminal UI.

The core idea is:

> **Give an investigator a Bitcoin wallet, transaction, entity, or local dataset, and help them acquire the relevant data, understand its relationships, detect suspicious patterns, calculate risk, explain the findings, and generate a detailed report — with all intelligence processing capable of running locally and offline.**

---

# 2. The Problem

Bitcoin creates a large amount of transaction and network data.

An investigator may need to reason about:

```text
Wallets
Transactions
Inputs
Outputs
Amounts
Fees
Timestamps
IP addresses
Ports
Network observations
Entity relationships
Transaction flows
```

The problem is not simply that the data exists.

The problem is that the useful evidence is distributed across a large and connected dataset.

A single transaction may not look unusual.

A sequence of transactions may be unusual.

A wallet may not look suspicious in isolation.

A cluster of connected wallets may reveal a much more interesting pattern.

A network observation by itself may be meaningless.

A combination of:

```text
IP
+
timestamp
+
transaction
+
wallet
+
graph relationship
+
behavior
```

can provide much more context.

The system therefore needs to turn raw records into an investigation environment.

---

# 3. The Core Question BCTX Answers

Instead of asking:

> "What happened in this one transaction?"

BCTX aims to help answer:

> **"What is happening around this wallet/entity, how is it connected to other activity, what looks unusual, what evidence supports that finding, and what should an analyst investigate further?"**

---

# 4. The Main End Product

The final product is a Linux terminal application:

```bash
bctx
```

The experience should feel like a professional investigation shell.

Example:

```text
$ bctx

BCTX
BITCOIN FORENSIC INTELLIGENCE PLATFORM

Network:      CONNECTED
Local Data:   READY
Models:       READY
Graph:        READY

bctx>
```

The user can then work using commands such as:

```bash
bctx analyze wallet <WALLET_ID>
bctx monitor wallet <WALLET_ID>
bctx wallet <WALLET_ID>
bctx transaction <TXID>
bctx graph wallet <WALLET_ID>
bctx alerts
bctx explain wallet <WALLET_ID>
bctx report wallet <WALLET_ID>
```

The system can also expose the same functionality through an interactive TUI.

---

# 5. The Most Important Product Concept

BCTX has **two major worlds**:

```text
                    BCTX
                     │
          ┌──────────┴──────────┐
          │                     │
      ACQUISITION           INTELLIGENCE
      / SYNCHRONIZATION       ENGINE
          │                     │
      may require            local/offline
       network                  │
          │                     │
          └──────────┬──────────┘
                     ▼
               LOCAL EVIDENCE
```

The key principle is:

> **The network is for acquiring data. The intelligence engine is for understanding the data.**

After the required information is stored locally, the machine can be disconnected.

The investigator can continue to:

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

without requiring internet access.

---

# 6. Why Offline Is Central

The product is designed around local processing.

The user should be able to:

```text
1. Install BCTX.
2. Acquire/sync the relevant data while connected.
3. Store the data locally.
4. Build the graph locally.
5. Run AI/ML locally.
6. Generate risk and evidence locally.
7. Disconnect the network.
8. Continue investigating the captured data.
```

The system must not depend on:

```text
Cloud AI APIs
Cloud databases
Online graph services
Online report generation
External summarization APIs
```

for its intelligence functions.

---

# 7. An Important Physical Limitation

An offline machine cannot receive genuinely new blockchain transactions from the external network.

Therefore BCTX distinguishes between:

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

When disconnected, BCTX analyzes everything that has already been captured locally.

The interface should make this state explicit:

```text
NETWORK: DISCONNECTED
LAST SYNC: 14:05:18
NEW NETWORK DATA: PAUSED
LOCAL ANALYSIS: READY
```

---

# 8. Primary User

The primary user is an:

> **Investigation / intelligence analyst working with Bitcoin transaction and network metadata.**

Their questions may include:

```text
Which wallets look unusual?

Which wallets are related?

Where did funds move?

What patterns exist around this wallet?

Which entities should I inspect?

Why was this entity flagged?

How did its risk score change?

Can I produce a report containing the evidence?
```

BCTX is designed to reduce the amount of manual work needed to answer those questions.

---

# 9. Example: The Wallet Investigation

Suppose the user has:

```text
W123
```

They execute:

```bash
bctx analyze wallet W123
```

BCTX first checks local data.

If the required information is missing and acquisition is permitted:

```text
W123
 ↓
data acquisition
 ↓
transaction history
 ↓
transaction details
 ↓
relevant available metadata
 ↓
local storage
```

After the data is stored locally:

```text
local data
 ↓
graph
 ↓
features
 ↓
ML
 ↓
risk
 ↓
evidence
 ↓
report
```

The result may look like:

```text
WALLET W123

Risk Score: 91/100
Confidence: 0.93

Signals:
[HIGH] Transaction anomaly
[HIGH] Peeling-chain-like pattern
[MED]  Entity cluster relationship
[MED]  Rapid transaction behavior

Why flagged:
• transaction velocity is outside the learned baseline
• suspicious flow structure detected
• wallet belongs to an inferred related cluster
• elevated risk propagated through graph relationships
```

---

# 10. What Makes BCTX More Than a Wallet Checker?

A simple wallet checker might answer:

```text
Address:
Balance:
Transactions:
```

BCTX goes beyond that.

It creates a connected investigation:

```text
Wallet
  ↓
Transactions
  ↓
Other wallets
  ↓
Entity clusters
  ↓
Network observations
  ↓
Transaction flows
  ↓
ML signals
  ↓
Risk
  ↓
Evidence
```

The goal is to expose **relationships and behavior**, not merely display raw transaction history.

---

# 11. The Transaction Graph

The core analytical representation is a graph.

Conceptually:

```text
             W100
               │
               ▼
             TX901
            /     \
           ▼       ▼
         W123     W441
           │
           ▼
         TX999
           │
           ▼
         W821
```

The graph can represent:

```text
Wallet
Transaction
IP
Entity
```

with relationships such as:

```text
Wallet → Transaction
Transaction → Wallet
IP → Transaction
Wallet → Entity
```

This allows BCTX to investigate the structure around an entity rather than viewing every record independently.

---

# 12. Why the Graph Matters

Suspicious activity may be distributed across many transactions.

For example:

```text
A → B
B → C
C → D
D → E
```

The significance may come from the sequence rather than any one transfer.

The graph allows us to ask:

```text
What is connected?
How many hops away?
Where does money split?
Where does it merge?
Which wallets repeatedly interact?
Which clusters appear related?
```

---

# 13. AI/ML Layer

BCTX should contain a real AI/ML layer.

The first focus should be:

## Anomaly Detection

Find behavior that is statistically unusual relative to learned normal behavior.

Potential features include:

```text
transaction frequency
amount
time gap
transaction velocity
fan-in
fan-out
wallet degree
counterparty count
graph structure
transaction chain length
```

Output:

```text
Anomaly Score: 0.93
```

---

# 14. Entity Clustering

BCTX should identify wallets that appear related based on available evidence.

Potential signals:

```text
common-input relationships
transaction behavior
graph neighborhood similarity
timing similarity
graph embeddings
```

Example:

```text
Cluster #18

W123
W456
W789
W821

Confidence: 0.87
```

The cluster represents an **inferred relationship**, not proof of real-world ownership.

---

# 15. Suspicious Flow Detection

BCTX should detect flow structures that resemble patterns of interest.

Examples include:

```text
peeling-chain-like behavior
mixing-like structures
rapid transfer chains
high fan-out
high fan-in
unusual flow patterns
```

The output should be a pattern finding:

```text
Pattern:
Peeling-chain-like behavior

Confidence:
0.88
```

rather than a claim about legal guilt or intent.

---

# 16. Risk Scoring

BCTX combines multiple signals.

Conceptually:

```text
Anomaly
+
Pattern Detection
+
Entity Clustering
+
Graph Signals
+
Network Correlation
+
Risk Propagation
        ↓
     Risk Engine
        ↓
Risk Score + Confidence
```

Example:

```text
Risk Score: 91/100
Confidence: 0.93
```

Risk is an investigative prioritization signal.

It is not intended to be presented as proof of wrongdoing.

---

# 17. Risk Propagation

If an entity already has elevated risk, BCTX can propagate part of that risk through graph relationships.

Conceptually:

```text
Seed
Risk 1.00
  │
  ├── W2
  │
  └── W3
```

Risk can decay with graph distance according to the selected algorithm.

Possible future methods:

```text
distance-based propagation
label propagation
graph diffusion
personalized PageRank
```

The path and propagation contribution should be visible to the analyst.

---

# 18. Network Correlation

The problem context includes network-layer metadata such as:

```text
source IP
source port
destination IP
destination port
timestamp
```

and blockchain-layer data such as:

```text
TXID
wallet addresses
amounts
transaction relationships
```

BCTX correlates those datasets within the available evidence.

Conceptually:

```text
IP
 │
 ▼
Network Observation
 │
 ▼
Transaction
 │
 ▼
Wallet
 │
 ▼
Entity Cluster
```

The system analyzes the metadata it has actually acquired or imported. It does not assume that every wallet has a known or reliable IP association.

---

# 19. Evidence-First Design

An alert should never simply say:

```text
W123 = suspicious
```

Instead:

```text
W123
Risk: 91

Why?

1. High transaction velocity anomaly
2. Peeling-chain-like pattern
3. Cluster relationship
4. Graph proximity to high-risk seed
5. Correlated network observations
```

Each reason should link to evidence.

---

# 20. The `explain` Command

One of the signature features is:

```bash
bctx explain wallet W123
```

The result should explain:

```text
RISK SCORE
CONFIDENCE
CONTRIBUTING SIGNALS
SOURCE TRANSACTIONS
GRAPH RELATIONSHIPS
NETWORK OBSERVATIONS
MODEL INFORMATION
```

Example:

```text
WHY FLAGGED

[1] Transaction anomaly
    Score: 0.93

[2] Flow pattern
    Peeling-chain-like behavior detected

[3] Cluster
    Member of Cluster #18

[4] Graph
    Connected to elevated-risk seed

[5] Network correlation
    Relevant observations found
```

This is a central part of the product's explainability.

---

# 21. Monitoring

The second major workflow is:

```bash
bctx monitor wallet W123
```

While connected:

```text
new external event
       ↓
capture
       ↓
persist locally
       ↓
graph update
       ↓
incremental analysis
       ↓
risk update
       ↓
TUI update
```

Example:

```text
12:41:02  TX a81f...  +0.82 BTC
12:41:05  TX 92bc...  -0.41 BTC
12:41:11  TX 81ae...  -0.39 BTC

Previous Risk: 63
Current Risk: 71
Change: +8
```

---

# 22. Monitor + Offline Transition

Suppose:

```text
14:00 — connected
```

BCTX receives:

```text
TX1
TX2
TX3
TX4
```

and stores them.

Then:

```text
14:05 — network disconnected
```

The system cannot receive TX5 from the network.

But it can still:

```text
TX1
TX2
TX3
TX4
 ↓
local analysis
 ↓
new summary
 ↓
risk assessment
 ↓
report
```

Therefore monitoring has two states:

```text
live acquisition
+
offline analysis of captured events
```

---

# 23. Summarization

A future/advanced command could be:

```bash
bctx summarize monitor
```

It can summarize:

```text
events received
risk changes
new patterns
new relationships
important transactions
alerts
evidence
```

An optional local LLM can turn structured evidence into natural-language prose.

Architecture:

```text
Deterministic Analysis
 ↓
Structured Evidence
 ↓
Local LLM
 ↓
Human-readable Summary
```

The LLM is not the source of the actual risk score.

---

# 24. Local AI Architecture

BCTX should package its models locally.

Development:

```text
Python
 ↓
train
 ↓
evaluate
 ↓
export
```

Production:

```text
ONNX model
 ↓
ONNX Runtime
 ↓
Go
 ↓
local inference
```

Model files can be shipped with the installation:

```text
models/
├── anomaly.onnx
├── entity.onnx
└── flow.onnx
```

No runtime model download is required.

---

# 25. Data Acquisition

BCTX should treat data acquisition as a pluggable layer.

Possible sources:

```text
external Bitcoin explorer/blockchain data provider
synthetic dataset
CSV
JSON
XML
future provider
```

The acquisition layer should normalize all sources into a canonical BCTX schema.

Correct:

```text
Provider
 ↓
Adapter
 ↓
Normalize
 ↓
Persist locally
 ↓
Analyze
```

The analysis engine should not directly call providers.

---

# 26. Local Storage

Each case should own its data.

Example:

```text
~/.bctx/
└── cases/
    └── case-001/
        ├── case.db
        ├── evidence/
        ├── imports/
        ├── reports/
        ├── exports/
        └── logs/
```

The database stores:

```text
transactions
wallets
network observations
entities
graph edges
features
model outputs
risk assessments
evidence
alerts
reports
audit events
```

---

# 27. Search

BCTX should make local information quickly searchable.

Examples:

```bash
bctx wallet W123
bctx transaction TX123
bctx ip 10.1.2.3
bctx search W123
```

Indexes allow the system to find records without scanning the full dataset every time.

---

# 28. Investigation Commands

The command system should be organized around real investigative actions.

### Data

```bash
bctx dataset import
bctx dataset inspect
bctx dataset list
bctx sync
```

### Investigation

```bash
bctx wallet
bctx transaction
bctx ip
bctx analyze
```

### Graph

```bash
bctx graph
bctx neighbors
bctx path
bctx cluster
```

### Detection

```bash
bctx anomalies
bctx patterns
bctx alerts
```

### Explanation

```bash
bctx explain
```

### Monitoring

```bash
bctx monitor
```

### Reporting

```bash
bctx report
bctx export
```

---

# 29. Terminal UI

The product should have a rich terminal interface.

Conceptually:

```text
┌─────────────────────────────────────────────────────────────┐
│ BCTX                         OFFLINE                         │
├──────────────────┬──────────────────────────────────────────┤
│ ALERTS           │                GRAPH                     │
│                  │                                          │
│ W123     94      │             W100                         │
│ TX991    91      │               │                          │
│ W441     89      │              TX1                        │
│                  │             /   \                       │
│                  │           W123  W441                    │
├──────────────────┴──────────────────────────────────────────┤
│ EVIDENCE                                                    │
│ [HIGH] Transaction anomaly                                  │
│ [HIGH] Peeling-chain-like pattern                           │
│ [MED]  Cluster relationship                                 │
├─────────────────────────────────────────────────────────────┤
│ bctx> explain W123                                          │
└─────────────────────────────────────────────────────────────┘
```

The TUI is the investigator's cockpit.

---

# 30. Installation Experience

The intended end-user experience is:

```bash
curl -fsSL https://<official-domain>/install.sh | sh
```

Then:

```bash
bctx
```

The user should not need to manually install:

```text
Python
Node.js
PostgreSQL
Redis
Docker
```

for the core runtime.

For an air-gapped machine, a complete release bundle can instead be transferred by controlled local media.

---

# 31. "One Binary" Philosophy

The ideal production experience is:

```text
bctx binary
+
local model bundle
+
optional local GeoIP data
+
user datasets
```

The product should feel like a native Linux investigation tool, not a collection of development services.

---

# 32. Technology Direction

Recommended implementation:

```text
Core Runtime
    Go

CLI
    Cobra

TUI
    Bubble Tea
    Bubbles
    Lip Gloss

Database
    SQLite

Analytics
    DuckDB when needed

Graph
    Custom Go graph subsystem

ML Training
    Python
    scikit-learn / PyTorch

ML Deployment
    ONNX
    ONNX Runtime

Configuration
    TOML

Reports
    Markdown
    JSON
    HTML
    PDF

GeoIP
    Local MMDB database
```

---

# 33. Why the Product Is Terminal-First

A terminal-first experience gives BCTX a different identity from a typical browser dashboard.

The analyst works through actions:

```text
analyze
trace
graph
explain
monitor
report
```

rather than navigating a large collection of web pages.

The system can still render rich visual information using a TUI.

The terminal also naturally supports:

```text
SSH/local Linux environments
automation
scripts
reproducibility
offline systems
restricted environments
```

---

# 34. Product Personality

BCTX should feel:

```text
technical
focused
investigative
fast
local
evidence-driven
```

It should avoid looking like:

```text
generic banking dashboard
generic crypto wallet
generic SaaS analytics panel
```

The experience should communicate:

> **This is an intelligence workstation.**

---

# 35. What the User Actually Gets

The user gives BCTX:

```text
wallet
transaction
entity
dataset
```

BCTX gives back:

```text
historical context
transaction relationships
graph
anomalies
clusters
patterns
risk
confidence
evidence
explanation
timeline
monitoring state
report
```

---

# 36. End-to-End Idea

The product can be summarized as:

```text
                USER
                  │
                  ▼
           "Investigate W123"
                  │
                  ▼
            BCTX COMMAND
                  │
                  ▼
       Is required data local?
             /           \
           YES            NO
            │              │
            │        acquire while
            │          connected
            │              │
            │              ▼
            │         store locally
            │              │
            └──────┬───────┘
                   ▼
             Build / update
                 graph
                   │
                   ▼
             Extract features
                   │
                   ▼
              Local ML
                   │
                   ▼
          Pattern / Cluster
              analysis
                   │
                   ▼
             Risk Engine
                   │
                   ▼
           Evidence Engine
                   │
             ┌─────┴─────┐
             ▼           ▼
           Explain      Alert
             │           │
             └─────┬─────┘
                   ▼
                 Report
```

---

# 37. The Signature Experience

The strongest BCTX experience is:

```bash
$ bctx
```

```text
BCTX > analyze wallet W123
```

```text
[1/7] Checking local evidence...
[2/7] Expanding transaction graph...
[3/7] Extracting behavioral features...
[4/7] Running anomaly model...
[5/7] Detecting flow patterns...
[6/7] Calculating risk...
[7/7] Building evidence...
```

Then:

```text
INVESTIGATION COMPLETE

Wallet: W123

Risk Score: 91
Confidence: 0.93

3 major signals detected
18 related wallets
231 relevant transactions
6-hop suspicious flow

Type:
    explain
    graph
    timeline
    report
```

The user can then turn off the network and continue:

```bash
bctx> explain W123
bctx> graph W123
bctx> report W123
```

---

# 38. What Makes the Idea Interesting

The product combines several capabilities into one local workflow:

```text
Bitcoin data
      +
Network metadata
      +
Graph analysis
      +
AI/ML
      +
Risk scoring
      +
Explainability
      +
Terminal UX
      +
Offline execution
```

The result is not merely another analytics dashboard.

It is a **portable investigation environment**.

---

# 39. SIH Alignment

The product is designed around the main capabilities described by the challenge:

```text
bulk metadata ingestion
        ↓
IP / wallet / TX correlation
        ↓
entity / transaction graph
        ↓
AI / ML detection
        ↓
entity clustering
        ↓
anomaly detection
        ↓
suspicious flow analysis
        ↓
risk scoring
        ↓
ranked explainable alerts
        ↓
visualization / evidence
        ↓
offline Linux deployment
```

This is the foundation of the BCTX idea.

---

# 40. MVP

The first working version should not attempt every advanced capability.

The MVP should deliver:

```text
✓ Linux binary
✓ bctx CLI
✓ basic TUI
✓ case management
✓ CSV import
✓ local SQLite storage
✓ wallet lookup
✓ transaction lookup
✓ IP lookup against local data
✓ transaction graph
✓ basic feature extraction
✓ anomaly model
✓ local ONNX inference
✓ entity clustering
✓ one suspicious-flow detector
✓ risk scoring
✓ evidence
✓ alerts
✓ explain command
✓ report generation
✓ offline analysis
```

---

# 41. Future Capabilities

After the MVP:

```text
WebSocket/live monitoring
incremental graph updates
risk deltas
local GeoIP
DuckDB analytics
advanced graph embeddings
GNN-based models
local LLM
natural-language investigation
agentic workflow orchestration
USB/offline model bundles
advanced report versioning
```

---

# 42. Final Product Definition

> **BCTX is a terminal-first, offline-first Bitcoin forensic intelligence platform that acquires relevant Bitcoin and network metadata when connected, stores it locally, constructs a graph of wallets, transactions and network observations, applies local AI/ML and graph analysis to detect anomalies and suspicious patterns, calculates explainable risk, and gives investigators a searchable terminal environment for investigation, monitoring, summarization, and report generation.**

---

# 43. The One-Sentence Idea

> **BCTX is "Claude Code for Bitcoin investigations" — a native terminal intelligence workstation that turns Bitcoin transaction/network data into offline, graph-based, AI-assisted, evidence-backed investigations.**

---

# 44. Final Mental Model

Remember BCTX as:

```text
             RAW BITCOIN DATA
                    │
                    ▼
              BCTX INGEST
                    │
                    ▼
             LOCAL EVIDENCE
                    │
                    ▼
                 GRAPH
                    │
                    ▼
              LOCAL AI/ML
                    │
                    ▼
                  RISK
                    │
                    ▼
                EVIDENCE
                    │
           ┌────────┴────────┐
           ▼                 ▼
        INVESTIGATE        REPORT
           │
           ▼
        TERMINAL
```

And the defining promise is:

```text
Acquire while connected.
Own the data locally.
Analyze locally.
Disconnect the network.
Keep investigating.
```
