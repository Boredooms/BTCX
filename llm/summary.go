package llm

import (
	"context"
	"fmt"
	"strings"
)

// summary.go is the deterministic, LLM-free summarizer (design §E.2). It is a
// pure string template over SummaryInput — no network, no randomness, no
// hidden computation. Every token it emits derives from a field of the input,
// so it invents nothing. It is the value shown when the LLM is disabled or
// unreachable, and the prompt context the ollama adapter rephrases.

// deterministicSummarizer renders a SummaryInput as plain-language prose using
// only string templating. It holds no state and opens no socket.
type deterministicSummarizer struct{}

// Summarize renders the input deterministically. The ctx is accepted to satisfy
// the Summarizer interface and is honored for cancellation, but the work is
// pure and immediate so it never blocks.
func (deterministicSummarizer) Summarize(ctx context.Context, in SummaryInput) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return renderDeterministic(in), nil
}

// renderDeterministic is the pure template. It is exported-free and reused by
// the ollama adapter as the prompt context, so the rephrased summary is always
// grounded in exactly these already-computed facts.
func renderDeterministic(in SummaryInput) string {
	var b strings.Builder

	subject := in.Subject
	if strings.TrimSpace(subject) == "" {
		subject = "(unspecified subject)"
	}
	band := in.RiskBand
	if strings.TrimSpace(band) == "" {
		band = "UNSCORED"
	}

	// Headline: band + numeric score (both already computed by the risk engine).
	fmt.Fprintf(&b, "Subject %s carries a %s risk band at %d/100.", subject, band, in.RiskScore)

	// Corpus footprint (already-counted related wallets + relevant txs).
	fmt.Fprintf(&b, " The subject touches %d related %s across %d relevant %s.",
		in.Related, plural(in.Related, "wallet", "wallets"),
		in.Txs, plural(in.Txs, "transaction", "transactions"))

	// Top detected patterns (verbatim labels the detectors already produced).
	if len(in.Patterns) > 0 {
		b.WriteString(" Detected patterns: ")
		b.WriteString(strings.Join(in.Patterns, "; "))
		b.WriteString(".")
	} else {
		b.WriteString(" No patterns were detected.")
	}

	// Top evidence lines (verbatim descriptions the evidence engine produced).
	if len(in.Evidence) > 0 {
		b.WriteString(" Key evidence: ")
		b.WriteString(strings.Join(in.Evidence, "; "))
		b.WriteString(".")
	} else {
		b.WriteString(" No supporting evidence was recorded.")
	}

	return b.String()
}

// plural picks the singular or plural noun for a count.
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
