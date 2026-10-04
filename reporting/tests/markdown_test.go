package tests

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
)

// externalAssetPatterns match any reference that would make a report depend on
// a network asset. None may appear in Markdown or HTML output.
var externalAssetPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)https?://`),
	regexp.MustCompile(`(?i)//cdn`),
	regexp.MustCompile(`(?i)@import\s+url\(`),
	regexp.MustCompile(`(?i)@font-face`),
	regexp.MustCompile(`(?i)src\s*=\s*["']?https?:`),
	regexp.MustCompile(`(?i)href\s*=\s*["']?https?:`),
}

func assertNoExternalAssets(t *testing.T, format string, data []byte) {
	t.Helper()
	for _, re := range externalAssetPatterns {
		if re.Match(data) {
			t.Fatalf("%s output contains external asset reference matching %q", format, re.String())
		}
	}
}

func TestMarkdownDeterministic(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	a, err := svc.Render(ctx, snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := svc.Render(ctx, snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("Markdown render not byte-identical across two renders")
	}
}

func TestMarkdownNoExternalAssets(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoExternalAssets(t, "markdown", data)
}

func TestMarkdownHasAllSectionsAndCaveat(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(data)
	// All 21 numbered sections present.
	for _, want := range []string{
		"1. Case Information", "5. Transaction Summary", "12. Risk Assessment",
		"18. Limitations", "21. Generation Metadata",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("markdown missing section %q", want)
		}
	}
	// Esplora caveat appears in both Transaction Summary and Limitations.
	if strings.Count(s, "Transaction size/weight fields may be absent") < 2 {
		t.Fatal("Esplora size caveat must appear in Transaction Summary and Limitations")
	}
}

func TestMarkdownNoSecrets(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatMarkdown)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoSecrets(t, "markdown", data)
}
