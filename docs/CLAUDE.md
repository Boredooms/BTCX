# CLAUDE.md — BCTX Engineering Instructions

## Mission

You are implementing **BCTX**, a Linux-native, terminal-first Bitcoin forensic intelligence platform.

BCTX must behave like a native investigation tool:

```bash
bctx
```

The primary product is:

```text
CLI + Interactive TUI
+
Local evidence store
+
Graph engine
+
Local ML
+
Risk engine
+
Evidence/explainability
+
Monitoring
+
Reports
```

The system is **offline-first**.

---

# 1. Absolute Architecture Rules

## Rule 1 — Offline means genuinely offline

Once required data is local, all of these must work with networking disabled:

```text
search
wallet lookup
transaction lookup
IP lookup against local data
graph traversal
feature extraction
ML inference
pattern detection
clustering
risk scoring
risk propagation
alerts
explanations
summaries
reports
PDF/HTML/JSON export
TUI
```

Never add a hidden network call to any of these paths.

---

## Rule 2 — Acquisition is isolated

Only:

```text
internal/acquisition/
```

and explicit provider implementations may use network access.

Analysis code must not import an HTTP client.

Correct:

```text
provider
 ↓
adapter
 ↓
local repository
 ↓
analysis
```

Incorrect:

```text
risk engine
 ↓
HTTP API
```

---

## Rule 3 — Do not invent network evidence

A blockchain explorer can provide blockchain data.

It does not automatically prove a wallet-to-IP relationship.

BCTX must distinguish:

```text
blockchain evidence
network telemetry evidence
local imported evidence
```

Only establish an IP/wallet relationship when the available dataset/provider actually supplies evidence for it.

---

## Rule 4 — Local data is the source for offline analysis

After acquisition:

```text
remote response
 ↓
normalize
 ↓
persist
 ↓
index
 ↓
analyze locally
```

Never directly send a provider response into the ML/risk layer.

---

## Rule 5 — ML is real, local, and versioned

Production inference uses:

```text
ONNX
+
ONNX Runtime
+
local model files
```

No runtime model download.

Every model has:

```text
model version
feature schema
input schema
checksum
manifest
```

---

## Rule 6 — Risk is not a black box

A risk score must always be decomposable into:

```text
signal
+
evidence
+
confidence
```

Never output a score with no explanation path.

---

## Rule 7 — LLM is never the source of truth

A future local LLM can:

```text
summarize evidence
write report prose
interpret structured investigation results
```

It must not:

```text
invent transactions
invent IP associations
invent model outputs
invent evidence
become the primary risk calculator
```

---

# 2. Technology Rules

## Core

Use:

```text
Go
```

## CLI

Use:

```text
Cobra
```

## TUI

Use:

```text
Bubble Tea
Bubbles
Lip Gloss
```

## Storage

Default:

```text
SQLite
```

Optional analytical layer:

```text
DuckDB
```

## ML development

Use:

```text
Python
NumPy
pandas
scikit-learn
PyTorch
```

## ML deployment

Use:

```text
ONNX
ONNX Runtime
```

Do not add technologies just because they sound impressive.

---

# 3. Backend-First Rule

The development order is mandatory:

```text
storage
 ↓
ingestion
 ↓
indexing
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
alerts
 ↓
reports
 ↓
CLI
 ↓
TUI
```

Do not start by polishing the TUI while backend behavior is incomplete.

---

# 4. Repository Boundaries

Suggested ownership:

```text
internal/storage
    Persistence only

internal/acquisition
    External data only

internal/ingestion
    File parsing and import

internal/normalization
    Canonical schema conversion

internal/indexing
    Query acceleration

internal/graph
    Graph algorithms/data

internal/features
    Feature calculation

internal/ml
    Local model execution

internal/risk
    Risk calculations

internal/evidence
    Evidence aggregation

internal/alerts
    Alert lifecycle/ranking

internal/monitoring
    Long-running event ingestion

internal/reports
    Output generation

internal/cli
    Command presentation

internal/tui
    Terminal presentation
```

