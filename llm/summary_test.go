package llm

import (
	"context"
	"strings"
	"testing"
)

// fixedInput is a deterministic SummaryInput used by the golden + provenance
// assertions below. Every field value here must appear (or its derived token
// must appear) in the rendered summary — the summarizer invents nothing.
func fixedInput() SummaryInput {
	return SummaryInput{
		Subject:   "WFIXTURE",
		RiskScore: 72,
		RiskBand:  "HIGH (50-74)",
		Evidence:  []string{"rapid fan-out to 12 wallets", "reused change address"},
		Patterns:  []string{"peeling_chain: sequential small spends", "rapid_flow: burst of transfers"},
		Related:   5,
		Txs:       41,
	}
}

// TestDeterministicGolden pins the exact rendered string for a fixed input, so
// a drift in the template is caught. The summarizer is pure, so this is stable.
func TestDeterministicGolden(t *testing.T) {
	const want = "Subject WFIXTURE carries a HIGH (50-74) risk band at 72/100. " +
		"The subject touches 5 related wallets across 41 relevant transactions. " +
		"Detected patterns: peeling_chain: sequential small spends; rapid_flow: burst of transfers. " +
		"Key evidence: rapid fan-out to 12 wallets; reused change address."

	got, err := NewDeterministic().Summarize(context.Background(), fixedInput())
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != want {
		t.Fatalf("deterministic summary drift:\n got: %q\nwant: %q", got, want)
	}
}

// TestDeterministicInventsNothing asserts every input-derived token appears in
// the output and that the output introduces no fabricated numbers. It proves
// the summarizer is a narrator of provided facts, never a source of new ones.
func TestDeterministicInventsNothing(t *testing.T) {
	in := fixedInput()
	got, err := NewDeterministic().Summarize(context.Background(), in)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}

	// Each provided fact must be present verbatim.
	mustContain := []string{
		in.Subject,
		in.RiskBand,
		"72/100",
		"5 related",
		"41 relevant",
	}
	mustContain = append(mustContain, in.Evidence...)
	mustContain = append(mustContain, in.Patterns...)
	for _, tok := range mustContain {
		if !strings.Contains(got, tok) {
			t.Errorf("summary omitted provided fact %q:\n%s", tok, got)
		}
	}

	// Every number in the output must be derivable from the input: the supplied
	// counts (72, 5, 41) plus "100" (the fixed /100 denominator the template
	// states) plus any digit that already appears in a provided string field
	// (the band range "50-74", or evidence/pattern descriptions). No number may
	// be invented from nowhere.
	inputText := in.Subject + " " + in.RiskBand + " " + strings.Join(in.Evidence, " ") +
		" " + strings.Join(in.Patterns, " ")
	allowed := map[string]bool{"72": true, "100": true, "5": true, "41": true}
	for _, field := range digits(got) {
		if allowed[field] || strings.Contains(inputText, field) {
			continue
		}
		t.Errorf("summary invented a number %q not derived from the input:\n%s", field, got)
	}
}

// digits splits s into maximal runs of ASCII digits.
func digits(s string) []string {
	var out []string
	for _, field := range strings.FieldsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

// TestDeterministicEmptyIsHonest asserts the summarizer degrades honestly for an
// input with no patterns/evidence rather than fabricating any.
func TestDeterministicEmptyIsHonest(t *testing.T) {
	got, err := NewDeterministic().Summarize(context.Background(), SummaryInput{
		Subject: "WEMPTY", RiskScore: 10, RiskBand: "LOW (0-24)", Related: 0, Txs: 1,
	})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	for _, want := range []string{
		"No patterns were detected.",
		"No supporting evidence was recorded.",
		"0 related wallets",
		"1 relevant transaction",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("empty-input summary missing honest phrase %q:\n%s", want, got)
		}
	}
}

// TestDeterministicRespectsCancellation asserts a cancelled context short
// circuits rather than returning a fabricated summary.
func TestDeterministicRespectsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewDeterministic().Summarize(ctx, fixedInput()); err == nil {
		t.Fatal("expected a cancellation error from a cancelled context")
	}
}
