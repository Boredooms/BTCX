# AGENTS.md — BCTX Agent Operating System

## 1. Purpose

This file defines how AI coding agents must work on BCTX.

BCTX is a complex, backend-heavy, offline-first Linux terminal application. Agents must behave like coordinated engineering contributors, not independent code generators.

The primary rule is:

> **Do not build the interface around imaginary backend functionality. Build and verify the engine first, then make the terminal UX expose the verified engine.**

---

# 2. Global Priorities

Every agent must prioritize in this order:

```text
1. Correctness
2. Offline guarantees
3. Data integrity
4. Evidence traceability
5. Testability
6. Performance
7. CLI usability
8. TUI quality
9. Visual polish
10. Advanced convenience features
```

Never reverse this order.

---

# 3. Master System Boundary

All agents must preserve:

```text
                 NETWORK
                    │
                    ▼
             ACQUISITION ONLY
                    │
                    ▼
             LOCAL EVIDENCE
                    │
        ─── OFFLINE BOUNDARY ───
                    │
          ┌─────────┼─────────┐
          ▼         ▼         ▼
        Graph      ML        Search
          │         │         │
          └─────────┼─────────┘
                    ▼
                  Risk
                    │
                 Evidence
                    │
              CLI / TUI
                    │
                 Reports
```

No agent may weaken this boundary.

---

# 4. Agent Roles

Agents should operate under explicit roles.

## Agent 01 — Architect

Owns:

```text
system contracts
package boundaries
interfaces
data-flow correctness
architecture decisions
```

Does not own visual polish.

---

## Agent 02 — Core Runtime

Owns:

```text
Go entrypoint
configuration
filesystem layout
runtime initialization
version
doctor
```

Files:

```text
cmd/
internal/config/
```

---

## Agent 03 — Storage

Owns:

```text
SQLite
migrations
repositories
transactions
case persistence
```

Files:

```text
internal/storage/
internal/cases/
migrations/
```

---

## Agent 04 — Ingestion

Owns:

```text
CSV
JSON
XML
normalization
validation
streaming
checkpoints
```

Files:

```text
internal/ingestion/
internal/normalization/
```

---

## Agent 05 — Acquisition

Owns:

```text
blockchain provider adapters
network metadata adapters
sync
pagination
monitor transport
```

Files:

```text
internal/acquisition/
```

Strict rule:

> Acquisition code may use the network. Analysis code must not.

---

## Agent 06 — Index/Search

Owns:

```text
wallet index
TX index
IP index
timestamp index
query optimization
```

Files:

```text
internal/indexing/
```

---

## Agent 07 — Graph

Owns:

```text
graph model
edges
N-hop traversal
path
neighbors
graph algorithms
```

Files:

```text
internal/graph/
```

---

## Agent 08 — Feature Engineering

Owns:

```text
wallet features
transaction features
temporal features
graph features
flow features
network features
```

Files:

```text
internal/features/
```

---

## Agent 09 — ML Research

Owns:

```text
synthetic data generation
training
evaluation
model selection
ONNX export
```

Files:

```text
ml/
models/development/
```

Does not modify TUI behavior.

---

## Agent 10 — ML Runtime

Owns:

```text
ONNX Runtime
model loader
manifest validation
prediction contracts
model status
```

Files:

```text
internal/ml/
models/
```

---

## Agent 11 — Detection

Owns:

```text
entity clustering
peeling-chain-like detection
mixing-like detection
fan-in/fan-out
rapid-flow
```

Files:

```text
internal/patterns/
internal/entity/
```

If these directories do not yet exist, create them consistently with the architecture.

---

## Agent 12 — Risk

Owns:

```text
risk calculation
risk aggregation
risk propagation
risk delta
```

Files:

```text
internal/risk/
```

---

## Agent 13 — Evidence/Alerts

Owns:

```text
evidence
explanations
alert lifecycle
ranking
```

Files:

```text
internal/evidence/
internal/alerts/
```

---

## Agent 14 — Monitoring

Owns:

```text
monitor sessions
event streams
incremental analysis
disconnect/reconnect
```

Files:

```text
internal/monitoring/
```

---

## Agent 15 — Reports

Owns:

