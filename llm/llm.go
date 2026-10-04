// Package llm is the transport-free interface + deterministic summarizer for
// the OPTIONAL, off-by-default local-LLM context pane (design §E). It is a
// plain-language NARRATOR of already-computed evidence — it never computes or
// alters any risk/evidence/pattern value. The package imports only context and
// stdlib string/format helpers; it NEVER imports net/http and NEVER imports
// pkg/schema, so it is safe for BOTH tui and tui/screens to import (the
// air-gap gate asserts package llm carries no transport in its closure). The
// only network-capable addition lives in the sub-package llm/ollama, which is
// imported solely from cli/commands.
package llm

import "context"

// Summarizer turns an already-computed InvestigationResult projection into a
// plain-language SUMMARY string. It NEVER computes or alters any value: every
// fact it states is already present in the SummaryInput handed to it.
type Summarizer interface {
	Summarize(ctx context.Context, in SummaryInput) (string, error)
}

// SummaryInput is a read-only projection of values the services ALREADY
// produced. Every field is copied from a schema.InvestigationResult by
// screens.BuildSummaryInput; nothing here is computed by llm. It carries no raw
// secrets (the projection is built after redaction in the screens layer).
type SummaryInput struct {
	// Subject is result.Subject.
	Subject string
	// RiskScore is result.Risk.Score (0..100, from the risk engine).
	RiskScore int
	// RiskBand is DERIVED from RiskScore by the EXISTING presentation helper
	// riskBand(score) ("CRITICAL"/"HIGH"/"ELEVATED"/"LOW"), applied inside
	// package screens by BuildSummaryInput — NOT by llm and NOT by package tui.
	// schema.InvestigationResult has NO RiskBand field; the band is a
	// presentation mapping, never a new computation.
	RiskBand string
	// Evidence is the top-N result.Evidence[i].Description (already produced).
	Evidence []string
	// Patterns is result.Patterns[i].Type + Description (already produced).
	Patterns []string
	// Related is result.RelatedWallets.
	Related int
	// Txs is result.RelevantTxs.
	Txs int
}

// NewDeterministic returns the always-available, network-free Summarizer. It is
// the default and the fallback: it renders a plain-language summary purely from
// the SummaryInput, inventing nothing. It is used whenever the local LLM is
// disabled or unreachable so the pane is never empty and never fabricates.
func NewDeterministic() Summarizer { return deterministicSummarizer{} }
