# BCTX Phase 8 — TUI Architecture

This document records the layering, package boundaries, and import rules that
keep the Phase 8 TUI a thin presentation layer over the phases 0–7 backend. It
is grounded in the shipped code; the authoritative design is `docs/tui-design.md`
and the operational view is `docs/tui-runtime.md`.

---

## 1. The one dependency direction

```
CLI / TUI  (cli/commands, tui/*)
  ↓  calls
application / bootstrap seam  (app/*)   ← shared by CLI and TUI
  ↓  calls
existing BCTX services  (sdk, graph, orchestrator, reporting, monitoring,
                         acquisition, risk, ml/inference, cases)
  ↓
repository / domain  (storage/sqlite, pkg/schema)
```

The TUI MUST NOT contain SQL, HTTP/TLS/socket dialing, provider parsing,
normalization, feature calculation, ML algorithms, risk formulas, graph-building
or traversal algorithms, report-generation business logic, or GeoIP-download
logic (AGENTS §12). All of that lives behind the seams below. The TUI holds only
ephemeral presentation state and never owns canonical forensic state.

---

## 2. The shared `app/` seam

`app/` exists so the CLI and TUI never duplicate engine construction, the
offline gate, model-dir resolution, or the analysis-service constructors.

- `app/bootstrap.go` — `App{Engine, Cases, Repo, CaseID, Cleanup}`; `Build(cfg)`
  mirrors the legacy `buildEngine` (Engine service fields left nil; active-case
  `Repo` + `Cleanup` attached only when a case is open). `App.ReopenCase(id)` is
  the single committed case-switch contract: it opens the new case's repo, then
  closes the old one, guaranteeing case isolation.
- `app/gates.go` — `AcquisitionAllowed(cfg)` returns
  `acquisition.ErrOfflineAcquisition` when `!cfg.AcquisitionAllowed()`; it relies
  on `loadConfig` having folded `--offline`/`--airgap` into `cfg.Network.Mode`
  upstream.
- `app/services.go` — the per-call analysis-service constructors
  (`NewOrchestrator`, `NewGraph`, `NewReporting`, `NewRegistry`) with the exact
  inputs the CLI call sites use. Each screen builds a fresh, correctly
  case-scoped service per command and closes it when the command completes.
- `app/models.go` — `ModelsDir(cfg)` presence-checked resolution so a shipped
  binary finds installed models and a dev checkout finds `./models`.

The Engine's analysis fields (Graph/Features/ML/Detection/Risk/Evidence/Reports)
are intentionally nil; screens obtain services from `app/services.go`, never
from `Engine.*`.

---

## 3. Package layout

```
tui/
  app.go, router.go, keys.go, messages.go, commands.go, adapter.go,
  layout.go, program.go, stub.go      # shell, registry, keymap, plumbing
  redact.go                           # package-tui re-export of the redact leaf
  redact/redact.go                    # pure secret-redaction leaf (no imports of tui)
  theme/{tokens,theme,styles}.go      # the ONLY place a lipgloss color literal lives
  components/*.go                     # reusable widgets (TopBar, SideNav, tables,
                                      #   graphview, mapview, timeline, …)
  screens/*.go                        # one Bubble Tea sub-model per registry screen
  geoip/*.go                          # strictly-offline GeoIP .mmdb reader + registry
  mapdata/*.go                        # strictly-offline world-geometry asset + registry
tools/mapgen/                         # release-time map asset generator (NOT imported by tui)
cli/commands/{home,tui,geo,map}.go    # CLI entry points that drive the TUI + local assets
```

### Avoiding the import cycle

The registry factories in `tui/router.go` build screens, so `tui` imports
`tui/screens`. The reverse would be a cycle. Therefore:

- `screens` defines its own `screens.Model` contract and a tiny `screens.Subject`
  value type; `tui.screenAdapter` bridges a `screens.Model` to `tui.Screen`.
- Shared pure helpers that both packages need (secret redaction) live in the
  leaf package `tui/redact`, which neither imports `tui` nor `screens`.
- The theme is a leaf (`tui/theme`) consumed by components, screens, and the
  shell; no component or screen constructs a color itself.

---

## 4. The offline boundary, enforced by test

`tests/offline/boundary_test.go` holds the architectural guards. The Phase 8
addition is `TestTUIPackagesHaveNoHTTPTransport`, modeled on
`TestMonitoringAndReportingNoHTTPTransport` — **not** added to the `analysisDirs`
rule, which would wrongly flag the legitimate `tui → acquisition` and
`tui → monitoring` edges that the Data and Monitoring screens require.

The check:

- Over all of `tui/...`: forbids any transport import — `net/http`,
  `net/http/httptest`, `crypto/tls`, `golang.org/x/net/` (transport).
- For the two strictly-offline leaves `tui/geoip` and `tui/mapdata`:
  additionally forbids reaching `acquisition`/`monitoring` and any bare
  `net.Dial*`/`net.Listen*` call pattern, with a source backstop that runs even
  when the Go toolchain is absent.

The general `tui → acquisition/monitoring` composition stays allowed because the
Data/Monitoring screens gate it through `app.AcquisitionAllowed` and the TUI
imports no HTTP provider. The existing `analysisDirs` rule is unchanged.

### Preserved boundary decision

