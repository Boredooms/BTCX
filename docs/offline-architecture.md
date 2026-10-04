# BCTX Offline Architecture

BCTX has one hard invariant:

```
ACQUISITION MAY USE THE NETWORK.
EVERYTHING AFTER ACQUISITION MUST WORK FULLY OFFLINE.
```

Acquisition is the only boundary where a socket may open. Once canonical data
lands in local SQLite, the entire investigation lifecycle — ingestion, graph,
features, ML, detection, risk, evidence, investigation, monitoring, reporting,
and every CLI analysis/offline command — runs without touching the network,
whether disconnected, air-gapped, or inside a dropped network namespace.

This document is the authoritative classification of which packages may reach
the network and how the boundary is mechanically enforced.

---

## 1. Package classification

### NETWORK-ALLOWED (may open sockets)

Only the acquisition surface and its provider adapters:

| Package                                   | Role                                               |
|-------------------------------------------|----------------------------------------------------|
| `acquisition`                             | Provider **interface** + the scripted/offline `FakeProvider`. Opens **no** socket itself; defines the `DataSource` contract. |
| `blockchain/acquisition/...` (`explorer`) | The real Esplora/Blockstream HTTP provider. The **only** component that opens a TCP/TLS socket at runtime. |
| provider adapters                         | Any future `DataSource` implementation that fetches over the wire — lives under the acquisition tree. |
| `network/state`                           | Connectivity prober. Uses a bounded `net.Dialer` TCP probe to classify connected / disconnected / air-gap. Not an analysis package; never called from analysis code. |

### NETWORK-FORBIDDEN (must never reach a network client)

```
schema (pkg/schema)   storage        ingestion       graph
features / ml          detection      risk            evidence
investigation          monitoring     reporting       blockchain/normalizer
blockchain/parser      CLI analysis / offline commands
```

