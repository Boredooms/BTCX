# BCTX Phase 8 — Investigator TUI Design

Status: REVISED — review iteration 4 (iteration 2 addressed design-review.json#1: 2 HIGH + 4 MEDIUM + 3 NIT; iteration 3 addressed design-review.json#2: 3 MEDIUM + 2 NIT; this iteration 4 addresses design-review.json#3: 0 HIGH + 1 MEDIUM + 3 NIT; see §21, §22, §23 Review Responses) — pending re-review
Repository: `/home/devara/btcx` · Go module: `github.com/bctx/bctx` · Go 1.23.4 · `CGO_ENABLED=1`
Grounded in: `.phase8-artifacts/audit.md` (service inventory + baseline GREEN) and the live code read during authoring (`cli/commands/{home,engine,sync}.go`, `tui/screens/home.go`, `sdk/interfaces.go`, `graph/service.go`, `investigation/orchestrator/orchestrator.go`, `reporting/service.go`, `reporting/models/timeline.go`, `network/state/state.go`, `configs/config.go`).

This document is self-contained. A planner and coder can build Phase 8 from it without re-deciding architecture. Where it names a service method, that method already exists per the audit — the TUI calls it and never reimplements the logic behind it.

---

## 0. Scope, Non-Goals, and the One Architectural Rule

Phase 8 turns the existing BCTX backend (phases 0–7, baseline GREEN) into a single professional, keyboard-first investigator terminal application. It is **presentation + interaction only**.

The dependency direction is locked:

```
TUI (tui/*)
  ↓  calls
application / bootstrap seam (app/*)   ← shared with CLI
  ↓  calls
existing BCTX services (sdk, graph, orchestrator, reporting, monitoring, acquisition, risk, ml/inference, cases)
  ↓
repository / domain (storage/sqlite, pkg/schema)
```

The TUI MUST NOT contain any of: SQL, HTTP/TLS/socket dialing, provider parsing, normalization, feature calculation, ML algorithms, risk formulas, graph-building/traversal algorithms, report-generation business logic, GeoIP-download logic. All of these live behind the service seams below. The offline-boundary guard tests (`tests/offline/boundary_test.go`) must keep passing: the `tui/`, `tui/geoip`-consuming code, and map packages import no `net/http`/`crypto/tls` and dial nothing.

Non-goals: no browser/React/Electron/web server; no LLM/Ollama; no new backend logic; no rebuilding of any phase 0–7 system; no change to the canonical schema or storage.

### Technology stack (LOCKED on approval)

| Concern | Choice | Version | Notes |
|---|---|---|---|
| TUI runtime | `github.com/charmbracelet/bubbletea` | v1.3.4 (in go.mod) | `tea.WithContext(ctx)` for cancellation |
| Styling | `github.com/charmbracelet/lipgloss` | v1.1.0 (in go.mod) | tokens only; no inline colors |
| Widgets | `github.com/charmbracelet/bubbles` | **new dep — pin v0.20.0 (gated)** | `list`, `table`, `viewport`, `textinput`, `spinner`, `key`, `help`, `paginator`; v0.20.0 is the release compatible with bubbletea v1.3.x / lipgloss v1.x — gate per §0.1 |
| TTY detection | `github.com/mattn/go-isatty` | v0.0.20 (in go.mod) | keep TTY-vs-plain split |
| CLI | `github.com/spf13/cobra` | v1.8.1 (in go.mod) | root `RunE` still launches the TUI |
| MMDB reader (GeoIP) | `github.com/oschwald/maxminddb-golang` | **new dep — pin v1.13.1 (gated)** | expected pure-Go, expected ISC license — unverified until §0.1 gate passes; reads DB-IP `.mmdb`; properties MUST be verified per §0.1 before locking; see §15 and §20 |

Every new dep must be pure-Go, offline, permissively licensed, and pinned to an exact version. Adding `bubbles` and `maxminddb-golang` is the complete expected dependency delta for Phase 8. No other new UI or network dependency is permitted.

### 0.1 New-dependency verification gate (must pass before deps are locked)

Neither `bubbles` nor `maxminddb-golang` is in `go.mod` today (verified: only bubbletea/lipgloss/isatty/sqlite/cobra/toml are declared), so their pure-Go / permissive-license / no-network properties cannot be verified from the repo yet and are **not assumed**. Before either dependency is committed, the implementer MUST run and pass these checks (and the planner MUST encode them as a precondition):

1. `go get github.com/charmbracelet/bubbles@v0.20.0` and `go get github.com/oschwald/maxminddb-golang@v1.13.1` resolve against the locked bubbletea v1.3.4 / lipgloss v1.1.0 without forcing a major bump of any existing module. If the exact tag does not resolve cleanly, bump to the nearest compatible patch within the same minor (never a major) and record the resolved tag here.
2. For `maxminddb-golang`: `go list -deps github.com/oschwald/maxminddb-golang` shows **no** `net/*` and no `crypto/tls` transitive import; the package builds with `CGO_ENABLED=0` (pure-Go, no cgo); and the vendored/module license file is ISC (or another permissive MIT/BSD/Apache/CC0/public-domain license). If any check fails, fall back to the minimal in-repo MMDB reader (§15) instead.
3. The offline-boundary guard test is extended with a **dedicated, narrowly-scoped** check (§2, §17, §19.2) — not by adding `tui` to the existing `analysisDirs` rule, which would wrongly flag the legitimate `tui → acquisition`/`tui → monitoring` edges (see §17 for why). The new check asserts the no-network property for the `tui/*` roots after the dep lands.

Approval of these two deps is conditional on checks 1–3 passing. Until they pass, the versions above are the proposed pins, not confirmed facts.

---

## 1. Package Layout (new/changed)

```
app/                         # NEW — shared bootstrap seam (CLI + TUI use one wiring path)
  bootstrap.go               #   App{Engine, Cases, Repo, CaseID, Cleanup}; exported Build(cfg)
                             #   mirroring buildEngine's (*Engine,*manager.Manager,func(),error)
  services.go                #   constructors for graph.Service, orchestrator, reporting.Service,
                             #   ml/inference.Registry, monitoring wiring — reused by both layers
  gates.go                   #   AcquisitionAllowed(cfg) offline gate (see §1.1 contract)
  models.go                  #   ModelsDir(cfg) resolution (cfg.Models.Directory → dev fallback)

tui/
  app.go                     # Root model (top-level state machine, §4). Owns global state only.
  router.go                  # Screen registry + routing (§2). Maps ScreenID -> factory.
  keys.go                    # Global + per-mode keymaps (§5), built on bubbles/key.
  messages.go                # tea.Msg types (data-loaded, data-error, nav, notify, size, tick).
  commands.go                # tea.Cmd factories: every service call wrapped as a cancellable Cmd (§11).
  layout.go                  # Breakpoint detection + panel reflow math (§6).
  theme/
    tokens.go                # Raw palette + spacing/border tokens (§7). No semantics.
    theme.go                 # Semantic theme (role -> token) + Theme struct + degraded profile.
    styles.go                # lipgloss.Style builders consuming the active Theme.
  components/                # Reusable, self-contained components (§3). Each: Model/Init/Update/View.
    appshell.go topbar.go sidenav.go statusbar.go contextbar.go
    table.go detailpanel.go splitview.go modal.go
    commandpalette.go searchbox.go notification.go helpoverlay.go
    graphview.go mapview.go timelineview.go alertlist.go monitorstream.go
  screens/                   # One file per screen; each builds from components, holds no logic.
    home.go (REWRITTEN) search.go wallet.go transaction.go entity.go
    graph.go network.go geomap.go detection.go alerts.go
    monitoring.go data.go reports.go settings.go help.go timeline.go
  geoip/                     # NEW — NETWORK-FORBIDDEN. MMDB query + local registry (§15).
    geoip.go registry.go errors.go
  mapdata/                   # NEW — NETWORK-FORBIDDEN. World-geometry loader + local registry (§16).
    geometry.go registry.go errors.go

cli/commands/
  home.go (CHANGED)          # runHome delegates to app.Build + tui.NewProgram; plain branch kept.
  geo.go (NEW)               # `bctx geo status|inspect|install|verify` (§15).
  map.go (NEW)               # `bctx map status|verify|install` (§16) — geometry-asset lifecycle.

scripts/
  build_mapdata.sh (NEW)     # one-time offline preprocess: Natural Earth 1:110m -> world-110m.asset
                             #   (coastline polylines + derived country-centroid table) + its sha256.
                             #   Invokes the in-repo generator `tools/mapgen` (see §16). Run at
                             #   release build time only; never at runtime. Output is reproducible.

tools/
  mapgen/                    # NEW — one-time build-time generator (NOT shipped in the runtime binary,
    main.go                  #   NOT imported by tui/*). Reads a local Natural Earth source file and
                             #   emits world-110m.asset + records its sha256. Build-tool, not a dep.
```

`buildEngine`/`withActiveRepo`/`acquisitionAllowed` move (or are re-exported) into `app/` so the TUI and CLI share exactly one wiring path. This resolves audit risk #1 and prevents the TUI from duplicating business logic.

### 1.1 `app.Build(cfg)` contract (concrete — mirrors `buildEngine`)

The real `buildEngine(cfg)` (verified in `cli/commands/engine.go`) returns `(*sdk.Engine, *manager.Manager, func(), error)` and leaves the Engine's `Graph/Features/ML/Detection/Risk/Evidence/Reports` fields **nil** — those services are constructed directly at their call sites (orchestrator, `graph.Service`, `reporting.Service`), exactly as audit §3.2 warns. `app.Build` preserves that reality and does not try to populate the nil fields.

```go
// app/bootstrap.go
type App struct {
  Engine  *sdk.Engine        // Config/Layout/NetworkStatus only; service fields stay NIL
  Cases   *manager.Manager   // case lifecycle (Active/Open/List/OpenRepository)
  Repo    sdk.Repository     // active-case repo, or nil if no case is open
  CaseID  string             // active case id, or "" if none
  Cleanup func()             // closes repo + any registries; always defer app.Cleanup()
}

// Build wires exactly what buildEngine wires: Engine (service fields nil) + Cases + the
// active-case Repo (if a case is open) + a cleanup(). It constructs NO analysis services
// itself — those come from app/services.go on demand so each screen gets a fresh,
// correctly-scoped instance tied to the active case.
func Build(cfg *configs.Config) (*App, error)
```

The TUI MUST NOT read `Engine.Graph/Features/ML/Detection/Risk/Evidence/Reports` (they are nil). It obtains analysis services from `app/services.go` (§1.3).

Switching cases uses **one committed contract — `App.ReopenCase(id)`** (not a full `Cleanup()+Build()` rebuild). This is the single testable case-isolation path and is the method §19.13's isolation assertion targets:

```go
// app/bootstrap.go — the ONE case-switch contract.
// ReopenCase closes the old active-case repo (via the existing cleanup), opens the
// new case's repo through the Cases manager, and resets Repo/CaseID in place.
// Engine/Cases are preserved (they are case-agnostic); only the per-case Repo and
// CaseID change, and Cleanup is re-pointed at the new repo's close. No cross-case
// data can survive because the old repo handle is closed before the new one opens.
func (a *App) ReopenCase(id string) error
```

`Cleanup()+Build()` (rebuilding the whole `App`) is explicitly **not** the case-switch mechanism; it is reserved for process teardown. Committing to `ReopenCase` guarantees case isolation (§12 invariant 4) through a single method the isolation test can exercise directly.

### 1.2 Offline gate contract (replaces the inaccurate "verbatim" claim)

The real gate (verified in `cli/commands/sync.go`) is:

```go
func acquisitionAllowed(cfg *configs.Config, gf *globalFlags) error // errs if gf.offline||gf.airgap||!cfg.AcquisitionAllowed()
```

The TUI version drops the `gf` argument deliberately and this is **safe only because of an upstream guarantee**, which must hold and must be stated: the TUI launches via `runHome` **after** `loadConfig(gf)` has already folded `--offline`/`--airgap` into `cfg.Network.Mode` (audit §3.13), and `configs.Config.AcquisitionAllowed()` returns **false** for both `ModeOffline` and `ModeAirgap` (verified in `configs/config.go`). Therefore:

```go
// app/gates.go — NOT "verbatim" acquisitionAllowed; it wraps only the cfg-mode check and
// RELIES ON loadConfig(gf) having folded --offline/--airgap into cfg.Network.Mode upstream.
func AcquisitionAllowed(cfg *configs.Config) error {
  if !cfg.AcquisitionAllowed() { return acquisition.ErrOfflineAcquisition }
  return nil
}
```

`app.Build` MUST receive the already-flag-applied `cfg` (the one `loadConfig(gf)` produced). If a future caller constructs the TUI with a raw `cfg` that has not had the flags folded in, this gate would under-block — so the contract is: **flags are folded before `Build`.** Acceptance criterion §19.13 asserts this end to end.

### 1.3 `app/services.go` constructors (concrete)

Analysis services are nil on the Engine, so `app/services.go` builds them explicitly with the exact inputs audit §3.6 specifies:

```go
// app/services.go
func NewOrchestrator(repo sdk.Repository, caseID string, modelsDir string) *orchestrator.Orchestrator {
  return orchestrator.New(orchestrator.Options{
    Repo:             repo,
    ModelsDir:        modelsDir,                     // from app.ModelsDir(cfg) — §1.4
    FeatureSchemaSHA: schema.FeatureSchemaSHA256,
    CaseID:           caseID,
    Weights:          scoring.DefaultWeights(),
  })
}
func NewGraph(repo sdk.Repository) *graph.Service          { return graph.NewService(repo) } // MaxNodes=5000
func NewReporting(repo sdk.Repository) *reporting.Service  { return reporting.NewService(repo, reporting.BuildInfo{GeneratedBy: "bctx-tui"}) }
func NewRegistry(modelsDir string) *inference.Registry     { return inference.NewRegistry(modelsDir, schema.FeatureSchemaSHA256) } // MODELS status
```

Each screen that needs analysis asks `app` for the service it needs (the `AppCtx` passed to screen factories, §2, carries these). The orchestrator and registry are `Close()`d by `app.Cleanup` or by the screen's own cancellation path (§11.3).

### 1.4 Models directory resolution (fixes CWD-relative breakage)

Audit risk #7: `repoModelsDir()` returns `models/` relative to CWD, so a TUI launched outside the repo root fails model loads — which makes `AnalyzeWallet` fail-closed and makes MODELS show MISSING even when models are installed. `app.ModelsDir(cfg)` resolves this once, and both the orchestrator and the MODELS-status registry use it.

Two verified facts constrain the resolution order and must be respected (both checked against source for this revision):

1. `configs.Default()` sets `Models.Directory = ~/.bctx/models`, and `Config.normalize()` re-fills it to `~/.bctx/models` whenever empty. So after `configs.Load`, `cfg.Models.Directory` is **never** `""`. A "fall back only when the configured dir is empty" scheme is therefore dead code via the normal config path, and in a **dev checkout** (models live at `<repo>/models`, nothing copied to `~/.bctx/models`) it would resolve `~/.bctx/models`, find nothing, and show MODELS MISSING + fail-closed analysis — reintroducing audit risk #7 in a new disguise.
2. The CLI (`cli/commands/analyze.go`) uses `modelsDir := repoModelsDir()` **unconditionally** = a bare `filepath.Join("models")` (CWD-relative) and **never** consults `cfg.Models.Directory`. So the CLI's real behavior is "always `./models` relative to CWD"; the TUI must not claim to "match the CLI" by reading the configured dir first.

The correct resolution order checks for the *presence* of models at each candidate before accepting it, so both dev and release work:

```go
// app/models.go
func ModelsDir(cfg *configs.Config) string {
  // 1. explicit configured dir, but ONLY if it actually contains models
  //    (release / operator override; avoids a populated-but-empty ~/.bctx/models shadowing a dev ./models)
  if d := cfg.Models.Directory; d != "" && dirHasModels(d) { return d }
  // 2. repo-local ./models if present — the same path the CLI's repoModelsDir() returns (dev checkout)
  if dirHasModels("models") { return "models" }
  // 3. otherwise return the configured dir so MODELS reports MISSING honestly (never fabricate a path)
  return cfg.Models.Directory
}

// dirHasModels reports whether a candidate directory actually holds loadable models:
// anomaly/manifest.json AND flow/manifest.json present (the two fail-closed models the
// orchestrator requires). A directory that exists but lacks these is treated as "no models".
func dirHasModels(dir string) bool // os.Stat(dir/anomaly/manifest.json) && os.Stat(dir/flow/manifest.json)
```

This makes a shipped binary resolve the installed models (step 1) and a dev checkout resolve `./models` (step 2, the same path the CLI returns), and reports MISSING honestly only when neither candidate has models (step 3). MODELS status (§8) and every `AnalyzeWallet` path use `app.ModelsDir(cfg)`, never a bare `repoModelsDir()`.

CLI/TUI divergence is **intentional and documented**: the CLI continues to use its own `repoModelsDir()` (always `./models`); the TUI uses the richer `app.ModelsDir`. The planner MAY optionally switch the CLI's `analyze.go` to call `app.ModelsDir` for consistency, but that is not required by this design and is not claimed as current behavior. Acceptance §19.14 asserts both a release case (configured dir with models, non-repo CWD) and a dev-checkout case (models at `./models`, `~/.bctx/models` empty, CWD = repo root) show MODELS = LOADED.

---

## 2. Information Architecture & Screen Registry

There are 15 screens. This is a deliberate +1 over the brief's enumeration of ~14 primary sections: BCTX splits **Wallet** and **Entity Cluster** into two screens (distinct seams — orchestrator `AnalyzeWallet` vs `InvestigationResult.Clusters`/common-input detection) and adds **Timeline** as a *contextual, non-nav* screen (`Nav=false`, opened from a subject via `reporting.Service.Timeline`), not a top-level nav item. The delta is scope refinement, not scope creep: every screen below maps to a real service seam. Every screen is a value registered in a **centralized screen registry** (`tui/router.go`) so navigation, help, and the command palette are generated from one source of truth — no screen is reachable unless it is registered, and no nav item exists unless it maps to a real service (audit §3, brief §7).

```go
// tui/router.go
type ScreenID string

type RenderCap uint8 // bitfield of capabilities a screen needs
const (
    CapTable RenderCap = 1 << iota
    CapGraph
    CapMap
    CapTimeline
    CapStream
    CapDetail
    CapForm
)

type ScreenDef struct {
    ID        ScreenID
    Title     string      // shown in top bar / breadcrumb
    Key       string      // single-key nav shortcut (global keymap)
    Caps      RenderCap   // drives which components the screen mounts
    Help      string      // one-line context help (status bar + help overlay)
    Nav       bool        // appears in SideNav (Help/Timeline are contextual, Nav=false)
    New       func(*AppCtx) Screen // factory; AppCtx carries engine/services/theme/size
}

type Screen interface {
    Init() tea.Cmd
    Update(tea.Msg) (Screen, tea.Cmd)
    View(Frame) string      // Frame = allocated rect from the shell (§6)
    ShortHelp() []key.Binding
}
```

Registry (exact set, keys are the global nav shortcuts):

| ID | Title | Key | Caps | Primary service seam (audit ref) | Context help |
|---|---|---|---|---|---|
| `home` | Dashboard | `1` | Table, Detail | `Repo.Counts`, `NetworkStatus`, `ListAlerts`, ml manifest status | Case overview, counts, network/model state |
| `search` | Search | `2` `/` | Table | `Repo.GetWallet/GetTransaction/NetworkObservationsByIP` | Find a wallet, tx, or IP in the active case |
| `wallet` | Wallet / Entity | `3` | Detail, Table, Graph | orchestrator `AnalyzeWallet`, `WalletTransactions` | Analyze a wallet: risk, signals, related, subgraph |
| `transaction` | Transaction | `4` | Detail, Table | `GetTransaction`, `EdgesFrom/EdgesTo` | Inspect a transaction and its edges |
| `entity` | Entity Cluster | `5` | Detail, Graph | `InvestigationResult.Clusters`, detection common-input | Clustered addresses for the subject |
| `graph` | Graph | `6` `g` | Graph | `graph.Service.Subgraph/NeighborsDepth/Path` (MaxNodes=5000) | Explore the bounded transaction graph |
| `network` | Network / IP | `7` | Table, Detail | `NetworkObservationsByIP`, `AllNetworkObservations` | Network observations by IP / ASN |
| `geomap` | Geo Map | `8` `m` | Map | geoip pkg + mapdata + `NetworkObservation.Country/ASN` | Country-binned observation density (metadata) |
| `detection` | Detection | `9` | Table, Detail | `InvestigationResult.Patterns/Predictions` | Pattern + anomaly/flow model outputs |
| `alerts` | Alerts | `a` | Table, Detail | `ListAlerts`, `ListMonitorAlerts` | Case alerts and monitor alerts |
| `monitoring` | Live Monitoring | `o` | Stream, Table | `monitoring.Service.MonitorWallet`, `ListMonitorSessions/Events/RiskDeltas` | Live/historical monitor sessions |
| `data` | Data / Ingestion | `d` | Table, Detail | `ListDatasets/GetDataset/GetCheckpoint`, `graph.Builder.GraphStats` | Datasets, checkpoints, graph build |
| `reports` | Reports | `r` | Table, Detail | `reporting.Service` BuildSnapshot/Render/Timeline/ExportBundle/Verify | Build, preview, export, verify reports |
| `settings` | Settings | `s` | Form, Detail | `configs.Config`, `cases` list, geoip registry | Config, cases, geoip DB status (read-mostly) |
| `help` | Help | `?` | Detail | static keymap registry | Keyboard reference (overlay, Nav=false) |
| `timeline` | Timeline | `t` | Timeline | `reporting.Service.Timeline(snap)` | Chronological events for the subject |

Navigation flows: Dashboard is the landing screen. From Search, selecting a result pushes the matching detail screen (wallet/transaction/network) and sets the global **selected subject**. Wallet → `g` opens Graph centered on the subject; Wallet → `t` opens Timeline for the subject's snapshot; Wallet → `r` opens Reports pre-seeded with the subject. The registry's `Key` fields define direct jumps; `Esc` pops back along the navigation stack (§4).

---

## 3. Component System

No monolithic `Update`/`View`. The root model composes an **AppShell**, which composes exactly one active **Screen**, which composes **components**. Each component is an independent Bubble Tea sub-model (`Model`, `Init() tea.Cmd`, `Update(tea.Msg) (self, tea.Cmd)`, `View(Frame) string`) and owns only its own ephemeral UI state. Parents forward sized `Frame`s down and bubble `tea.Cmd`s up.

Shell + chrome:
- **AppShell** — the frame. Lays out TopBar / SideNav / main workspace / ContextBar per breakpoint (§6), hosts the NotificationLayer and Modal/HelpOverlay as overlays, and routes keys to global keymap first, then the active screen.
- **TopBar** — persistent status line (§8): BCTX wordmark, active case, NETWORK, ACQUISITION, provider, MODELS, local DB, current screen title/breadcrumb, selected subject, clock. Values are read from live services; never faked.
- **SideNav** — vertical list of registry screens where `Nav=true`, highlighting the active one; shows nav key hints; collapses to an icon/initial rail in compact mode.
- **StatusBar / ContextBar** — bottom bar: left = contextual keyboard hints (from the active screen's `ShortHelp()`), center = last event/alert, right = transient status/error. This is the "event/alert/command context" band of the shell.
- **Breadcrumb** — rendered inside TopBar; shows `Case ▸ Screen ▸ Subject` from the nav stack.

Data/display primitives:
- **Table** — wraps `bubbles/table`; virtualized/paginated (§11.4); columns reflow by breakpoint; emits a `RowSelected{id,kind}` msg. Used by home, search, network, detection, alerts, data, reports, monitoring history.
- **DetailPanel** — labeled key/value + sectioned body for an `InvestigationResult`/row; wraps a `bubbles/viewport` for scroll. Renders risk block (§10), signals, related wallets, relevant txs, evidence, patterns.
- **SplitView** — composes two child components (list|detail, or graph|detail) with a configurable ratio; collapses to a single pane with a toggle in compact mode.
- **Modal** — the **single** modal subsystem. All overlays (confirm, input prompt, error expand, "start monitor?" network-gate confirmation) route through one `Modal` stack so only one modal shows and `Esc` closing is uniform. No ad-hoc popups elsewhere.
- **CommandPalette** — fuzzy command/screen launcher over the registry + a fixed verb list (analyze, build report, start monitor, open case, geo inspect…). Opened with `:` or `Ctrl+P`.
- **SearchBox** — `bubbles/textinput` for the global `/` search and in-screen filters.
- **NotificationLayer** — transient toasts from global notifications (§11.1); auto-expire; stack cap 5; `Esc`/`enter` dismiss. Overlay, never steals focus.
- **HelpOverlay** — full keymap reference generated from the keymap registry; opened with `?`; `Esc` closes. Overlay (`help` screen `Nav=false`).

Domain visualizers:
- **GraphView** — renders a `schema.Subgraph` (§9). Pan/zoom/depth/select/focus/filter/path. Shows the engine's MaxNodes/MaxDepth limits in a corner badge.
- **MapView** — terminal-native Braille/half-block world renderer (§16) plotting country-binned observation density (§13). In-repo renderer only.
- **TimelineView** — vertical, scrollable chronology of `reporting.TimelineEvent`s (`bubbles/viewport`), grouped by day. `reporting.models.TimelineEvent` is `{Kind, Timestamp, ID, Detail}` (verified) with `Kind ∈ {tx, monitor_event, risk_delta, alert}` — there is **no per-event severity field**. The view therefore renders **one glyph per `Kind`** (tx `▣`, monitor event `◇`, risk delta `Δ`, alert `!`), not a severity glyph. A severity may be shown **only on `alert`-kind rows**, and only by looking the alert up in the snapshot's alerts by `ID`; all other kinds carry no severity. The view never implies a severity exists on every row.
- **AlertList** — specialized Table variant with severity glyphs/colors for `schema.Alert` / `MonitorAlertRow`.
- **MonitorStream** — append-only, auto-scrolling log of `MonitorEventRow`/`RiskDeltaRow` from a live or historical session; shows session health; honors cancellation (§11.3).

---

## 4. State Model & Top-Level State Machine

The root owns **global** state; everything else is ephemeral and lives in the active screen/components.

```go
// tui/app.go
type Mode string // the top-level state machine states
const (
  ModeHome Mode = "home"; ModeSearch="search"; ModeWallet="wallet"
  ModeTransaction="transaction"; ModeEntity="entity"; ModeGraph="graph"
  ModeNetwork="network"; ModeMap="map"; ModeDetection="detection"
  ModeAlerts="alerts"; ModeMonitoring="monitoring"; ModeReports="reports"
  ModeData="data"; ModeSettings="settings"; ModeHelp="help"; ModeModal="modal"
)

type GlobalState struct {
  Case        schema.Case        // active case (from Cases.Active)
  Subject     Subject            // selected context: {ID, Kind: wallet|tx|ip|entity}
  Network     state.Status       // from engine.NetworkStatus(ctx); UPPERCASE constants
                                 //   state.Connected/Disconnected/Airgapped — honest, no socket when offline
  Acquisition AcqState           // enabled|paused|offline|airgapped, from cfg + gate
  Models      ModelStatus        // from ml/inference manifests (loaded/missing/schema-mismatch)
  GeoIP       GeoStatus          // from tui/geoip registry (installed/missing/version)
  Notes       []Notification     // global notification queue
  Size        Breakpoint         // current layout class (§6)
}

type Root struct {
  ctx     context.Context
  app     *app.App              // shared bootstrap (engine/services/cleanup)
  mode    Mode
  stack   []ScreenID            // navigation stack for Esc-back
  shell   components.AppShell
  active  Screen                // current screen (from registry factory)
  modal   components.ModalStack
  state   GlobalState
}
```

Transitions: a `NavMsg{to}` pushes the current screen onto `stack`, builds the target via the registry factory, and swaps `active`. `Esc` pops the stack (or, if a modal is open, closes the top modal first; `ModeModal` is overlaid, not a stack entry). Opening a modal sets a modal overlay flag; it does not replace the active screen. `q`/`Ctrl+C` from the top level quits; from within a screen `q` is only quit when no input field has focus (SearchBox/CommandPalette capture it).

Rule: the root owns **case + selected subject + global notifications + network/acquisition/model/geoip status** and nothing else. Screens never mutate canonical forensic state; they request work via `tea.Cmd`s (§11) and render the results they receive back as `tea.Msg`s. The TUI holds no canonical forensic state — only presentation state (scroll offsets, selection, filter text, zoom/pan).

---

## 5. Keyboard Model

Keymaps are declared with `bubbles/key` in `tui/keys.go` and surfaced through `bubbles/help`. There is a global keymap plus per-context keymaps; the HelpOverlay and ContextBar are generated from these bindings (single source of truth).

Global (active unless a text input has focus):

| Key | Action |
|---|---|
| `↑`/`k`, `↓`/`j` | move selection up/down |
| `Enter` | activate selection / drill in |
| `Esc` | back (pop nav stack) / close modal / clear filter |
| `Tab` / `Shift+Tab` | next / previous panel focus |
| `/` | global search (opens Search screen or in-screen SearchBox) |
| `:` or `Ctrl+P` | command palette |
| `1`–`9`, `a o d r s t g m ?` | jump to the screen whose registry `Key` matches |
| `q`, `Ctrl+C` | quit |

Graph keymap (adds): `+`/`-` zoom, `h/j/k/l` or arrows pan, `[`/`]` decrease/increase depth, `f` focus/center selection, `F` filter by node type, `p` path mode (pick src then dst), `n`/`N` next/prev node, `0`/`r` reset view.
Map keymap (adds): `+`/`-` zoom, arrows pan, `c` cycle bin granularity (country/ASN), `0` reset.
Table keymap (adds): `g`/`G` top/bottom, `PgUp`/`PgDn` page, `f` filter (opens SearchBox), `Enter` open row.
Monitor keymap (adds): `space` pause/resume follow (auto-scroll), `x` stop session (cancels ctx), `Enter` open event detail.

When a focused `textinput` (SearchBox/CommandPalette) is active, only `Esc`, `Enter`, and editing keys apply; single-letter nav is suppressed so typing works. This is enforced in `AppShell.Update` by checking focus before dispatching the global keymap.

---

## 6. Responsive Layout

The only source of terminal size is `tea.WindowSizeMsg`. The root stores `Size` and classifies it into a breakpoint; the shell recomputes rects and passes a `Frame{X,Y,W,H}` to each region. Nothing reads `os.Stdin`/env for size, and no size is assumed.

Breakpoints (by terminal width):

| Class | Width | SideNav | Workspace | Behavior |
|---|---|---|---|---|
| compact | < 100 | hidden (icon rail or toggle with `Tab`) | full width, single pane | SplitView collapses to one pane with a toggle; TopBar sheds least-critical fields first (clock, provider), keeping NETWORK/ACQUISITION/CASE/MODELS |
| standard | 100–159 | 20 cols, labeled | remainder, SplitView allowed 1:1 or 2:3 | default target |
| wide | 160–199 | 24 cols | SplitView + optional right detail rail | detail panel can pin open alongside list |
| ultrawide | ≥ 200 | 28 cols | three columns (nav · list · detail) | graph/map get more canvas; tables show more columns |

Reflow rules: panels never overlap; the shell computes region heights as TopBar(1–2 rows) + body + ContextBar(1–2 rows) and the body width as `W − navWidth`. Each component is told its exact `Frame` and must clamp/truncate (lipgloss width) rather than overflow. Minimum supported size is **80×24**; below that, show a single centered message "Terminal too small — resize to at least 80×24" and nothing else (no panic, no misrender). The design must render cleanly at 80×24, 100×30, 120×40, 160×50, 200×60 — the planner's acceptance set.

Degraded rendering: the theme exposes a **degraded ASCII profile** (§7) selected when the terminal reports no color or `TERM`/`NO_COLOR` indicates limited capability (detected via lipgloss color profile). In degraded mode: ASCII box borders instead of rounded Unicode, ASCII glyphs for graph/map/severity, and single-attribute styling. The TUI uses **no truecolor-only features and no sixel** — all color goes through lipgloss's adaptive profile so 256-color and 16-color terminals downgrade gracefully.

---

## 7. Theme System

All color/spacing/border decisions live in `tui/theme/`. Views consume **semantic tokens only**; no hard-coded `lipgloss.Color` anywhere else (this replaces the current inline bitcoin-orange styling in `tui/screens/home.go`). This satisfies brief §3 and audit risk #3.

`tokens.go` — raw palette (BCTX identity, deliberately distinct from the reference image: a cyan/teal information language, not an orange brand). Each color is an adaptive triple (truecolor / 256 / 16) so downgrade is automatic:

```go
// tokens.go — raw values only, no meaning attached
var Palette = struct {
  BgBase, BgPanel, BgRaised  lipgloss.AdaptiveColor
  Border, BorderActive       lipgloss.AdaptiveColor
  TextPrimary, TextDim, TextInverse lipgloss.AdaptiveColor
  Cyan, Teal   lipgloss.AdaptiveColor // primary information language
  Green        lipgloss.AdaptiveColor // healthy / local / confirmed
  Blue         lipgloss.AdaptiveColor // neutral information
  Amber        lipgloss.AdaptiveColor // warning
  Red          lipgloss.AdaptiveColor // high-risk (restrained use)
}{ /* dark workspace: BgBase ~#0B0F12, Cyan ~#30D5E8, Teal ~#17B2A3,
     Green ~#3FB950, Blue ~#4C8DF6, Amber ~#E3B341, Red ~#F05C5C,
     TextPrimary ~#E6EDF3, TextDim ~#7D8A96 */ }

var Space = struct{ PanelPadX, PanelPadY, Gap int }{1, 0, 1} // compact, high-density
```

`theme.go` — semantic mapping (role → token) and the `Theme` struct the app passes down. Roles express meaning, never raw color:

```go
type Theme struct {
  Role     map[Role]lipgloss.AdaptiveColor // Title, Label, Value, Info, Healthy, Warning, Critical, Neutral, Selected, Muted
  Border   lipgloss.Border
  Degraded bool
}
func Default() Theme    // dark BCTX identity, Unicode borders
func Degraded() Theme   // ASCII borders, 16-color safe
```

Risk-role mapping (used by §10): `Healthy`→Green, `Neutral/Info`→Cyan/Teal, `Warning`→Amber, `Critical`→Red. Bright text = `TextPrimary` for primary data; dim = `TextDim` for metadata/labels. Borders are consistent (one `theme.Border`), monospaced throughout, spacing compact.

`styles.go` — `lipgloss.Style` builders that take the active `Theme` and return styles for panels, titles, labels, values, selected rows, severity chips, badges. Components call these; they never build styles from raw colors. The degraded profile swaps border + collapses adaptive colors to the 16-color column.

---

## 8. Top Status Bar (honest state)

The TopBar reflects only real service state (brief §6, audit risk #4). Fields and sources:

| Field | Source | Rendering |
|---|---|---|
| `BCTX` | static wordmark | Title role |
| CASE | `Cases.Active()` | value or `(none)` |
| NETWORK | `engine.NetworkStatus(ctx)` → `state.Connected`/`state.Disconnected`/`state.Airgapped` (UPPERCASE constants) | Healthy if `state.Connected`, Warning if `state.Disconnected`, Neutral if `state.Airgapped`; the lowercase label (`connected`/etc.) is a display transform in the status/theme layer — comparisons use the constants, never the lowercase literal — never faked "connected" |
| ACQUISITION | `cfg.AcquisitionAllowed()` + mode | `ENABLED`/`PAUSED`/`OFFLINE`/`AIRGAPPED` |
| PROVIDER | `cfg` provider name | value (dim) |
| MODELS | `ml/inference` registry built with `app.ModelsDir(cfg)` (§1.4) — manifest status (loaded + schema-SHA match vs `schema.FeatureSchemaSHA256`) | `LOADED`/`MISSING`/`SCHEMA-MISMATCH` — replaces hard-coded `"PENDING"`; because the dir is resolved via `cfg.Models.Directory` (not a bare CWD-relative `models/`), MODELS reflects installed models regardless of launch CWD |
| DB | `Repo.Counts` reachable | `LOCAL` (Healthy) / `NO CASE` |
| GEOIP | `tui/geoip` registry | `v<version>` or `NOT INSTALLED` |
| SUBJECT | global selected subject | id (truncated) |
| TIME | `time.Now()` via a 1s `tea.Tick` | HH:MM:SS |

Status values are computed by `tea.Cmd`s on startup and refreshed on relevant events (case change, after a sync/monitor action). The TUI never writes "LIVE" over stored data (AGENTS §16/§17): the monitoring screen is the only place that may say LIVE, and only while a `MonitorWallet` loop is actually running.

---

## 9. Graph UX

The GraphView renders a `*schema.Subgraph` fetched from the existing bounded engine (`graph.Service`, audit §3.5) — the TUI never builds or traverses the graph itself.

- Data source: `graph.Service.Subgraph(ctx, center, depth)` for the view; `NeighborsDepth` for incremental expansion; `Path(ctx, src, dst)` for path mode. `MaxNodes = 5000` is a property of the service; the view reads it and renders a **limit badge** e.g. `nodes 212/5000 · depth 2/[cap]`. If `Subgraph` returns the capped set, the badge turns Warning and a note says "result truncated at engine cap".
- Interaction (keymap §5): pan, zoom (level of detail — fewer labels when zoomed out), depth `[`/`]` (re-queries `Subgraph` with new depth as a cancellable Cmd), select (`n`/`N`, arrows), focus/center (`f`), filter by node type (`F`), search within loaded nodes (`/`), reset (`0`), path mode (`p` → pick src, pick dst, call `Path`, highlight the returned id sequence).
- Node glyphs (distinct per `schema.NodeType`): wallet `◆`, transaction `▣`, ip `◉`, entity `⬢` (degraded ASCII: `W` `T` `I` `E`). Edge direction is shown with ASCII connectors; edge type (`input_to`/`output_to`/`sent_to`/`observed_with`/`member_of`) shown on selection in the side DetailPanel.
- **Node risk coloring (data-honest):** `schema.GraphNode` has `Risk float64` and `Label string` fields, but `graph.Service.assemble()` constructs every node as `{ID, Type}` only — `Risk` and `Label` are **never populated** by `Subgraph`, `NeighborsDepth`, or `Neighbors` (verified), and the orchestrator's attached depth-2 subgraph inherits those unpopulated nodes. The TUI must therefore NOT color arbitrary nodes by `GraphNode.Risk` (it is always zero) and must NOT print a per-node "risk N" for non-subject nodes. The design rule is: **only the subject/center node carries a risk value**, resolved at render time by cross-referencing the center id against the already-loaded `InvestigationResult.Risk.Score` (presentation-layer join, no backend change). **All non-subject nodes render in the Neutral role with no numeric risk**, because the graph service does not score nodes. Populating `GraphNode.Risk` in the service would be a backend change outside Phase 8 scope (it would need the graph/analysis agent plus an acceptance test) and is explicitly not undertaken here.
- Depth changes and center changes dispatch new Cmds (§11) and cancel the previous one, so rapid `[`/`]` presses don't pile up queries. **Depth control (`[`/`]`) applies only to the Graph screen's own `graph.Service.Subgraph(ctx, center, depth)` queries.** The subgraph embedded in an `InvestigationResult` (e.g. rendered in the Wallet detail pane) is the orchestrator's **fixed depth-2 snapshot** (`o.graph.Subgraph(ctx, address, 2)`, baked into `AnalyzeWallet`) and is **not** adjustable in place; to re-query at a different depth the user presses `g` to switch to the Graph screen centered on the subject, which then owns the live `Subgraph` query and the depth keymap.
- The visible node budget for rendering is independently capped (default 800 drawn) with a "zoom/filter to see more" hint — the engine cap and the render cap are both honest and both shown.

---

## 10. Risk Presentation

Risk is always shown as the engine produced it; the TUI invents no forensic meaning (brief, audit §3.8, CLAUDE Rule 6).

- The **numeric score (0–100) is always displayed**, alongside **confidence (0–1)**, the list of **contributing signals** (`RiskAssessment.Signals[]`, each with its name/weight/contribution), and the **delta** vs previous when `Previous`/`Delta` are present (monitoring). The DetailPanel renders: `RISK 72/100 · conf 0.81 · Δ +9` then the signals table.
- Presentation buckets map the numeric score to a label **for color/emphasis only**, with documented thresholds shown in Help and Settings:

  | Bucket | Score range | Role/color |
  |---|---|---|
  | LOW | 0–24 | Healthy (green) |
  | ELEVATED | 25–49 | Neutral (cyan) |
  | HIGH | 50–74 | Warning (amber) |
  | CRITICAL | 75–100 | Critical (red) |

  These thresholds are presentation constants in `tui/theme` (or a small `tui/riskband.go`), documented in-app, and never alter the numeric score or the engine's own classification. The bucket is a visual cue; the number is the truth. Red is used sparingly — only CRITICAL and genuine error states.

---

## 11. Async Work, Notifications, and Large-Data Safety

### 11.1 Notifications & empty/loading/error UX
Every data region has four explicit states driven by messages: **loading** (spinner + "querying…"), **empty** ("no transactions in this case for <subject>" — honest, never fabricated rows), **error** (one-line in ContextBar + expandable Modal with the error text), **loaded** (content). Global events (sync finished, report exported, model missing) raise a `Notification{level, text}` into the root queue and surface as a NotificationLayer toast. Nothing silently fails.

### 11.2 Background work off the UI thread
Every service call is a `tea.Cmd` (`tui/commands.go`) that runs on Bubble Tea's goroutine pool and returns a `tea.Msg` (`DataLoaded`/`DataError`). The UI thread never blocks on I/O or analysis. Example shape:

```go
func analyzeCmd(ctx context.Context, orch *orchestrator.Orchestrator, addr string, offline bool) tea.Cmd {
  return func() tea.Msg {
    res, err := orch.AnalyzeWallet(ctx, addr, offline)
    if err != nil { return DataError{Screen: ModeWallet, Err: err} }
    return WalletLoaded{Result: res}
  }
}
```

### 11.3 Cancellation (no goroutine leaks)
The program runs under `tea.WithContext(ctx)` (kept from current `runHome`). Long operations (orchestrator analysis, graph re-query, monitor loop) derive a child `context.Context` stored on the screen; it is cancelled on `Esc`, on screen change, on a new query superseding the old, and on quit. The monitor loop (`monitoring.Service.MonitorWallet`) is started in a Cmd with its own cancel func exposed to the screen; `x`/stop/screen-change/quit all call it. `orchestrator.Close()` and `repo.Close()` are deferred by the shared `app` cleanup. No goroutine outlives the screen that started it.

### 11.4 Large-data safety
- Tables: `bubbles/table` with pagination; reads use the repository `limit` parameters (`WalletTransactions(ctx,addr,limit)`, `AllTransactions(ctx,limit)`, `ListMonitorEvents(ctx,id,limit)`, `ListAlerts(ctx,limit)`). The TUI pages via limit/offset-style requests; it never pulls an unbounded result set into memory.
- Graph: bounded by the engine's `MaxNodes=5000` plus the render cap (§9).
- Map: observations are clustered/binned by country/ASN before rendering (§13), so point count is bounded by the number of countries/ASNs, not raw observations.
- Timeline/MonitorStream: `bubbles/viewport` virtualization; only visible rows are rendered.

---

## 12. Offline Behavior & Hard Invariants

The TUI enforces (by construction, verified by the offline-boundary guard tests):

1. **Acquisition is the only network boundary.** The only screens that can touch the network are Monitoring (start session) and Data (sync) — and both MUST call the offline gate `app.AcquisitionAllowed(cfg)` (§1.2) **before** constructing a provider. This is **not** the verbatim `acquisitionAllowed(cfg, gf)`: it wraps only the `!cfg.AcquisitionAllowed()` check and relies on `loadConfig(gf)` having already folded `--offline`/`--airgap` into `cfg.Network.Mode` upstream (both modes make `cfg.AcquisitionAllowed()` false). `app.Build` therefore must receive the flag-applied `cfg`. If the gate returns `acquisition.ErrOfflineAcquisition`, the action is blocked with an honest message, not attempted.
2. **`tui`, `tui/geoip`, `tui/mapdata` dial nothing.** They import no `net/http`/`crypto/tls`/`golang.org/x/net` transport. This is enforced by the dedicated, scoped guard `TestTUIPackagesHaveNoHTTPTransport` (§17.1) — a new test modeled on `TestMonitoringAndReportingNoHTTPTransport`, NOT by adding `tui` to the analysis `analysisDirs` rule (which would wrongly flag the legitimate, network-gated `tui → acquisition`/`tui → monitoring` edges the Monitoring/Data screens need). The two leaf packages `tui/geoip` and `tui/mapdata` are additionally forbidden from importing `acquisition`/`monitoring` or any bare net dial — they are strictly offline. The geoip package is explicitly NETWORK-FORBIDDEN (§15).
3. **Analysis / graph / monitor-history / report / map rendering never implicitly fetch.** They read the local repository and local assets only; reporting never dials even when connected (audit §3.10).
4. **Case isolation is absolute.** All reads go through the active case's `Repo` (per-case SQLite under `~/.bctx/cases/<id>/`). Switching cases goes through the single `App.ReopenCase(id)` contract (§1.1), which closes the old case's repo before opening the new one; no cross-case data is ever shown.
5. **No fake UI data.** Empty states say empty; demo/fixture data (if ever shown) is labeled DEMO/SIMULATED/FIXTURE (AGENTS §16). Status fields reflect real service state only.
6. **Geolocation is metadata, not ownership proof** (§13).
7. **Graph limits are visible** (§9).

---

## 13. Map UX & Geolocation Honesty

`schema.NetworkObservation` has `Country` and `ASN` but **no lat/lon** (audit §4). The Geo Map therefore plots a presentation-only, offline mapping and is explicit about what it represents.

Pipeline (all offline, all presentation-layer): `NetworkObservationsByIP`/`AllNetworkObservations` → for each observation, resolve a **country ISO code** via the local `tui/geoip` MMDB lookup on `SrcIP`/`DstIP` (or fall back to the stored `Country`/`ASN`) → map that ISO code to a **plot coordinate via the `tui/mapdata` centroid table** (the geoip reader returns only an ISO code and ASN, never coordinates — the lat/lon comes from `tui/mapdata`, §16) → bin/cluster by country or by ASN → the in-repo MapView renderer draws density at those per-country **centroid** coordinates over a low-res coastline.

The centroid lat/lon is load-bearing (audit §4 confirms `NetworkObservation` carries `Country`/`ASN` but no lat/lon), so its provenance is specified, not assumed. The country-centroid table is **derived deterministically at preprocess time from the shipped Natural Earth 1:110m admin-0 polygons** (§16) — each country's centroid is the representative point (pole-of-inaccessibility / polygon centroid) of its admin-0 geometry, computed by the same one-time offline build step that reduces the geometry asset. Because it is derived from the public-domain Natural Earth source, it **inherits Natural Earth's public-domain license and is covered by the geometry asset's single sha256** (it is emitted as part of the `world-110m.asset`, not a separate downloaded artifact); it has no independent source/license/version/checksum because it is not an independently sourced dataset. §16's table records this derivation explicitly.

No-centroid fallback: if a resolved country has no centroid in the derived table (e.g. an ISO code absent from the Natural Earth admin-0 set, or a corrupt asset), that country is shown as a **list-only bin with its density count and no plotted point**, consistent with the no-DB degrade path — never a fabricated coordinate.

Honest wording is mandatory and lives in the view copy:
- A point is labeled "**observed from IP** <ip>" (the fact), with "**IP geolocation estimate**: <country> (DB-IP IP-to-Country Lite)" and "**ASN metadata**: AS<n> <org>" as clearly-separate derived lines.
- The map NEVER says or implies "wallet belongs to country X". The screen header carries a persistent note: "Geolocation is network-observation metadata, not proof of custody or ownership."
- When no GeoIP DB is installed, the map still renders the coastline and shows country bins only for observations that already carry a stored `Country`; IPs without a stored country are listed as "ungeolocated (no DB)".

---

## 14. ASCII Wireframes

### Persistent App Shell (standard breakpoint, ~120×36)
```
┌ BCTX ─ CASE investigation-01 ─ NET ●offline ─ ACQ paused ─ MODELS loaded ─ GEOIP v2024.11 ─ 14:03:22 ┐
│ ▸ wallet bc1q…k3 (subject)                                                                            │
├────────────┬──────────────────────────────────────────────────────────────────────────────────────┤
│ NAV        │ MAIN WORKSPACE                                                                           │
│ 1 Dashboard│                                                                                          │
│ 2 Search   │                                                                                          │
│ 3 Wallet ◂ │                                                                                          │
│ 4 Tx       │                                                                                          │
│ 6 Graph    │                                                                                          │
│ 8 Geo Map  │                                                                                          │
│ a Alerts   │                                                                                          │
│ o Monitor  │                                                                                          │
│ r Reports  │                                                                                          │
├────────────┴──────────────────────────────────────────────────────────────────────────────────────┤
│ ↑↓ move · enter open · / search · : palette · g graph · r report · q quit    [last: sync ok · 0 err]│
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Dashboard
```
┌ Dashboard ──────────────────────────────────────────────────────────────────┐
│ ┌ Case ────────────────┐ ┌ Local corpus ──────────┐ ┌ System ─────────────┐ │
│ │ id   investigation-01 │ │ transactions   12,480  │ │ NETWORK   offline   │ │
│ │ opened  2024-11-02    │ │ wallets         3,201  │ │ ACQ       paused    │ │
│ │ subject bc1q…k3       │ │ network recs    8,772  │ │ MODELS    loaded 3/3│ │
│ └───────────────────────┘ │ edges          41,009  │ │ GEOIP     v2024.11  │ │
│ ┌ Recent alerts ─────────┐ │ alerts             7   │ └─────────────────────┘ │
│ │ ! CRITICAL  bc1q…k3  Δ+22 structuring_like   14:01 │                        │
│ │ · HIGH      3F2a…9x  Δ+8  peel_chain_like    13:40 │                        │
│ │ · ELEVATED  tx 9ab…   new high-value output   13:12 │                       │
│ └──────────────────────────────────────────────────┘                         │
└───────────────────────────────────────────────────────────────────────────────┘
```

### Wallet / Entity (SplitView: list | detail)
```
┌ Wallet  bc1q…k3 ──────────────────────────┬ Analysis ────────────────────────┐
│ Transactions (limit 50)        ▸ page 1/5 │ RISK  72/100  conf 0.81  Δ +9     │
│  txid        value     dir   when         │ band  HIGH (50–74)                │
│  9ab3…        0.42 ₿   in    11-02 14:01  │ ── signals ───────────────────    │
│  7c1d…        0.10 ₿   out   11-02 13:40  │  structural_flow     +28          │
│  2fe8…        1.20 ₿   in    11-01 09:12  │  anomaly_score       +19          │
│  …                                        │  common_input_links  +15          │
│                                           │ ── related wallets (6) ───────    │
│                                           │  3F2a…9x  bc1qd…7m  …             │
│                                           │ ── evidence (4) ──────────────    │
│                                           │  HIGH  common-input cluster  →4   │
│ [enter] open tx   [g] graph   [t] timeline│  MED   peel chain pattern    →2   │
└───────────────────────────────────────────┴───────────────────────────────────┘
```

### Transaction
```
┌ Transaction  9ab3… ───────────────────────────────────────────────────────────┐
│ status confirmed   block 842,113   value 0.42 ₿   fee 1,240 sat   2024-11-02   │
│ ┌ inputs (3) ───────────────────┐ ┌ outputs (2) ──────────────────────────┐   │
│ │ bc1q…k3      0.30 ₿            │ │ 3F2a…9x     0.41 ₿   (sent_to)        │   │
│ │ bc1qd…7m     0.08 ₿           │ │ bc1q…k3     0.0088 ₿ change           │   │
│ │ bc1q…aa      0.04 ₿           │ └───────────────────────────────────────┘   │
│ └───────────────────────────────┘   edges: input_to×3 · output_to×2           │
│ [enter] open address   [g] graph centered here                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Graph
```
┌ Graph  center bc1q…k3 ─ depth 2 ─ nodes 212/5000 ─ drawn 212/800 ───────────────┐
│                       ▣ 9ab3                                                     │
│            ◆ bc1q…k3 ──input_to──▣ 7c1d──output_to──◆ 3F2a…9x                   │
│                 │                                        │                        │
│             observed_with                            member_of                   │
│                 │                                        │                        │
│                 ◉ 203.0.113.9                        ⬢ cluster-04                 │
│                                                                                  │
│ selected: ◆ bc1q…k3  risk 72  type wallet  (center node — risk from analysis)    │
│ [+/-] zoom  [←↑↓→] pan  [ [ ] ] depth  [f] focus  [F] filter  [p] path  [0] reset│
└──────────────────────────────────────────────────────────────────────────────────┘
```
(The `risk 72` readout appears **only** when the selected node is the analysis subject/center — joined from `InvestigationResult.Risk.Score` per §9. Selecting any non-subject node shows `type <t>` and `risk —` because `graph.Service` does not score nodes.)

### Geo Map
```
┌ Geo Map ─ country bins ─ 42 observations across 9 countries ────────────────────┐
│  Geolocation is network-observation metadata, not proof of custody/ownership.    │
│       ⠀⠀⢀⣤⣶⣦⡀⠀⠀⠀⠀⣠⣤⣄⠀⠀⠀⠀⠀⣀⣠⣤⣤⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀                               │
│    ⢀⣴⣿⣿⣿⣿⣿⣷⡄  •DE(12) ⣾⣿⣿⣿⡆  •US(9)   ⣠⣾⣿⣿⣿⣿⣿⣷⡀  •SG(6)                        │
│    ⠈⠛⠿⠿⠟⠋    •NL(5)         •RU(4)        •JP(3)                                │
│ selected bin: DE — 12 observations · top ASN AS3320 Deutsche Telekom              │
│   observed from IP 203.0.113.9 · IP geolocation estimate: DE (DB-IP Lite)         │
│ [+/-] zoom  [←↑↓→] pan  [c] bin by ASN  [0] reset        GEOIP v2024.11           │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### Alerts
```
┌ Alerts ─ case 7 · monitor 3 ────────────────────────────────────────────────────┐
│  sev       subject   trigger            Δrisk  status      when                  │
│  CRITICAL  bc1q…k3   structuring_like   +22    NEW         11-02 14:01           │
│  HIGH      3F2a…9x   peel_chain_like    +8     REVIEWING   11-02 13:40           │
│  ELEVATED  tx 9ab3   high_value_output  +4     NEW         11-02 13:12           │
│ [enter] open evidence   [a] filter status                                        │
└────────────────────────────────────────────────────────────────────────────────────┘
```

### Live Monitoring (stream)
```
┌ Monitoring  session mon-7f3 ─ ●LIVE ─ health ok ─ polls 48 ─ new 3 ─ dup 45 ────┐
│ 14:03:10  event   tx 9ab3  confirmed  block 842,113                              │
│ 14:03:10  Δrisk   63 → 72  (+9)   signal: structural_flow                       │
│ 14:03:10  ALERT   CRITICAL structuring_like  evidence→4                          │
│ 14:01:55  event   tx 7c1d  seen (unconfirmed)                                    │
│ reconnects 0 · gaps 0 · last ok 14:03:10                                         │
│ [space] pause follow   [x] stop (cancels)   [enter] event detail                 │
└───────────────────────────────────────────────────────────────────────────────────┘
```
(The `●LIVE` marker shows only while `MonitorWallet` is actually looping; historical sessions show `●ENDED`.)

### Reports
```
┌ Reports ──────────────────────────────┬ Preview  report-5c2 ────────────────────┐
│  id         subject   fmt     when     │ subject  bc1q…k3    schema report-v1    │
│  report-5c2 bc1q…k3   bundle  14:05    │ snapshot sha256 7f3a…  (deterministic)  │
│  report-4a1 3F2a…9x   md      11-01    │ sections: summary · risk · evidence ·   │
│                                        │           patterns · graph · timeline   │
│                                        │ [b] build  [e] export bundle            │
│                                        │ [v] verify bundle   [t] open timeline   │
└────────────────────────────────────────┴──────────────────────────────────────────┘
```

Compact-breakpoint note: each two-pane wireframe collapses to the list pane with `Tab` toggling to the detail pane; the SideNav becomes a one-column key rail (`1 2 3 4 6 8 a o r`).

---

## 15. GeoIP / ASN Source & Licensing (DECIDED — documented)

Decision (locked by the orchestrator; documented and justified here):

- **Dataset: DB-IP "IP to Country Lite" + "IP to ASN Lite"**, format MMDB (`.mmdb`), license **CC BY 4.0** (free, redistributable with attribution). Chosen over MaxMind GeoLite2 because GeoLite2's license forbids redistribution without an account; BCTX must stay installable offline without per-user accounts. GeoLite2 is NOT defaulted and NOT bundled; if a user supplies their own GeoLite2 file via `bctx geo install`, the reader accepts it (same MMDB format) but BCTX ships nothing MaxMind-licensed.
- **Reader: `github.com/oschwald/maxminddb-golang` v1.13.1** — pure-Go, ISC license (permissive), reads DB-IP `.mmdb` directly, no cgo, no network. Chosen over a hand-rolled in-repo MMDB reader to avoid re-implementing a binary-format parser (correctness risk) for no benefit; the library is tiny, pure-Go, and offline. Pinned to an exact version.
- **The DB is NEVER embedded in source, NEVER fetched during analysis/graph/monitor/report/map rendering.** It is installed out-of-band via the CLI, checksummed, versioned, and queried fully offline from `~/.bctx/geoip/`.

`tui/geoip` package (NETWORK-FORBIDDEN — contains no `net/*` import; this is asserted by extending the offline-boundary guard test to cover `tui/geoip`):
```go
type DB interface {
  LookupCountry(ip netip.Addr) (CountryResult, error) // ISO code + name
  LookupASN(ip netip.Addr) (ASNResult, error)         // ASN number + org
  Status() RegistryEntry                               // from local registry
  Close() error
}
```
Opens the installed `.mmdb` read-only via `maxminddb.Open`; never downloads.

Local metadata registry (`~/.bctx/geoip/registry.json`, no secrets):
```
{ db_name, db_type ("country"|"asn"), source ("DB-IP IP-to-Country Lite"),
  source_url, version ("2024.11"), license ("CC BY 4.0"),
  downloaded_at, installed_at, sha256, record_count }
```

CLI (`cli/commands/geo.go`): `bctx geo status` (print registry + install path), `bctx geo inspect <ip>` (country + ASN for one IP, fully offline), `bctx geo install <file>` (copy user-supplied `.mmdb` into `~/.bctx/geoip/`, compute sha256, write registry), `bctx geo verify <file>` (recompute sha256 and compare to registry). Install procedure (documented in Help/Settings): the user downloads the free DB-IP MMDB from db-ip.com, then runs `bctx geo install <path>`; updates are the same command with a newer file (version/sha256/dates are rewritten). BCTX performs no download step itself.

Missing-DB behavior (never crash, never fake): any geoip-dependent view shows `GEOIP DATABASE NOT INSTALLED — install with: bctx geo install <file>` and degrades to stored `Country`/`ASN` fields only (§13).

Source/version/license/sha256/install-path/update table (filled at install time, surfaced by `bctx geo status` and the Settings screen):

| db_name | type | source | version | license | install path | sha256 |
|---|---|---|---|---|---|---|
| dbip-country-lite | country | DB-IP IP-to-Country Lite (db-ip.com) | 2024.11 | CC BY 4.0 | `~/.bctx/geoip/dbip-country-lite.mmdb` | (recorded at install) |
| dbip-asn-lite | asn | DB-IP IP-to-ASN Lite (db-ip.com) | 2024.11 | CC BY 4.0 | `~/.bctx/geoip/dbip-asn-lite.mmdb` | (recorded at install) |

---

## 16. World Map Geometry Source (DECIDED — documented)

Decision: **Natural Earth (public domain)**, reduced to a low-resolution coastline + country-boundary set, installed as a **local asset under `~/.bctx/mapdata/`** with the **same install/verify/version lifecycle as the GeoIP asset (§15)** — not downloaded at startup, not fetched at runtime. The map asset is deliberately **symmetric with GeoIP**: a build-time generator produces it, a registry records its provenance + sha256, and a CLI verb set verifies it. (This resolves the earlier §16 ambiguity between "installed + registry + update command" and "shipped alongside the binary": the asset is **installed under `~/.bctx/mapdata/`**; the release *may* bundle a copy of `world-110m.asset` in the distribution tarball as a convenience, but it is still placed into `~/.bctx/mapdata/` and registered there by `bctx map install` — there is no second, un-registered code path that reads an asset from anywhere else.)

Source and build:
- Source: Natural Earth 1:110m "admin 0 countries" / coastline, public domain (no attribution required, redistribution unrestricted).
- **One-time offline build step** (`scripts/build_mapdata.sh`, which invokes the in-repo generator `tools/mapgen`; run at release time only, never at runtime, no `tui/*` import of it): reads a local Natural Earth source file and emits a compact newline/JSON polyline asset **plus a country-centroid table derived from the same admin-0 polygons** (representative point / polygon centroid per country), both written into **one asset `world-110m.asset` covered by a single sha256**. The output is deterministic (same source + same generator version → identical bytes → identical sha256), so the checksum is reproducible, not hand-waved. `tools/mapgen` prints the resulting sha256 for recording in the release manifest / the installer.
- The centroid table is **not an independently sourced dataset**: it is a deterministic derivative of the public-domain Natural Earth admin-0 polygons produced in the same step, so it inherits Natural Earth's public-domain license and is covered entirely by the single `world-110m.asset` sha256 — it carries no separate source/license/version/checksum (resolves iteration-2 Finding 3).

Install, registry, verify (mirrors `tui/geoip`, §15):
- `tui/mapdata/registry.go` writes/reads `~/.bctx/mapdata/registry.json` with the same shape as the geoip registry (no secrets):
  ```
  { name ("world-110m"), source ("Natural Earth 1:110m admin-0 / coastline"),
    source_url, version ("natural-earth-110m"), license ("public domain"),
    installed_at, sha256, record_count (country/centroid count) }
  ```
- `tui/mapdata` reads the installed asset read-only (no network, no `net/*` import — NETWORK-FORBIDDEN, enforced by §17.1's dedicated guard) and returns coastline polylines + the derived country centroids. The **renderer lives in `tui/components/mapview.go`** (Braille/half-block rasterizer), cleanly in-repo — it is NOT a copy of MapSCII and has no dependency on mapscii.me, remote tiles, telnet, Node, or a browser at build or runtime.
- CLI (`cli/commands/map.go`), symmetric with `bctx geo …`:
  - `bctx map status` — print the registry (name/source/version/license/sha256/install path) or the honest "not installed" message; fully offline.
  - `bctx map verify` — recompute the installed asset's sha256 and compare to the registry; report OK / mismatch; fully offline. (`bctx map verify <file>` verifies a candidate file before install.)
  - `bctx map install <file>` — copy a `world-110m.asset` (produced by `scripts/build_mapdata.sh`) into `~/.bctx/mapdata/`, compute its sha256, and write the registry. Updates are the same command with a newer asset (sha256/version/`installed_at` rewritten). BCTX performs no download step itself.
- Missing/corrupt geometry asset (never crash, never fake, never fabricate a coastline): MapView degrades to a **label-only country list with density counts (no coastline, no plotted points)**, with the honest note `WORLD GEOMETRY ASSET NOT INSTALLED — install with: bctx map install <file>`; a sha256 mismatch on load is treated the same as missing (degrade + warn), it never renders a corrupt/partial coastline.

Geometry source/version/license/sha256/install-update table (one asset covers both the coastline and the derived centroid table):

| name | source | version | license | install path | contents | produced by | verify / update |
|---|---|---|---|---|---|---|---|
| world-110m | Natural Earth 1:110m admin-0 / coastline | natural-earth-110m | public domain | `~/.bctx/mapdata/world-110m.asset` | coastline polylines **+ derived country-centroid table** (representative point per admin-0 country, deterministic preprocess output; no independent source/license) | `scripts/build_mapdata.sh` → `tools/mapgen` (release-time, offline, reproducible sha256) | `bctx map verify`; update = `bctx map install <newer file>` (rewrites registry sha256/version) |

The centroid table deliberately has **no row of its own** in this table or in §15: it is not an independently obtained dataset, it is a deterministic derivative of the Natural Earth admin-0 polygons produced in the same offline build step, so the single `world-110m.asset` sha256 and its public-domain license cover it entirely.

---

## 17. Testability

- **Unit-testable (pure, no network, no TTY):** theme token resolution + degraded profile; breakpoint classification from a `WindowSizeMsg` size; keymap dispatch (focus-aware suppression); the risk-band mapping (score→bucket thresholds); registry completeness (every `Nav=true` screen has a key, help, factory); GraphView limit-badge formatting; map binning/clustering from a fixed observation slice; geoip registry read/write and sha256 verify (against a tiny fixture `.mmdb`); mapdata registry read/write and sha256 verify + the missing/corrupt-asset degrade (against a tiny fixture `world-110m.asset`); each component's `View(Frame)` returning non-overflowing output for the five acceptance sizes (golden-ish width assertions). Bubble Tea models are tested by feeding `tea.Msg`s to `Update` and asserting the returned model/Cmd — no terminal needed.
- **Integration-testable:** `app.Build(cfg)` wiring produces an Engine (service fields nil) + `*manager.Manager` + active-case `Repo` + `Cleanup`, matching `buildEngine`'s shape (§1.1); `app/services.go` constructors return orchestrator/graph/reporting/registry wired with the inputs from §1.3; launching with `--offline`/`--airgap` makes `cfg.AcquisitionAllowed()==false` so `app.AcquisitionAllowed(cfg)` blocks sync/monitor (§1.2); MODELS resolves via `app.ModelsDir(cfg)` so installed models are found regardless of CWD (§1.4); the TUI reads counts/alerts/analysis through the seams against a seeded fixture case DB; the non-TTY plain branch still prints a status summary.

### 17.1 Offline-boundary guard extension (dedicated, scoped — NOT a drop-in to `analysisDirs`)

The existing `tests/offline/boundary_test.go` has two kinds of rule, verified against source:
- `TestAnalysisPackagesHaveNoTransitiveNetworkDeps` runs `go list -deps` over `analysisDirs` (`storage, graph, ml, risk, evidence, investigation, ingestion, reporting, pkg, blockchain/normalizer, blockchain/parser, monitoring`) and forbids a transitive import of a prefix set that **includes `modulePath + "/acquisition"`**. `tui` is deliberately **not** in `analysisDirs`.
- `TestMonitoringAndReportingNoHTTPTransport` is a **narrower** rule that forbids only the transport set (`net/http`, `net/http/httptest`, `crypto/tls`, `golang.org/x/net/` transport) for packages that are legitimately allowed to compose `acquisition` (monitoring).

Because the design's Monitoring and Data screens legitimately call `app.AcquisitionAllowed` and construct providers, `tui` will transitively import `acquisition` (and `monitoring`, which composes `acquisition`). **Naively adding `tui` to `analysisDirs` would flag `tui → acquisition` and `tui → monitoring → acquisition` and fail the build for a legitimate reason.** The extension must therefore be a new, dedicated test modeled on `TestMonitoringAndReportingNoHTTPTransport`, not an edit to `analysisDirs`:

```go
// TestTUIPackagesHaveNoHTTPTransport — new, dedicated check.
// go list -deps over tui/..., tui/geoip/..., tui/mapdata/...
// FORBID ONLY the transport set:
//   net/http, net/http/httptest, crypto/tls, golang.org/x/net/ (transport)
// DO NOT forbid modulePath+"/acquisition" or modulePath+"/monitoring" for tui/... :
//   those edges are legitimate, network-gated, and already proven transport-free
//   by TestMonitoringAndReportingNoHTTPTransport.
//
// For the two STRICTLY-OFFLINE leaf packages tui/geoip and tui/mapdata, additionally
// FORBID modulePath+"/acquisition", modulePath+"/monitoring", and bare net dial
// patterns (net.Dial*, net.Listen*) — they must never reach acquisition or a socket.
```

This enforces the no-network property by test (not assertion) while preserving the legitimate `tui → acquisition/monitoring` composition that the Monitoring/Data screens require. Acceptance §19.2 asserts the dedicated check exists and passes.

- A design is testable because screens hold no logic: all behavior under test is either a pure helper or a `Update(msg)→(model,Cmd)` transition. If any screen needs a live socket or SQL to test, that is a design error and the work routes back through the `app`/service seam.

---

## 18. Reference-Image Guidance

Phase 8 adopts the reference image's **interaction principles only**: dense multi-panel information, strong visual hierarchy, a persistent navigation rail, keyboard-first workflow, graph visualization, a world map, activity/alert streams, risk indicators, context-aware detail panes, and a persistent status bar. BCTX's **identity is unique and deliberately different**: a cyan/teal dark forensic-workstation palette (not the reference's branding/colors), BCTX's own wordmark and labels, its own layout, and its own screen set driven by the real service inventory. No name, logo, color scheme, exact layout, label text, or product identity is copied from the reference.

---

## 19. Acceptance Criteria (numbered, testable)

1. `go build ./...`, `go test ./...`, `go test -race ./...`, `go vet ./...`, `gofmt -s -l .` all stay GREEN; `scripts/phase6_offline_proof.sh` (5/0), `phase7_offline_e2e.sh` (32/0), `phase7_airgap_bundle.sh` (8/0) still pass.
2. The offline-boundary guard suite passes, including a new dedicated check `TestTUIPackagesHaveNoHTTPTransport` (§17.1, modeled on `TestMonitoringAndReportingNoHTTPTransport`) that asserts `tui/...`, `tui/geoip/...`, `tui/mapdata/...` have no transitive `net/http`/`net/http/httptest`/`crypto/tls`/`golang.org/x/net` transport import — scoped so it does NOT flag the legitimate `tui → acquisition`/`tui → monitoring` edges, while additionally forbidding `acquisition`/`monitoring`/bare-net-dial for the strictly-offline `tui/geoip` and `tui/mapdata` leaf packages. The existing `analysisDirs` rule is left unchanged.
3. All 15 registry screens are reachable by their declared key; every `Nav=true` screen appears in the SideNav; `?` opens the generated HelpOverlay listing all bindings.
4. The TUI renders without panic or misrender at 80×24, 100×30, 120×40, 160×50, 200×60; below 80×24 it shows the resize message only.
5. No `lipgloss.Color` literal exists outside `tui/theme`; `grep` for hard-coded hex in `tui/components`/`tui/screens` returns nothing.
6. TopBar NETWORK/ACQUISITION/MODELS/GEOIP/CASE values come from live services (`NetworkStatus`, `cfg`, ml manifests, geoip registry, `Cases.Active`); none are hard-coded, and `"PENDING"` is gone.
7. Risk views always show numeric score + confidence + signals, and the delta when present; buckets use the documented thresholds and never alter the number.
8. Graph screen shows the engine's node/depth cap and marks truncated results; path mode highlights a `graph.Service.Path` result.
9. Geo Map uses only `Country`/`ASN` + offline geoip, labels points as observation metadata, and never implies ownership; with no DB it degrades honestly.
10. Any network-touching action (sync, start monitor) calls the offline gate first and is blocked with an honest message when offline/airgapped.
11. `bctx geo status|inspect|install|verify` work fully offline; a missing DB produces the documented install message, never a crash or fake data.
12. Long operations run as cancellable `tea.Cmd`s; `Esc`/screen-change/quit/new-query cancel in-flight work with no leaked goroutines; the monitor loop stops on `x`/quit.
13. Launching the TUI with `--offline` or `--airgap` yields `cfg.AcquisitionAllowed()==false`, so `app.AcquisitionAllowed(cfg)` returns `acquisition.ErrOfflineAcquisition` and sync/start-monitor are blocked (Finding #2). `app.Build` is tested to return a non-nil active `Repo` only when a case is open, with Engine service fields left nil. Case isolation is asserted through the single `App.ReopenCase(id)` contract (§1.1): after `ReopenCase` the old case's repo handle is closed and reads return only the new case's data — no `Cleanup()+Build()` rebuild is involved.
14. MODELS status and `AnalyzeWallet` resolve the models directory via `app.ModelsDir(cfg)` using the presence-checked resolution order of §1.4 (`dirHasModels`). Two cases are asserted: (a) **release** — `cfg.Models.Directory` points at a dir containing `anomaly/manifest.json`+`flow/manifest.json`, launched from a non-repo CWD → MODELS = LOADED, analysis succeeds; (b) **dev checkout** — models at `./models`, `~/.bctx/models` empty, CWD = repo root → `ModelsDir` returns `"models"` and MODELS = LOADED (the exact regression the review flagged). A third case where neither candidate has models asserts MODELS = MISSING honestly (iteration-1 Finding #4; iteration-2 Finding #1).
15. Before `bubbles` and `maxminddb-golang` are committed, the §0.1 gate passes: tags resolve against the locked bubbletea/lipgloss; `go list -deps github.com/oschwald/maxminddb-golang` shows no `net/*`/`crypto/tls`; the package builds under `CGO_ENABLED=0`; its license is ISC/permissive (Finding #6).
16. Graph node coloring: only the center/subject node can show a numeric risk (joined from `InvestigationResult.Risk`); a test asserts non-subject nodes render Neutral with no numeric risk because `graph.Service` leaves `GraphNode.Risk` zero (Finding #1).
17. TimelineView renders a glyph per `TimelineEvent.Kind` (tx/monitor_event/risk_delta/alert); a severity is shown only on `alert`-kind rows via an alert lookup by `ID`, and never on other kinds (Finding #5).
18. The map geometry asset has the same install/verify/version lifecycle as GeoIP: `bctx map status|verify|install` work fully offline; `tui/mapdata/registry.go` reads/writes `~/.bctx/mapdata/registry.json` and recomputes+compares the asset sha256; `scripts/build_mapdata.sh`/`tools/mapgen` deterministically emits `world-110m.asset` (same source+generator → identical sha256). A missing or sha256-mismatched asset produces the documented `WORLD GEOMETRY ASSET NOT INSTALLED` message and the label-only country-list degrade — never a crash and never a fabricated coastline (iteration-3 Finding #1).

---

## 20. Open Assumptions (flagged for design review)

- A new `app/` package is introduced to share `buildEngine`/`withActiveRepo`/`acquisitionAllowed` between CLI and TUI (audit risk #1 recommends exactly this). Alternative — exporting them in place from `cli/commands` — is rejected because it would make `tui` import `cli`, inverting the layering.
- `bubbles` v0.20.0 and `maxminddb-golang` v1.13.1 are the **proposed** pins (compatible with the locked bubbletea v1.3.4 / lipgloss v1.1.0 and the pure-Go/offline constraints). They are **not confirmed facts** until the §0.1 verification gate passes (tag resolution, no `net/*`/`crypto/tls` transitive import, `CGO_ENABLED=0` build, ISC/permissive license). If `maxminddb-golang` fails any check, fall back to the minimal in-repo MMDB reader (§15); if a tag does not resolve, bump to the nearest compatible patch within the same minor (no major bump).
- World geometry + GeoIP assets install under `~/.bctx/{mapdata,geoip}/` and are read only from there, each with its own registry + sha256 and its own CLI verb set (`bctx geo …` / `bctx map …`, §15/§16). A release may bundle a copy of the public-domain `world-110m.asset` in the distribution tarball for convenience, but it is still **placed into and registered under `~/.bctx/mapdata/` by `bctx map install`** — there is no alternate, un-registered read path, so "installed" is the single committed model and the renderer + no-network rule are unaffected either way.

---

## 21. Review Responses (design-review.json iteration 1)

Each finding is addressed (A), backlogged (B), or justified-ignored (J). All responses align with the original requirements (presentation-only layering, no fake data, offline invariants).

| # | Sev | Finding | Response |
|---|---|---|---|
| 1 | HIGH | GraphView colors nodes by `GraphNode.Risk`, which `graph.Service` never populates | **A (fix in-scope).** §9 now states only the center/subject node carries a risk value, joined at render time from `InvestigationResult.Risk.Score`; all non-subject nodes render Neutral with no number because the service leaves `Risk` zero. Graph wireframe caption updated. Populating `GraphNode.Risk` is called out as an out-of-scope backend change. Acceptance §19.16 asserts it. |
| 2 | HIGH | Gate specified as `app.AcquisitionAllowed(cfg)` "verbatim", but real gate is `acquisitionAllowed(cfg, gf)` | **A.** New §1.2 gives the explicit contract: the single-`cfg` gate is safe only because `loadConfig(gf)` folds `--offline`/`--airgap` into `cfg.Network.Mode` upstream and `AcquisitionAllowed()` is false for both modes; `app.Build` must receive the flag-applied `cfg`. "Verbatim" removed from §1 and §12 invariant 1. Acceptance §19.13 asserts the flag→block path. |
| 3 | MEDIUM | `app.Build(cfg)` under-specified vs real `buildEngine` (no case manager/cleanup, nil services) | **A.** New §1.1 defines `App{Engine, Cases, Repo, CaseID, Cleanup}` mirroring `buildEngine`'s return shape and states Engine service fields stay nil; new §1.3 defines `app/services.go` constructors (orchestrator with ModelsDir/FeatureSchemaSHA/CaseID/Weights, `graph.NewService`, `reporting.NewService`, `inference.NewRegistry`) with exact inputs. |
| 4 | MEDIUM | CWD-relative models dir breaks MODELS/analysis when launched outside repo root | **A.** New §1.4 defines `app.ModelsDir(cfg)` = `cfg.Models.Directory` with `repoModelsDir()` dev fallback; §8 MODELS row and all analysis use it. Acceptance §19.14 asserts MODELS reflects installed models regardless of CWD. |
| 5 | MEDIUM | TimelineView "severity glyphs" but `TimelineEvent` has no severity | **A.** §3 TimelineView now specifies one glyph per `Kind` (tx/monitor_event/risk_delta/alert); severity shown only on `alert` rows via alert-by-`ID` lookup. Acceptance §19.17 asserts it. |
| 6 | MEDIUM | `maxminddb-golang`/`bubbles` license/purity/offline unverifiable from repo | **A.** New §0.1 adds a dependency verification gate (tag resolution, `go list -deps` no `net/*`, `CGO_ENABLED=0` build, ISC/permissive license), extends the offline-boundary guard to cover `tui/geoip`, and makes dep approval conditional. §15 fallback to in-repo reader if checks fail. §20 assumption downgraded from fact to proposed-pending-gate. Acceptance §19.15 asserts it. |
| 7 | NIT | NetworkStatus constants are UPPERCASE; prose used lowercase | **A.** §4 and §8 now reference `state.Connected/Disconnected/Airgapped` and state the lowercase label is a display transform. |
| 8 | NIT | Header self-declares "APPROVED-FOR-IMPLEMENTATION" | **A.** Header changed to "REVISED — review iteration 2 … pending re-review". |
| 9 | NIT | Depth control implied everywhere, but orchestrator subgraph is fixed depth-2 | **A.** §9 now scopes depth control to the Graph screen's own `Subgraph` queries and notes the Wallet pane's embedded subgraph is the fixed depth-2 snapshot; expanding it means pressing `g` to open the Graph screen. |

No findings were backlogged or ignored; all nine are resolved in that revision.

---

## 22. Review Responses (design-review.json iteration 2)

Second-iteration review returned **3 MEDIUM + 2 NIT, 0 HIGH**. Each is addressed (A) here; none backlogged or ignored. All responses hold the original invariants (presentation-only layering, honest offline/status, no fake data).

| # | Sev | Finding | Response |
|---|---|---|---|
| 1 | MEDIUM | `app.ModelsDir(cfg)` never hits the dev fallback (configured dir always populated) and does NOT match the CLI; can reintroduce MODELS MISSING in a dev checkout | **A.** §1.4 rewritten with a **presence-checked resolution order** via `dirHasModels` (anomaly+flow manifest present): (1) configured dir only if it has models, (2) repo-local `./models` if present — the same path the CLI's `repoModelsDir()` returns, (3) else configured dir so MISSING is honest. The false "matches the CLI" claim is removed; the CLI/TUI divergence is documented as intentional (CLI keeps its own `repoModelsDir()`; optionally switchable). Acceptance §19.14 now asserts both a release case and a **dev-checkout case** (models at `./models`, `~/.bctx/models` empty, CWD = repo root → LOADED). |
| 2 | MEDIUM | Offline-guard extension to `tui`/`tui/geoip`/`tui/mapdata` asserted but not scoped; naive addition to `analysisDirs` flags legitimate `tui → acquisition/monitoring` edges | **A.** New §17.1 specifies a **dedicated** check `TestTUIPackagesHaveNoHTTPTransport` modeled on `TestMonitoringAndReportingNoHTTPTransport`: `go list -deps` over `tui/...`, `tui/geoip/...`, `tui/mapdata/...` forbidding **only** transport (`net/http`, `net/http/httptest`, `crypto/tls`, `golang.org/x/net` transport), explicitly NOT the `acquisition`/`monitoring` prefixes for `tui`; the two leaf packages additionally forbid `acquisition`/`monitoring`/bare net dial. `analysisDirs` is left unchanged. §0.1 check 3, §12 invariant 2, and §19.2 updated to match. |
| 3 | MEDIUM | Country-centroid lat/lon table is load-bearing for the map but has no named source/license/version/sha256 | **A.** §13 and §16 now specify the centroid table is **derived deterministically at preprocess time from the shipped Natural Earth admin-0 polygons** (representative point per country), emitted into the single `world-110m.asset`, inheriting Natural Earth's public-domain license and covered by that asset's single sha256 — so it needs no independent provenance row (explicitly stated). The no-centroid fallback (list-only bin, no plotted point) is specified. |
| 4 | NIT | 15 screens vs brief's ~14 not reconciled in one line | **A.** §2 opens with a sentence noting the delta: Entity split from Wallet (distinct seams) and Timeline added as a contextual `Nav=false` screen; both map to real seams, so it is refinement, not scope creep. |
| 5 | NIT | §1.4 inline comment "matches CLI" factually wrong | **A.** The comment is reworded to "repo-local `./models`, the same path the CLI's `repoModelsDir()` returns" and scoped to the fallback branch only; the primary branch no longer claims CLI parity. |

Verified-correct items the iteration-2 review confirmed (TimelineEvent has no severity; `graph.Service` leaves `GraphNode.Risk`/`Label` unpopulated; `AcquisitionAllowed()` false for both offline and airgap; real gate signature; UPPERCASE network constants; `buildEngine` return shape with nil service fields; orchestrator fixed depth-2; `MaxNodes=5000`; `runHome` TTY-vs-plain + `tea.WithContext`) are unchanged and remain the design's basis. The two deps (`bubbles`, `maxminddb-golang`) stay **proposed pins pending the §0.1 gate**, as the review agreed is the correct handling.

---

## 23. Review Responses (design-review.json iteration 3)

Third-iteration review returned **0 HIGH + 1 MEDIUM + 3 NIT**. All four are addressed (A) here; none backlogged or ignored. All responses hold the original invariants (presentation-only layering, honest offline/status, no fake data, GeoIP/map assets never fetched at runtime).

| # | Sev | Finding | Response |
|---|---|---|---|
| 1 | MEDIUM | Map geometry asset declared checksummed/versioned/installed but had no install/verify/produce mechanism (asymmetric with the fully-specified GeoIP path); §16 also contradicted itself ("installed + registry + update command" vs "shipped alongside the binary") | **A (preferred path chosen).** §16 rewritten to give the map asset the **same lifecycle as GeoIP (§15)**: a new `cli/commands/map.go` with `bctx map status|verify|install`; a new `tui/mapdata/registry.go` mirroring `tui/geoip/registry.go` (`~/.bctx/mapdata/registry.json` with `name/source/version/license/sha256/installed_at/record_count`, recompute+compare sha256); the one-time build tool is **named** — `scripts/build_mapdata.sh` invoking the in-repo generator `tools/mapgen` (release-time, offline, deterministic/reproducible sha256), both added to §1 package layout. The self-contradiction is resolved explicitly: the asset is **installed under `~/.bctx/mapdata/`** (a release may bundle a copy in the tarball, but it is still placed and registered there — no second un-registered read path). New acceptance **§19.18** asserts `bctx map status|verify|install` work offline, the registry/sha256 round-trips, the build tool is reproducible, and a missing/corrupt asset degrades to the label-only list with the documented message, never a crash or fabricated coastline. §17 adds the mapdata registry + degrade to the unit-test list. |
| 2 | NIT | §0 stack table stated `maxminddb-golang` "pure-Go, ISC license" as fact while §0.1/§20 correctly label it unverified-pending-gate | **A.** The §0 MMDB row now reads "expected pure-Go, expected ISC license — unverified until §0.1 gate passes" (the "expected" hedge applied consistently to purity and license), with a cross-ref to §15 and §20. |
| 3 | NIT | §13 map pipeline conflated "country/coords via geoip" when coordinates actually come from the mapdata centroid table | **A.** §13 pipeline reworded: `tui/geoip` resolves a **country ISO code** (never coordinates); that ISO code is mapped to a **plot coordinate via the `tui/mapdata` centroid table**, matching §16's package responsibilities. |
| 4 | NIT | Case-switch offered `Cleanup()+Build()` OR `App.ReopenCase(id)` without committing to one contract | **A.** §1.1 now commits to **one** contract: `App.ReopenCase(id)` (closes the old active-case repo, opens the new case's repo, resets `Repo`/`CaseID` in place; Engine/Cases preserved). `Cleanup()+Build()` is explicitly reserved for process teardown and is not the switch mechanism. §19.13's isolation assertion now targets `ReopenCase`. |

No findings were backlogged or ignored; all four are resolved in this revision. The MEDIUM was the only blocker (CHANGES_REQUESTED on 1 MEDIUM); the three NITs are folded in during the same pass. No source facts changed — this iteration adds a map-asset lifecycle symmetric with the already-approved GeoIP lifecycle and tightens three phrasings.