Business logic should not live inside CLI/TUI rendering files.

---

# 5. Service-Oriented Internal Design

Use stable interfaces between layers.

Conceptually:

```go
type Repository interface {}
type DataSource interface {}
type GraphService interface {}
type FeatureService interface {}
type MLService interface {}
type RiskService interface {}
type EvidenceService interface {}
type ReportService interface {}
```

The exact signatures can evolve, but the separation must remain.

---

# 6. Investigation Workflow

For:

```bash
bctx analyze wallet W123
```

follow:

```text
1. Check local evidence.
2. If incomplete and acquisition is permitted:
      acquire data.
3. Persist locally.
4. Normalize.
5. Index.
6. Update graph.
7. Extract features.
8. Run local ML.
9. Run pattern detectors.
10. Run clustering where applicable.
11. Run risk.
12. Run propagation.
13. Build evidence.
14. Create/update alert.
15. Return InvestigationResult.
16. Render CLI/TUI.
```

The presentation layer must never independently reconstruct this pipeline.

---

# 7. Offline Workflow

For:

```bash
bctx analyze wallet W123 --offline
```

the call path must be:

```text
CLI
 ↓
InvestigationService
 ↓
Local Repository
 ↓
Graph
 ↓
Features
 ↓
Local ML
 ↓
Risk
 ↓
Evidence
 ↓
Result
```

No acquisition call.

---

# 8. Data Modeling Rules

Use explicit types for:

```text
Wallet
Transaction
TransactionInput
TransactionOutput
NetworkObservation
Entity
GraphEdge
FeatureVector
Prediction
RiskAssessment
EvidenceItem
Alert
Report
```

Avoid generic:

```text
map[string]interface{}
```

for core domain objects.

Use typed structures and validation.

---

# 9. Error Handling

Never silently recover from important correctness failures.

Examples:

```text
missing model
invalid model checksum
schema incompatibility
database corruption
invalid transaction
missing required evidence
```

Return structured errors.

Do not generate a fake result.

---

# 10. Data Ingestion Rules

Large input must be processed using:

```text
streaming
batching
transactions
checkpoints
```

Do not do:

```go
os.ReadFile(hugeDataset)
```

for large production imports.

---

# 11. Database Rules

All schema changes use migrations.

Never manually mutate production tables outside migrations.

Indexes are added based on actual query patterns.

All imports are transactional in batches.

---

# 12. Graph Rules

Graph traversal must be bounded.

Never recursively expand an unbounded graph.

Default:

```text
depth = small bounded value
```

Require explicit expansion for deeper searches.

Large graphs must be reduced to investigation subgraphs before rendering.

---

# 13. ML Rules

Models must be loaded once and reused where possible.

Every prediction should include:

```text
model version
timestamp
feature schema
subject
score
```

Don't normalize scores inconsistently across commands.

Keep the score contract stable.

---

# 14. Risk Rules

Risk engine must be deterministic given:

```text
same dataset
same model version
same configuration
```

within defined numerical tolerance.

Risk calculation must preserve contribution information.

---

# 15. Evidence Rules

Every risk signal should be traceable to:

```text
source record(s)
derived feature(s)
model output or deterministic detector
```

The evidence layer should not produce unsupported statements.

---

# 16. Monitoring Rules

Monitor is event-driven.

Correct:

```text
event
 ↓
persist
 ↓
increment graph
 ↓
increment features
 ↓
local inference
 ↓
risk update
 ↓
TUI event
```

When network disappears:

```text
stop acquisition
keep local analysis alive
```

When network returns:

```text
resume from last local state
deduplicate
catch up
reanalyze
```

---

# 17. Report Rules

Reports must be generated from structured `InvestigationReport`.