The TUI does not import `blockchain/acquisition/explorer` (the HTTP provider).
In-TUI live monitoring would require injecting a provider through a seam that
keeps the HTTP client OUT of the `tui/*` import closure, or the guard fails.
Today an allowed sync/monitor is launched via the CLI.

---

## 5. Interaction state on the Root

The interaction model (see `docs/tui-runtime.md` §2a) is a small amount of
state on the `Root` and nothing more:

- `focus FocusRegion` — `FocusNav` / `FocusBody` / `FocusModal`; the single
  owner of "who has the keyboard." Default `FocusNav`.
- `navCursor int` — the SideNav selection index into `NavScreens()`, driven only
  while `FocusNav`.
- `priorFocus FocusRegion` — remembered when an overlay opens, restored on close.
- `navForced bool` — the compact sidebar toggle (`b`).
- `overlay overlayState{palette, search, kind}` — the two Root-owned overlays
  (command palette, global search box) and which (if any) is open.
- `nowFn func() time.Time` — a clock seam read by `topBar()` instead of
  `time.Now` directly, so the layout golden tests freeze the clock. It is
  test-only wiring and does not change runtime behavior.

`handleKey` is the focus-aware router (the A.5 table); it is pure state
transition over this in-memory state, so the only failure mode is "key maps to
nothing," which the documented `default` no-op covers. Navigation failures are
honest no-ops (`navTo`/`jump:` ignore an unregistered id), never crashes.

### Geometry contract: `SplitH` / `PanelInner`

The box-break class of bug (panels wider than their column) is prevented by a
single width budget in the leaf package `tui/components`:

- `SplitH(total, n, gap) []int` returns `n` column widths that sum to
  `total - (n-1)*gap` exactly, with a `minCol` floor; when the floor cannot be
  met it returns *fewer* columns rather than overflowing, and the integer
  remainder goes to the last column. `sum(cols) + (n-1)*gap == total` always.
- `PanelInner(f Frame) (w, h)` returns the drawable inner size after the panel
  chrome (border + theme padding), each clamped to `>= 0`.

`cockpit.go`'s `Row2`/`TileRow` are built on `SplitH` with an explicit gap-wide
spacer rendered *between* columns, so the rendered width equals the budget. The
Home dashboard sizes every panel from the width `SplitH` returns rather than from
local ratio literals. `TestSplitHSumsToWidth` is the property test; the
five-size layout goldens assert no rendered line ever exceeds the width.

### The `screens → Root` signal set

Screens are `tui`-independent (§3). They request cross-screen work by returning
small value messages the `Root` recognizes and the adapter forwards unchanged:

- `screens.NavSearch{Kind, ID}` — "navigate to the detail screen for this
  subject." The `Root` maps `Kind` → screen id (`tx`→Transaction, `ip`→Network,
  else Wallet) and calls `navTo` with the subject. Emitted by Search row-select,
  the Dashboard open-subject submit, and the Geo Map `g` ("go") cross-link.
- `components.CommandChosen{ID}` / `components.SearchSubmitted{Query}` — the two
  overlay results, consumed by the `Root` **only while the matching overlay is
  open** (guarded by `overlay.kind`) so an in-screen box's `SearchSubmitted`
  never collides with the global one.

This keeps the `Root` the sole owner of navigation and overlay lifecycle while
screens stay a leaf that never imports `tui`.

### The optional-LLM interface boundary

The optional local-LLM narrator (see `docs/llm-optional.md`) is threaded as an
interface, never a transport:

- Package `llm` declares `Summarizer` + `SummaryInput` + `NewDeterministic()`
  and is transport-free (its closure has no `net/http`). **Both** `tui` and
  `tui/screens` import only `llm`.
- Package `llm/ollama` is the single new `net/http` importer; it is imported
  **only** from `cli/commands` (the composition root), which builds
  `ollama.New(cfg)` when `[llm] enabled` else `llm.NewDeterministic()` and injects
  it via `tui.NewRoot(ctx, a, tui.WithSummarizer(sum))`.
- The summarizer flows `Root → AppCtx.Summarizer → ScreenCtx.Summarizer`
  (copied in `tui/adapter.go:toScreenCtx`); the screen builds the projection and
  dispatches `explainCmd` only on the explicit `L`/`:explain` action.

So `tui` importing only `llm` is what keeps `net/http` out of the TUI closure
and the air-gap gate green. `tests/offline/boundary_test.go` confines
`llm/ollama` to the command roots and asserts `tui/...` (including
`tui/screens`), `reporting`, and every analysis dir reach neither `llm/ollama`
nor `net/http`.

---

## 6. Honesty invariants held by construction

1. Acquisition is the only network boundary; Data/Monitoring gate before any
   provider construction.
2. `tui`/`tui/geoip`/`tui/mapdata` dial nothing (guard above); the optional LLM
   transport is confined to `llm/ollama`, reachable only from `cli/commands`.
3. Analysis, graph, monitor-history, report, and map rendering read only the
   local repository and local assets; they never implicitly fetch.
4. Case isolation is absolute: all reads go through the active case's `Repo`;
   switching cases goes through `App.ReopenCase`.
5. No secret is ever rendered (`tui/redact` guard + the redaction test).
6. The optional LLM is off by default and is never a forensic source: it only
   rephrases a deterministic summary of already-computed facts and degrades to
   that deterministic text (with an honest note) when disabled or unreachable.
