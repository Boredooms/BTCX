# BCTX Phase 8 — Investigator TUI Runtime

The TUI runtime is the keyboard-first investigator terminal that turns the
phases 0–7 backend into a single workstation. It is **presentation and
interaction only**: every value it shows comes from a live service through the
shared `app/` seam, and it introduces no new analysis, no SQL, and no network.
It is the companion of the CLI — both front ends build the engine through one
wiring path (`app.Build`) so they can never diverge.

This document is grounded in the shipped code (`tui/*`, `app/*`,
`cli/commands/{home,tui,geo,map}.go`). For the authoritative design and
acceptance set see `docs/tui-design.md`; for the layering rules see
`docs/tui-architecture.md`; for the local-asset backends see
`docs/geoip-runtime.md` and `docs/map-runtime.md`.

---

## 1. What launches the TUI

`bctx` with no subcommand, and the explicit `bctx tui`, both call
`cli/commands/home.go:launchTUI`:

1. `loadConfig(gf)` folds `--offline`/`--airgap` into `cfg.Network.Mode` **before**
   the engine is built (so the offline gate downstream reads the flag-applied
   config).
2. `app.Build(cfg)` wires the Engine (service fields left nil), the Cases
   manager, and — when a case is open — the active-case repository plus a
   `Cleanup`.
3. On a TTY, `tui.NewProgram(tui.NewRoot(ctx, a), tea.WithContext(ctx))` runs the
   Bubble Tea program. Off-TTY (pipes, CI, scripts) it prints the honest plain
   status summary (`printHomePlain`) instead — the TTY-vs-plain split from the
   original command is preserved.

The non-TTY summary reports NETWORK, the active case, the live corpus counts
(`Repo.Counts`), MODELS (resolved from the inference registry via
`screens.ModelsStatus`, never a hard-coded "PENDING"), and the graph state.

---

## 2. Top-level state machine

`tui/app.go` holds the `Root`, the only owner of GLOBAL state (active case,
selected subject, notifications, and the live network/acquisition/model/geoip
status). Everything else is ephemeral and lives in the active screen.

- `Root.Update` is the reducer: global keys first (focus-aware), then window
  size and nav/notification messages, then the active screen gets a turn.
- `Root.navTo` / `navBack` push/pop a screen stack and build the target screen
  from the registry (`tui/router.go`), the single source of screen identity,
  key, help, and factory.
- `Root.refreshStatus` recomputes the status fields from live services on
  startup, on each 1s tick, and after every nav. Nothing is hard-coded; offline
  and airgap short-circuit with no socket.
- `Root.View` composes the AppShell (TopBar + SideNav + workspace + context
  bar) around the active screen's body, applying the responsive layout and the
  80×24 too-small floor.

The 16-row registry = 14 `Nav=true` screens (Dashboard, Search, Wallet,
Transaction, Entity, Graph, Network, Geo Map, Detection, Alerts, Monitoring,
Data, Reports, Settings) plus two contextual `Nav=false` screens (Help,
Timeline). Each screen implements `screens.Model`
(`Init/Update/View(Frame)/ShortHelp`) and is bridged to `tui.Screen` by a thin
adapter so the `screens` package never imports `tui`.

---

## 2a. Focus model (Nav / Body / Modal)

Interaction is driven by one small state machine on the `Root`: a `FocusRegion`
that says which region owns the keyboard.

- **FocusNav** (startup default) — the SideNav selection cursor is live, so the
  sidebar is immediately drivable.
- **FocusBody** — keys are forwarded to the active screen's `Update`, so each
  screen's own keymap (Graph/Map/Table/Monitor) finally receives input.
- **FocusModal** — an overlay (command palette or global search) owns the
  keyboard; the body is dimmed behind the centered overlay.

`Tab` / `Shift+Tab` are the single, global Nav↔Body focus cycle, consumed by the
`Root` **before any screen sees them** — screens never consume `Tab`. The cycle
never gets stuck: from Nav it goes to Body, from Body back to Nav. Sub-focus
inside a rich screen (e.g. the Search box vs its results table) is driven by the
keys that screen already owns, not by `Tab`. When a screen reports an in-screen
input is focused (its `Focused()` returns true), the `Root` treats the body like
a modal for suppression purposes so single-letter nav jumps don't fire while you
type.