```text
report model
Markdown
JSON
HTML
PDF
versioning
```

Files:

```text
internal/reports/
```

---

## Agent 16 — CLI

Owns:

```text
Cobra commands
flags
command output
JSON mode
shell UX
```

Files:

```text
internal/cli/
```

CLI must call core services, not implement business logic.

---

## Agent 17 — TUI

Owns:

```text
Bubble Tea
navigation
panels
graph renderer
timeline
alerts
evidence
monitoring screen
```

Files:

```text
internal/tui/
```

TUI must call the same services as CLI.

---

## Agent 18 — Security/Offline

Owns:

```text
offline enforcement
model verification
safe paths
network audit
case isolation
```

Files:

```text
internal/security/
tests/offline/
```

---

## Agent 19 — Release/DevOps

Owns:

```text
GitHub Actions
release packaging
install.sh
local installer
checksums
manifests
Linux builds
```

Files:

```text
.github/
scripts/
```

---

## Agent 20 — QA

Owns:

```text
unit tests
integration tests
E2E tests
offline tests
performance tests
release smoke tests
```

Agents should not mark their own work complete without QA-compatible verification.

---

# 5. Agent Collaboration Rule

The following dependency chain is authoritative:

```text
Core
 ↓
Storage
 ↓
Ingestion
 ↓
Indexing
 ↓
Graph
 ↓
Features
 ↓
ML
 ↓
Detection
 ↓
Risk
 ↓
Evidence
 ↓
Alerts
 ↓
Reports
 ↓
CLI
 ↓
TUI
 ↓
Monitoring UX
 ↓
Distribution polish
```

Agents may work in parallel only when their interfaces are already defined.

---

# 6. Backend-First Gate

No TUI agent should begin final implementation until these are functioning:

```text
SQLite
Import
Wallet lookup
TX lookup
Graph
Features
ML
Detection
Risk
Evidence
Alerts
Explain
Report object
Offline inference
```

Prototype layouts may exist before this gate, but production UX work waits for the backend gate.

---

# 7. Shared Contracts

Agents must use shared domain contracts.

Core types include:

```text
Wallet
Transaction
NetworkObservation
Entity
GraphEdge
FeatureVector
Prediction
PatternResult
RiskAssessment
EvidenceItem
Alert
InvestigationResult
InvestigationReport
MonitorEvent
```

Do not invent duplicate versions of these structs in different packages.

---

# 8. Database Contract

Storage agent owns persistence.

Other agents should access data through repository/service interfaces rather than executing arbitrary SQL everywhere.

Exception:

```text
performance investigation
migration
analytical query
```

must be explicitly justified.

---

# 9. Network Contract

Acquisition agent owns network access.

Any agent adding a network call must answer:

```text
Why is this acquisition?
Why can't the data come from local storage?
Why is the call required?
What happens offline?
```

If the answer is unclear, do not add the call.

---

# 10. ML Contract

ML agents must publish:

```text
model
version
manifest
feature schema
input schema
output schema
evaluation metrics
```

No model becomes production-active without validation.

---

# 11. Risk Contract

Risk agent must expose:

```text
score
confidence
signal breakdown
```

Evidence agent must be able to reconstruct:

```text
why score changed
```

from stored data.

---

# 12. TUI Contract

TUI receives structured state/events.

It must not directly:

```text
query external providers
run SQL everywhere
calculate risk
train models
```

It only requests services and renders results.

---

# 13. Agent Task Workflow

For every assigned task:

```text
1. Read TASKS.md.
2. Identify dependencies.
3. Inspect relevant package.
4. Inspect existing tests.
5. Implement.
6. Add/modify tests.
7. Run focused tests.
8. Run package tests.
9. Run end-to-end tests if required.
10. Update TASKS.md status.
11. Report changes.
```

---

# 14. No Scope Creep

Do not add:

```text
new frameworks
new databases
new cloud dependencies
new UI systems
```

just because they are convenient.

Prefer the existing architecture.

---

# 15. No Fake Implementations

Agents must not create placeholder behavior such as:

```text
risk = random()
```

or:

```text
"ML model" implemented as fixed threshold only
```

when the task explicitly requires a working ML component.

Temporary mocks are allowed only inside tests and must be clearly marked.

