package mapdata

import (
	"os"
	"path/filepath"
	"testing"
)

const fixtureAsset = "testdata/world-110m.asset"

// TestInstallRoundTripAndLoad installs the fixture asset, round-trips the
// registry, and loads the geometry via the sha256-verified Load path.
func TestInstallRoundTripAndLoad(t *testing.T) {
	dir := t.TempDir()
	meta := RegistryEntry{
		Source:    "Natural Earth 1:110m (fixture)",
		SourceURL: "https://www.naturalearthdata.com",
		Version:   "natural-earth-110m",
		License:   "public domain",
	}
	entry, err := Install(dir, fixtureAsset, meta)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if entry.Name != "world-110m" {
		t.Fatalf("name = %q, want world-110m", entry.Name)
	}
	if entry.RecordCount != 2 {
		t.Fatalf("record_count = %d, want 2 centroids", entry.RecordCount)
	}
	if entry.SHA256 == "" {
		t.Fatal("sha256 not recorded")
	}

	// Round-trip the registry.
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if reg.Entry.SHA256 != entry.SHA256 || reg.Entry.Path != entry.Path {
		t.Fatalf("reloaded entry differs: %+v vs %+v", reg.Entry, entry)
	}

	// Load (verifies sha256) and check geometry.
	g, le, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if le.SHA256 != entry.SHA256 {
		t.Fatal("Load returned mismatched registry entry")
	}
	if len(g.Coastlines) != 2 {
		t.Fatalf("coastlines = %d, want 2", len(g.Coastlines))
	}
	if len(g.Centroids) != 2 {
		t.Fatalf("centroids = %d, want 2", len(g.Centroids))
	}
	if g.Centroids[0].ISOCode != "AA" || g.Centroids[1].ISOCode != "BB" {
		t.Fatalf("centroid ISO codes unexpected: %+v", g.Centroids)
	}
	if len(g.Coastlines[0].Points) != 3 {
		t.Fatalf("first coastline points = %d, want 3", len(g.Coastlines[0].Points))
	}
}

// TestVerifyMatchesAndRejects covers VerifyInstalled and VerifyFile against the
// installed sha256.
func TestVerifyMatchesAndRejects(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, fixtureAsset, RegistryEntry{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := VerifyInstalled(dir); err != nil {
		t.Fatalf("VerifyInstalled: %v", err)
	}
	if _, err := VerifyFile(dir, fixtureAsset); err != nil {
		t.Fatalf("VerifyFile(original): %v", err)
	}

	// A tampered candidate must fail VerifyFile.
	bad := filepath.Join(t.TempDir(), "bad.asset")
	b, _ := os.ReadFile(fixtureAsset)
	b = append(b, []byte("line\t{\"points\":[{\"lon\":1.0,\"lat\":1.0},{\"lon\":2.0,\"lat\":2.0}]}\n")...)
	if err := os.WriteFile(bad, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyFile(dir, bad); err != ErrChecksumMismatch {
		t.Fatalf("VerifyFile(tampered) = %v, want ErrChecksumMismatch", err)
	}
}

// TestMissingAssetDegrades asserts the honest not-installed state (no crash, no
// fabricated coastline) when nothing is installed.
func TestMissingAssetDegrades(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Load(dir); err != ErrNotInstalled {
		t.Fatalf("Load(empty) = %v, want ErrNotInstalled", err)
	}
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry(empty): %v", err)
	}
	if reg.Entry.Name != "" {
		t.Fatal("expected empty registry entry")
	}
	if _, err := VerifyInstalled(dir); err != ErrNotInstalled {
		t.Fatalf("VerifyInstalled(empty) = %v, want ErrNotInstalled", err)
	}
}

// TestCorruptAssetDegrades asserts a sha256 mismatch on the installed asset is
// treated like missing (degrade + warn), never rendered as a partial coastline.
func TestCorruptAssetDegrades(t *testing.T) {
	dir := t.TempDir()
	entry, err := Install(dir, fixtureAsset, RegistryEntry{})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	// Corrupt the installed asset after install so its sha256 no longer matches.
	if err := os.WriteFile(entry.Path, []byte("line\t{\"points\":[{\"lon\":0,\"lat\":0},{\"lon\":1,\"lat\":1}]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err != ErrChecksumMismatch {
		t.Fatalf("Load(corrupted) = %v, want ErrChecksumMismatch", err)
	}
	if _, err := VerifyInstalled(dir); err != ErrChecksumMismatch {
		t.Fatalf("VerifyInstalled(corrupted) = %v, want ErrChecksumMismatch", err)
	}
}

// TestLoadGeometryCorrupt asserts a structurally invalid asset is rejected.
func TestLoadGeometryCorrupt(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.asset")
	if err := os.WriteFile(bad, []byte("garbage-no-tab-here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGeometry(bad); err == nil {
		t.Fatal("expected ErrCorruptAsset for malformed asset")
	}
	// Missing file -> ErrNotInstalled.
	if _, err := LoadGeometry(filepath.Join(t.TempDir(), "nope.asset")); err != ErrNotInstalled {
		t.Fatalf("LoadGeometry(missing) = %v, want ErrNotInstalled", err)
	}
}

// TestCorruptRegistry asserts a present-but-invalid registry.json is reported.
func TestCorruptRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(dir); err == nil {
		t.Fatal("expected error for corrupt registry")
	}
}
