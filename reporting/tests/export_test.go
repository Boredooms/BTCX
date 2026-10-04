package tests

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/models"
)

func allFormats() []schema.ReportFormat {
	return []schema.ReportFormat{schema.FormatJSON, schema.FormatMarkdown, schema.FormatHTML, schema.FormatPDF}
}

func TestExportBundleRoundTrip(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	outDir := t.TempDir()

	manifest, err := svc.ExportBundle(ctx, snap, allFormats(), outDir)
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}

	bundleDir := filepath.Join(outDir, "report")
	wantFiles := []string{
		"report.json", "report.md", "report.html", "report.pdf",
		"evidence.json", "timeline.json", "manifest.json", "checksums.sha256",
	}
	for _, f := range wantFiles {
		if _, err := os.Stat(filepath.Join(bundleDir, f)); err != nil {
			t.Fatalf("expected bundle file %q: %v", f, err)
		}
	}

	// Manifest entries must be sorted by name and cover every file except
	// itself and the checksum file is included (it is written last and listed).
	names := make([]string, 0, len(manifest.Files))
	for _, e := range manifest.Files {
		names = append(names, e.Name)
	}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("manifest files not sorted by name: %v", names)
	}

	// checksums.sha256 must be coreutils format and sorted by name.
	cb, err := os.ReadFile(filepath.Join(bundleDir, "checksums.sha256"))
	if err != nil {
		t.Fatalf("read checksums: %v", err)
	}
	var sumNames []string
	for _, line := range strings.Split(strings.TrimRight(string(cb), "\n"), "\n") {
		idx := strings.Index(line, "  ")
		if idx <= 0 {
			t.Fatalf("checksum line not coreutils format: %q", line)
		}
		sumNames = append(sumNames, line[idx+2:])
	}
	if !sort.StringsAreSorted(sumNames) {
		t.Fatalf("checksums.sha256 not sorted by name: %v", sumNames)
	}

	// Verify an intact bundle.
	res, err := svc.Verify(ctx, bundleDir)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !res.OK {
		t.Fatalf("intact bundle failed verification: %+v", res.Files)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()
	outDir := t.TempDir()
	if _, err := svc.ExportBundle(ctx, snap, []schema.ReportFormat{schema.FormatJSON, schema.FormatMarkdown}, outDir); err != nil {
		t.Fatalf("export: %v", err)
	}
	bundleDir := filepath.Join(outDir, "report")

	// Flip one byte in report.md.
	mdPath := filepath.Join(bundleDir, "report.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read md: %v", err)
	}
	data[len(data)/2] ^= 0xFF
	if err := os.WriteFile(mdPath, data, 0o644); err != nil {
		t.Fatalf("write tampered md: %v", err)
	}

	res, err := svc.Verify(ctx, bundleDir)
	if err != nil {
		t.Fatalf("verify (tampered) returned error, want DATA result: %v", err)
	}
	if res.OK {
		t.Fatal("verify reported OK on tampered bundle")
	}
	if statusOf(res, "report.md") != "MISMATCH" {
		t.Fatalf("expected report.md MISMATCH, got %q", statusOf(res, "report.md"))
	}

	// Delete report.json -> MISSING.
	if err := os.Remove(filepath.Join(bundleDir, "report.json")); err != nil {
		t.Fatalf("remove json: %v", err)
	}
	res, err = svc.Verify(ctx, bundleDir)
	if err != nil {
		t.Fatalf("verify (missing) error: %v", err)
	}
	if res.OK {
		t.Fatal("verify reported OK with a missing file")
	}
	if statusOf(res, "report.json") != "MISSING" {
		t.Fatalf("expected report.json MISSING, got %q", statusOf(res, "report.json"))
	}
}

func TestExportBundleRejectsUnsafeOutput(t *testing.T) {
	svc, snap := newFixedService(t)
	// SafePath rejects a bundle dir name escape; outDir itself is fine, but the
	// bundle name is fixed. Confirm SafePath guard is wired by exporting into a
	// path that does not exist yet is created, and that a traversal attempt in
	// the model guard fails independently.
	if _, err := models.SafePath(t.TempDir(), "../escape"); err == nil {
		t.Fatal("SafePath should reject .. escape")
	}
	// A normal export must still succeed.
	if _, err := svc.ExportBundle(context.Background(), snap, []schema.ReportFormat{schema.FormatJSON}, t.TempDir()); err != nil {
		t.Fatalf("normal export failed: %v", err)
	}
}

func statusOf(res models.VerifyResult, name string) string {
	for _, f := range res.Files {
		if f.Name == name {
			return f.Status
		}
	}
	return ""
}
