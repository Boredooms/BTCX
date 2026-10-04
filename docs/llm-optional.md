# BCTX Phase 8 — Optional Local-LLM Context Pane

BCTX can optionally show a plain-language **summary** of an investigation inside
the TUI. It is a convenience narrator, not an analyst: it only rephrases facts
the engine already computed, it is **off by default**, and it is **never a
forensic source**. This document records exactly what shipped.

Grounded in `llm/{llm,summary}.go`, `llm/ollama/client.go`,
`tui/screens/{explain,commands}.go`, `tui/screens/{wallet,alerts,reports}.go`,
`cli/commands/home.go`, `configs/config.go` (`LLMConfig`), and the boundary
guards in `tests/offline/boundary_test.go`.

---

## 1. What it is

The pane is a bordered panel titled **"LLM SUMMARY (optional, local)"** on the
Wallet, Alerts, and Reports screens. When invoked it shows a short paragraph
describing the subject's already-computed risk band/score, pattern types,
evidence descriptions, and related-wallet/tx counts — plus an honest provenance
line saying where that text came from.

It does **not** compute, re-rank, or alter any value. The risk score, the
patterns, and the evidence are produced by the BCTX engine; the pane only
narrates a read-only projection of them (AGENTS §11/§18).

---

## 2. Off by default

`[llm]` config (`configs.Config.LLM`, defaults in `configs.Default()`):

```toml
[llm]
enabled  = false                      # OFF by default
endpoint = "http://localhost:11434"   # local Ollama endpoint (only used when enabled)
model    = "gemma3:1b"                # local model name
```

With `enabled = false` (the default) BCTX constructs **no socket**: the
composition root builds the deterministic summarizer and the pane renders a
network-free summary. A local model is used only when you explicitly set
`enabled = true` and run a local Ollama endpoint yourself.

---

## 3. How you invoke it

The summary is produced **only** on an explicit action — never from a screen's
`Init`, `View`, or a tick:

- Press **`L`** on the Wallet, Alerts, or Reports screen (Body focus), or
- Run the command-palette verb **`:explain`** (the `Root` forwards it to the
  active screen exactly like the `L` key).

If no subject/result is loaded, the pane opens with the honest note **"open a
subject first"** and dispatches nothing (no summarizer call, no network). This
is a deliberate no-op, not an error.

---

## 4. Deterministic-first, honest degradation

Two summarizers implement one interface (`llm.Summarizer`):

- **`llm.NewDeterministic()`** — the default and the fallback. A pure template
  over the `SummaryInput` projection; it imports only `context` + stdlib string
  helpers, invents nothing (every token derives from the input), and never
  touches the network. The air-gap gate asserts package `llm` has no transport
  in its closure.
- **`llm/ollama.New(cfg)`** — used only when `enabled = true`. It POSTs the
  deterministic summary as prompt context to the local endpoint's
  `/api/generate` with a bounded (~3s), cancellable request and an explicit
  "summarize only the provided facts, invent nothing" instruction. **Any**
  failure — disabled, unreachable, timeout, non-200, empty — degrades to the
  deterministic text plus an honest note, so the pane is never empty and never
  fabricates.

The provenance line read by the analyst is one of:

| State | Note |
|---|---|
| LLM disabled (default) | `local LLM disabled — deterministic summary (enable in config)` |
| enabled but not reachable | `local LLM not reachable — deterministic summary` |
| enabled and a local model answered | `summary generated locally by <model>; risk/evidence computed by BCTX` |

The last line is deliberate: even when a model rephrases the text, it states
plainly that the risk and evidence were computed by BCTX, not by the model.

---

## 5. Never a forensic source

The pane is a narrator with hard limits:

- It only rephrases a projection of values the engine already produced
  (`SummaryInput`: subject, risk score, derived risk band, top evidence
  descriptions, pattern type+description, related-wallet and tx counts).
- It computes nothing. The risk band is the existing presentation mapping
  `riskBand(score)` applied in `package screens`, not a new computation and not
  the model's opinion.
- Its output is never persisted as evidence and never feeds a score. It is a
  reading aid on top of the real detail panel, which always remains visible.

This keeps the pane consistent with AGENTS §18 (evidence truthfulness): a
pattern is never upgraded to "confirmed," and a cluster relationship is never
upgraded to "same owner proven."

---

## 6. The package boundary and the air-gap guarantee

The transport is physically isolated so enabling the pane cannot weaken the
air-gap:

- `llm` (interface + deterministic summarizer) is **transport-free** and is the
  **only** llm package `tui` and `tui/screens` import.
- `llm/ollama` is the **single** new `net/http` importer. It is imported **only**
  from `cli/commands` (the composition root `launchTUI`), which chooses
  `ollama.New(cfg)` vs `llm.NewDeterministic()` and injects it via
  `tui.NewRoot(ctx, a, tui.WithSummarizer(sum))`. Package `app` never imports it.

`tests/offline/boundary_test.go` enforces this: `TestLLMOllamaIsOnlyNewTransport`
(package `llm`'s closure is transport-free), `TestLLMTransportConfinedToLLMOllama`
(no `tui/...`, `reporting`, or analysis-dir package reaches `llm/ollama` or
`net/http`), a reachability assertion that `llm/ollama` is imported only from the
enumerated command roots, and a direct-import grep backstop. So with the LLM OFF
the air-gap gate `scripts/phase8_airgap_tui.sh` stays green at its baseline count,
and even with it ON nothing in the TUI closure can dial.
