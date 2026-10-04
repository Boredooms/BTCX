package tests

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
)

func TestHTMLDeterministic(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	a, err := svc.Render(ctx, snap, schema.FormatHTML)
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := svc.Render(ctx, snap, schema.FormatHTML)
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("HTML render not byte-identical across two renders")
	}
}

func TestHTMLSelfContained(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatHTML)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoExternalAssets(t, "html", data)

	s := string(data)
	// Explicit disallowed constructs.
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script\s+src=`),
		regexp.MustCompile(`(?i)<link[^>]+rel=["']?stylesheet["']?[^>]+href=["']?https?:`),
		regexp.MustCompile(`(?i)<img[^>]+src=["']?https?:`),
	} {
		if re.Match(data) {
			t.Fatalf("HTML contains forbidden external reference %q", re.String())
		}
	}
	// CSS must be inlined in a <style> tag.
	if !strings.Contains(s, "<style>") {
		t.Fatal("HTML must inline CSS in a <style> tag")
	}
	// Must be a complete standalone document.
	if !strings.HasPrefix(strings.TrimSpace(s), "<!DOCTYPE html>") {
		t.Fatal("HTML must begin with a DOCTYPE")
	}
}

func TestHTMLNoSecrets(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatHTML)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoSecrets(t, "html", data)
}
