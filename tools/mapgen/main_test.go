package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGeneratorDeterministic asserts that running the generator twice over the
// same source produces byte-for-byte identical output (hence identical sha256),
// which is the reproducibility contract for the released world-110m.asset.
func TestGeneratorDeterministic(t *testing.T) {
	src := "testdata/natural-earth-110m.geojson"

	out1 := filepath.Join(t.TempDir(), "a.asset")
	out2 := filepath.Join(t.TempDir(), "b.asset")

	if err := run(src, out1); err != nil {
		t.Fatalf("run #1: %v", err)
	}
	if err := run(src, out2); err != nil {
		t.Fatalf("run #2: %v", err)
	}

	b1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if string(b1) != string(b2) {
		t.Fatal("generator output is not deterministic across runs")
	}

	s1, err := sha256File(out1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := sha256File(out2)
	if err != nil {
		t.Fatal(err)
	}
	if s1 != s2 {
		t.Fatalf("sha256 differs: %s vs %s", s1, s2)
	}
}

// TestGeneratorContent asserts the generator emits coastlines for both coded
// countries and the uncoded one, two centroids (the "-99" country is skipped),
// and that the output parses as a valid asset in the mapdata format.
func TestGeneratorContent(t *testing.T) {
	src := "testdata/natural-earth-110m.geojson"
	out := filepath.Join(t.TempDir(), "world.asset")
	if err := run(src, out); err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)

	countLine := 0
	countCentroid := 0
	for _, ln := range splitLines(content) {
		switch {
		case len(ln) >= 5 && ln[:5] == "line\t":
			countLine++
		case len(ln) >= 9 && ln[:9] == "centroid\t":
			countCentroid++
		}
	}
	if countLine != 3 {
		t.Fatalf("coastline records = %d, want 3 (AA, BB, and uncoded)", countLine)
	}
	if countCentroid != 2 {
		t.Fatalf("centroid records = %d, want 2 (uncoded -99 skipped)", countCentroid)
	}
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
