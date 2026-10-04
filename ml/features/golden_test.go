package features

import (
	"path/filepath"
	"testing"
)

// TestGoldenParity verifies the Go feature-schema-v1 contract matches the
// frozen Python golden fixtures: identical version, count, order, and vector
// width. This is the Go/Python feature parity gate.
func TestGoldenParity(t *testing.T) {
	path := filepath.Join("..", "..", "ml-lab", "evaluation", "feature_golden.json")
	g, err := LoadGolden(path)
	if err != nil {
		t.Skipf("golden fixtures not present (run ml_lab.features.freeze): %v", err)
	}
	if err := g.VerifyOrder(); err != nil {
		t.Fatalf("feature parity failed: %v", err)
	}
	if len(g.Fixtures) == 0 {
		t.Fatal("expected at least one golden fixture")
	}
	t.Logf("verified %d golden fixtures against %d-feature contract",
		len(g.Fixtures), len(Order))
}

// TestOrderMatchesSchemaLength is a static guard on the 25-feature contract.
func TestOrderMatchesSchemaLength(t *testing.T) {
	if len(Order) != 25 {
		t.Fatalf("feature-schema-v1 must have 25 features, got %d", len(Order))
	}
}