- **Monitoring** stays network-free *itself*. The Phase-7 brief permits it to
  *compose* the acquisition interface ("Monitoring … may compose acquisition,
  but must not import HTTP clients"), so monitoring may import the
  `acquisition` interface package — but never `blockchain/acquisition` (the HTTP
  explorer) and never an HTTP client. The real network hop stays inside the
  provider the CLI injects.
- **Reporting** is completely network-free: it imports no acquisition package at
  all (not even the interface) and no HTTP client. A report is composed from a
  frozen snapshot + local repository reads. See `docs/reporting-runtime.md`.

---

## 2. The socket-vs-parsing distinction

The bare `net` package is **allowed** in analysis code, because `net` is also
the home of pure address/IP parsing helpers that open no socket:

- **Allowed:** `net.ParseIP`, `net.ParseCIDR`, `net.IP`. `blockchain/parser`
  legitimately parses IPs from network-observation rows, and the real edge
  `ingestion → parser → net` is offline-safe.
- **Forbidden:** the dial/listen call forms that actually open a socket —
  `net.Dial(`, `net.DialTimeout(`, `net.DialContext(`, `net.Dialer{`,
  `net.Listen(`, `net.ListenPacket(`, `net.ListenTCP(`, `net.ListenUDP(`.

So the guard forbids the *socket-opening calls*, not the `net` import.

---

## 3. How the boundary is enforced

`tests/offline/boundary_test.go` enforces the classification with a strong
dependency-closure check plus a grep backstop, so the rule holds both with and
without the Go toolchain available.

### 3a. Deps-closure check (authoritative)

`TestAnalysisPackagesHaveNoTransitiveNetworkDeps` shells
`go list -deps -json <module>/<dir>/...` for every forbidden root, decodes the
JSON stream, and asserts that **no** analysis package's transitive closure
contains a forbidden path. This sees through any level of indirection a source
grep would miss. It `t.Skip`s (never silently passes) when `go` is absent, and
it asserts that `reporting` was actually among the audited roots.

**Forbidden transitively:**

- exact: `net/http`, `net/http/httptest`, `crypto/tls`
- prefixes: `golang.org/x/net/`, `github.com/bctx/bctx/acquisition`,
  `github.com/bctx/bctx/blockchain/acquisition`

**Three deliberately-excluded transitive edges**, each kept honest by a narrower
assertion (not a loosening):

1. **`network/state`** — reachable from every analysis package only as a *type*
   edge: `sdk/engine.go` holds a `state.Prober` field and `NetworkStatus()`
   returns `state.Status`. Analysis code uses only `sdk.Repository` + the
   persistence Row types; it never constructs the Engine nor calls the prober.
   Physically cutting the edge would require refactoring package `sdk`. Kept
   honest by `TestNetworkStateOpensNoHTTP` (network/state pulls in no
   HTTP/WebSocket/TLS) **and** the direct-import grep (no analysis package may
   directly import `network/state`).
2. **`net/url`** — rides in via the stdlib `html/template` (URL-attribute
   sanitization, in-memory, no socket) used by the self-contained HTML renderer,
   and via the stdlib net resolver pulled by `network/state`. Both are
   offline-safe stdlib; a *direct* `net/url` import in analysis source is still a
   grep failure.
3. **`monitoring → github.com/bctx/bctx/acquisition`** — the brief-sanctioned
   composition edge. Allowed only for the `acquisitionComposerRoots`
   (`monitoring`), only for the acquisition *interface* package, and never for
   `blockchain/acquisition`. Enforced by `exemptEdge` and
   `TestMonitoringAndReportingNoHTTPTransport`, which also asserts reporting
   imports no acquisition at all.

### 3b. Grep backstop (runs even without `go`)

`TestAnalysisPackagesHaveNoDirectNetworkImports` walks every non-test `.go`
file under the analysis dirs and fails on:

- a direct quoted transport import — `"net/http"`, `"net/http/httptest"`,
  `"net/url"`, `"crypto/tls"`, `"golang.org/x/net/…"`,
  `"github.com/bctx/bctx/blockchain/acquisition…"`,
  `"github.com/bctx/bctx/acquisition…"`,
  `"github.com/bctx/bctx/network/state"`; or
- any of the socket-opening `net` call patterns listed in §2.

The only exemption is the acquisition-*interface* import inside a composer root
(`monitoring`); `blockchain/acquisition` and all transport imports are never
exempt. The grep set is intentionally **broader** than the transitive set
(it also forbids a direct `net/url` / `network/state` import) because a direct
import of those from analysis source is a genuine red flag even though their
*indirect* presence is benign.

### 3c. Supporting tests

- `TestNetworkStateOpensNoHTTP` — network/state's own closure has no
  HTTP/WS/TLS transport (its only socket use is the bounded TCP probe).
- `TestMonitoringAndReportingNoHTTPTransport` — monitoring and reporting reach
  no HTTP transport; reporting reaches no acquisition at all.
- `TestAcquisitionBoundaryExists` — the acquisition layer is present (the
  boundary exists to be guarded).

---

## 4. Runtime proof (not just static analysis)

Static guards prove *imports*; the Phase-6/7 scripts prove *runtime behavior*:

- `scripts/phase6_offline_proof.sh` — acquisition refuses under `--offline` /
  `--airgap` and under `unshare -rn`, while local analysis commands still work.
- `scripts/phase7_offline_e2e.sh` — the full post-acquisition chain (analyze →
  graph → neighbors → monitor → report generate all four formats → export →
  verify) runs under `unshare -rn`; HTML carries no external asset; PDF starts
  `%PDF-`; the persisted-snapshot hash is reproducible and report content is
  stable across re-runs.
- `scripts/phase7_airgap_bundle.sh` — `generate` + `export` + `verify` need only
  the binary + a local case DB + local ONNX models, with Go/Python/Node/Docker
  shadowed by failing stubs on PATH so a report-time exec of any toolchain would
  fail loudly.

When `unshare -rn` is unavailable the Phase-7 scripts fall back to `--airgap` +
`BCTX_OFFLINE` and print a clear note that the flag fallback was used instead of
a dropped network namespace. Every script exits non-zero on any failure.

---

## 5. Offline truthfulness

Per AGENTS.md §17, the UX never hides the connected/offline distinction. Report
and analysis output show NETWORK / ACQUISITION / LOCAL ANALYSIS separately;
reporting always reports ACQUISITION as "not performed (reporting never
acquires)". Stored historical data is never relabeled as "live" while
disconnected.
