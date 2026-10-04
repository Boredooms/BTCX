package tests

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting"
)

func TestPDFDeterministic(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	a, err := svc.Render(ctx, snap, schema.FormatPDF)
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := svc.Render(ctx, snap, schema.FormatPDF)
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("PDF render not byte-identical across two fixed-clock renders")
	}
}

func TestPDFStructure(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatPDF)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(data)

	if !strings.HasPrefix(s, "%PDF-1.4") {
		t.Fatal("PDF must begin with %PDF-1.4")
	}
	for _, want := range []string{"xref", "trailer", "startxref", "%%EOF"} {
		if !strings.Contains(s, want) {
			t.Fatalf("PDF missing required marker %q", want)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(s, "\n"), "%%EOF") {
		t.Fatal("PDF must end with the EOF marker")
	}

	// /ID must equal the snapshot hash (lowercase hex) twice.
	wantID := reporting.SnapshotHash(snap)
	if !strings.Contains(s, "/ID [<"+wantID+"> <"+wantID+">]") {
		t.Fatalf("PDF /ID is not the snapshot hash %s", wantID)
	}

	// Object count must match /Size in the trailer.
	sizeRe := regexp.MustCompile(`/Size (\d+)`)
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		t.Fatal("PDF trailer missing /Size")
	}
	size, _ := strconv.Atoi(m[1])
	objRe := regexp.MustCompile(`(?m)^\d+ 0 obj`)
	objCount := len(objRe.FindAllString(s, -1))
	// /Size counts object 0 (the free head) plus every real object.
	if size != objCount+1 {
		t.Fatalf("PDF /Size %d != object count+1 (%d)", size, objCount+1)
	}

	// Required content sections (drawn as text) must be present.
	for _, want := range []string{
		"BCTX Forensic Report", "Case", "Subject", "Risk Assessment",
		"Key Findings", "Timeline", "Transaction Summary", "Graph Summary",
		"Evidence", "Alerts", "Provenance", "Limitations",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("PDF missing content section %q", want)
		}
	}
}

func TestPDFNoSecrets(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatPDF)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoSecrets(t, "pdf", data)
}