### A.5 — the complete global-key routing table

`handleKey` routes every token `GlobalKeyMap.Dispatch` can emit — the closed set
`up, down, enter, esc, tab, shift+tab, search, palette, help, quit, jump:<key>,
"" (unmatched)` — to exactly one destination per focus region. Precedence is
overlay first, then focus region.

| Key(s) | FocusNav | FocusBody | FocusModal |
|---|---|---|---|
| `↑`/`k`, `↓`/`j` | move SideNav cursor | forward to active screen | forward to overlay |
| `Enter` | open the screen under the cursor | forward to active screen | overlay submit |
| `Tab` / `Shift+Tab` | focus → Body | focus → Nav | overlay field cycle |
| `/` (search) | open search overlay | open search overlay | ignored (already open) |
| `:` / `Ctrl+P` (palette) | open command palette | open command palette | ignored |
| `?` (help) | open Help screen | open Help screen | ignored |
| `q` / `Ctrl+C` (quit) | quit | quit | `Ctrl+C` always quits |
| `Esc` | pop the nav stack (back) | focus → Nav | close overlay, restore prior focus |
| `1`–`9`,`a`,`o`,`d`,`r`,`s`,`t` (jump) | jump to that registry screen + focus Nav | same (jumps work from Body too) | ignored while typing |
| `b` | toggle the compact sidebar rail | toggle the compact sidebar rail | — |
| any other key | documented inert no-op | forwarded raw to the active screen | forwarded raw to the overlay |

The guarantee is precise: *every token Dispatch can emit — including the empty
token — has exactly one defined destination in each region, and the only no-ops
are deliberate and documented* (an unmatched key in Nav/Modal, and a cursor move
at a list boundary). `TestEveryGlobalTokenRouted` enumerates the token set
directly from `Dispatch`'s case labels so the test cannot drift from the real
set; `TestNoKeyCrashes` feeds a broad key sweep across several screens in both
Nav and Body and asserts no panic and a non-empty frame. The committed
interaction gate `scripts/phase8_tui_interactive.sh` replays a scripted key
sequence through the real `Root.Update` and asserts these transitions plus the
hard no-overflow invariant after every keypress.

### Sidebar toggle (`b`)

`b` flips `Root.navForced`. In the Compact breakpoint the SideNav is normally
hidden; `navForced` forces a narrow key-rail so the sidebar stays reachable on a
small terminal. It is a pure render switch handled in the routing table (never a
silent key); the reserved rail width and the rendered rail width agree, so
forcing it on never overflows.

### Overlay lifecycle

Two Root-owned overlays capture the keyboard as `FocusModal` and render centered
over the body via the AppShell:

- **Command palette** (`:` / `Ctrl+P`) — a filterable list built from the
  registry (`nav:<id>` entries) plus a fixed verb set (`:search`, `:help`,
  `:quit`, `:open <id>`, `:explain`). Choosing an entry emits
  `CommandChosen{ID}`, which the `Root` consumes while the palette is open: it
  closes the overlay, restores the prior focus, then routes the id — `nav:<id>`
  navigates, `:open <id>` classifies-and-navigates, `:search` opens the search
  overlay, `:help`/`:quit` do the obvious, `:explain` forwards to the active
  screen's optional LLM pane (see `docs/llm-optional.md`), and an unknown id is
  ignored (no crash).
- **Global search** (`/`) — a text box; on submit it emits `SearchSubmitted`,
  which the `Root` consumes *only while the overlay is open*: it closes the
  overlay, restores focus, navigates to the Search screen, and seeds the query so
  the in-screen box opens pre-filled and already running its bounded lookup. A
  `SearchSubmitted` arriving while no overlay is open is unambiguously the
  in-screen Search box and flows to that screen normally.

`Esc` closes any open overlay and restores the focus region that was active when
it opened.

### Open-subject input flow

The Dashboard carries an "open subject" input (focused with `i`): type a wallet
address, txid, IP, or entity id and submit. The id is classified by an ordered
rule set (`ClassifySubject`): a 64-hex string → transaction, a parseable IP →
network, a base58/bech32 address → wallet; anything else is rejected with an
honest inline note and does **not** navigate. There is no default-to-wallet
guess. A structurally-valid id for data that isn't in the case still navigates
and lets the destination screen show its honest "not found" state. The same
classifier backs the palette's `:open <id>` verb.