---

# 16. No Fake Live Data

Never simulate network data and label it:

```text
LIVE
```

outside an explicitly named demo/mock mode.

Use:

```text
DEMO
SIMULATED
FIXTURE
```

when appropriate.

---

# 17. Offline Truthfulness

When disconnected:

```text
new network acquisition = impossible
```

The system should show:

```text
NETWORK: DISCONNECTED
ACQUISITION: PAUSED
LOCAL ANALYSIS: AVAILABLE
```

Do not hide this distinction.

---

# 18. Evidence Truthfulness

Never convert:

```text
pattern detected
```

into:

```text
criminal activity confirmed
```

Never turn:

```text
cluster relationship
```

into:

```text
same owner proven
```

Use evidence-backed language.

---

# 19. Error Handling

Every agent must implement failure behavior.

At minimum:

```text
invalid input
missing local data
offline with missing data
provider timeout
database failure
model failure
corrupt model
corrupt dataset
export failure
```

Never silently swallow failures.

---

# 20. Performance Discipline

Before optimizing:

```text
measure
```

Do not prematurely introduce:

```text
Redis
Kafka
distributed systems
```

Use:

```text
streaming
indexes
batching
lazy graph expansion
incremental updates
worker goroutines
```

first.

---

# 21. Security Discipline

Do not log:

```text
credentials
API keys
sensitive raw evidence
```

unless explicitly required.

Imported files must not be able to escape their case directory through unsafe paths.

Model files must be verified before activation.

---

# 22. Git Discipline

Agents should use focused commits.

Examples:

```text
feat(db): add transaction schema
feat(ingest): add streaming csv importer
feat(graph): add bounded traversal
feat(ml): add anomaly onnx runtime
feat(tui): add alert panel
test(offline): verify report generation without network
```

Avoid mixing:

```text
database rewrite
+
TUI redesign
+
installer
```

in one commit.

---

# 23. Progress Protocol

After a meaningful task, provide:

```text
ROLE
TASK
STATUS
CHANGED FILES
TESTS RUN
RESULT
KNOWN LIMITATIONS
NEXT DEPENDENCY
```

Example:

```text
ROLE: Storage Agent
TASK: SQLite case schema
STATUS: COMPLETE

Changed:
- internal/storage/schema.go
- migrations/001_init.sql
- tests/storage_test.go

Tests:
go test ./internal/storage/... → PASS

Known limitation:
DuckDB analytics not implemented.

Next dependency:
Ingestion Agent.
```

---

# 24. Completion Gates

## Gate 1 — Runtime

```text
bctx version
bctx init
bctx doctor
```

## Gate 2 — Data

```text
dataset import
wallet lookup
TX lookup
IP lookup
```

## Gate 3 — Intelligence

```text
graph
features
ML
patterns
clustering
risk
evidence
alerts
```

## Gate 4 — Investigation

```text
analyze
explain
report
```

## Gate 5 — TUI

```text
navigation
search
graph
alerts
evidence
monitoring
reports
```

## Gate 6 — Offline

```text
network off
analysis works
ML works
reports work
```

## Gate 7 — Distribution

```text
GitHub release
curl installer
doctor
offline demo
```

---

# 25. Final Release Checklist

Before release, agents must verify:

```text
[ ] Linux amd64 build
[ ] Linux arm64 build
[ ] GitHub Release artifacts
[ ] Checksums
[ ] Installer
[ ] Local installer
[ ] Model bundle
[ ] Demo dataset
[ ] bctx doctor
[ ] Offline analysis
[ ] Report export
[ ] Monitoring
[ ] Case isolation
[ ] No hidden analysis-time network calls
[ ] Documentation updated
```

---

# 26. Final Agent Mental Model

BCTX is not:

```text
a pretty dashboard with some analysis
```

It is:

```text
a serious local intelligence engine
          ↓
wrapped in a terminal
          ↓
with controlled connected acquisition
          ↓
and fully local/offline investigation
```

Every agent must preserve that mental model.

---

# 27. Ultimate Rule

> **Never build something that only looks like it works. Build the data path, analysis path, evidence path, offline path, tests, and user-facing path so the complete scenario actually works end to end.**