Do not assemble reports by scraping terminal text.

Correct:

```text
InvestigationResult
 ↓
InvestigationReport
 ↓
PDF/HTML/JSON/Markdown
```

---

# 18. CLI Rules

CLI commands should be scriptable.

Where useful, support:

```bash
--json
```

Do not mix machine-readable JSON with decorative terminal styling.

---

# 19. TUI Rules

The TUI is a presentation layer over the same services used by CLI.

Do not duplicate business logic.

TUI must:

```text
remain responsive
support cancellation
show progress
show current network state
show current case
show job state
```

---

# 20. TUI Design Direction

Use a serious terminal investigation aesthetic.

Prioritize:

```text
clear hierarchy
dense useful information
keyboard navigation
graph visualization
evidence traceability
status visibility
```

Avoid:

```text
generic dashboard cards
browser-like layouts
excessive decoration
meaningless animations
```

---

# 21. Installation Rules

Production must be distributable from GitHub Releases.

Target:

```bash
curl -fsSL https://raw.githubusercontent.com/<ORG>/bctx/main/scripts/install.sh | sh
```

Installer must:

```text
detect OS/architecture
resolve release
download binary
download model bundle
verify
install
initialize
run doctor
```

Also support an explicit local installer for air-gapped systems.

---

# 22. Release Rules

A release should include:

```text
Linux amd64 binary/archive
Linux arm64 binary/archive
model bundle
checksums
release manifest
demo dataset where appropriate
```

The `bctx` executable is the core user-facing artifact.

---

# 23. No Unnecessary Infrastructure

Do not introduce:

```text
PostgreSQL
Redis
Kafka
Kubernetes
microservices
cloud queues
cloud ML
```

unless an explicit later requirement demands them.

The initial system should remain:

```text
native binary
+
embedded/local storage
+
local models
+
local files
```

---

# 24. Testing Rules

Every feature should be tested at three levels where applicable:

```text
unit
integration
end-to-end
```

Every offline feature must have an offline test.

Important tests:

```text
no-network analysis
model integrity
case isolation
large dataset import
graph traversal
risk reproducibility
report generation
monitor disconnect/reconnect
```

---

# 25. Definition of Done for Every Task

A task is not done because:

```text
code compiles
```

It is done when:

```text
implementation
+
tests
+
error handling
+
CLI verification
+
offline verification where applicable
+
documentation
```

are complete.

---

# 26. Agent Operating Procedure

Before changing code:

1. Read `TASKS.md`.
2. Identify the active phase and dependencies.
3. Inspect existing interfaces.
4. Avoid duplicating already-implemented functionality.
5. Implement the smallest correct unit.
6. Add tests.
7. Run the relevant test set.
8. Run broader tests before marking the task complete.
9. Update task status.

Do not skip prerequisite phases.

---

# 27. Worktree / Git Discipline

Keep commits focused.

Preferred commit pattern:

```text
feat(storage): add case schema
feat(ingestion): add streaming CSV importer
feat(graph): add bounded wallet traversal
feat(ml): integrate anomaly ONNX model
feat(tui): add wallet investigation view
```

Avoid giant commits containing unrelated systems.

---

# 28. Required Status Reporting

When completing a task, report:

```text
Task:
Status:
Files changed:
Tests:
Commands verified:
Known limitation:
Next task:
```

This makes agent work auditable.

---

# 29. Stop Conditions

Stop and ask for clarification only when:

```text
requirements conflict
required secret/provider credential is missing
destructive database migration is ambiguous
model input contract is unknown
```

Otherwise, infer implementation details from the existing architecture and proceed conservatively.

Never silently change the product contract.

---

# 30. Final Engineering Mantra

```text
Backend first.
Evidence first.
Offline first.
Local by default.
Network only for acquisition.
ML must be real.
Risk must be explainable.
TUI must consume the core.
Distribution must be native.
Tests must prove the claims.
```