---

## 3. Responsive layout

`tui/layout.go` classifies the terminal width into four breakpoints — compact
(<100), standard (100–159), wide (160–199), ultrawide (≥200) — and computes the
region geometry (TopBar, SideNav, workspace, context bar). The SideNav width and
split panes reflow per breakpoint; below the 80×24 floor the Root renders only a
resize message. Every component clamps its own output to its `Frame` so nothing
overflows at the five acceptance sizes (80×24, 100×30, 120×40, 160×50, 200×60).

---

## 4. Honest status (AGENTS §16/§17)

The TUI never fabricates connected or live state:

- NETWORK is read from `Engine.NetworkStatus` (UPPERCASE `CONNECTED` /
  `DISCONNECTED` / `AIRGAPPED`); offline/airgap never open a probe socket.
- ACQUISITION is derived from the config mode + the offline gate
  (`ENABLED`/`PAUSED`/`OFFLINE`/`AIRGAPPED`).
- MODELS is resolved from the ML inference registry
  (`LOADED`/`MISSING`/`SCHEMA-MISMATCH`).
- GEOIP is read from the local geoip registry.
- The Monitoring screen is the only place that may say LIVE, and only while a
  `MonitorWallet` loop is actually running; replayed history is marked `●ENDED`.

The two network-touching actions — Data (sync) and Monitoring (start session) —
call `app.AcquisitionAllowed(cfg)` **before** constructing any provider. When it
returns `acquisition.ErrOfflineAcquisition` the action is blocked with an honest
message and never attempted. The TUI deliberately imports no HTTP provider; an
allowed sync/monitor is guided to the CLI (`bctx sync wallet`,
`bctx monitor wallet`).

---

## 5. Secret redaction (AGENTS §21 / design §12 invariant 5)

The TUI shows no credentials, token-bearing endpoints, or raw secrets. By design
the Settings screen shows a provider NAME and public dataset provenance, not an
endpoint URL or API key. As a defensive guard, user-controlled config strings
that reach a view (the TopBar provider cell, the Settings provider row) pass
through the pure `tui/redact` helper, which scrubs URL userinfo credentials and
sensitive query parameters (`token`, `apikey`, `secret`, `password`, …) and
offers `redact.Field` to present a sensitive key as `(set)`/`(none)` instead of
its value. `tui/security_redaction_test.go` plants a secret in the config,
drives every screen plus the Root chrome, and asserts the secret never appears
in any rendered output.

---

## 6. Offline enforcement by test

`tui`, `tui/geoip`, and `tui/mapdata` dial nothing. The dedicated, scoped guard
`tests/offline/boundary_test.go:TestTUIPackagesHaveNoHTTPTransport` runs
`go list -deps` over `tui/...` and forbids any transport import
(`net/http`, `net/http/httptest`, `crypto/tls`, `golang.org/x/net/`); for the
strictly-offline leaves `tui/geoip` and `tui/mapdata` it additionally forbids
reaching `acquisition`/`monitoring` and any bare `net.Dial*`/`net.Listen*`
pattern. The general `tui → acquisition`/`tui → monitoring` edges (the Data and
Monitoring screens) remain allowed because they are network-gated and already
proven transport-free. See `docs/tui-architecture.md` §4.

---

## 7. Acceptance scripts

- `scripts/phase8_tui_e2e.sh` — seeds a fixture case, drives the plain TUI
  status path and the geo/map CLIs offline, asserts honest states + no fake
  data, and that sync/monitor are blocked. Reports PASS/FAIL counts.
- `scripts/phase8_airgap_tui.sh` — runs the TUI status path + geo/map CLIs under
  `unshare -rn` (with an `--airgap` fallback), asserting sync/monitor are blocked
  with an honest message and nothing dials.

Both must report `FAIL=0`, and the Phase 6/7 proofs
(`phase6_offline_proof.sh` 5/0, `phase7_offline_e2e.sh` 32/0,
`phase7_airgap_bundle.sh` 8/0) must still pass with no regression.
