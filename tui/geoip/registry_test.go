package geoip

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

// TestRegistryRoundTripAndVerify installs a tiny fixture .mmdb, confirms the
// registry round-trips, and that Verify matches by recomputed sha256 and
// rejects a tampered file.
func TestRegistryRoundTripAndVerify(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "dbip-country-lite.mmdb")
	writeFixtureCountryMMDB(t, src)

	meta := RegistryEntry{
		Source:    "DB-IP IP-to-Country Lite (fixture)",
		SourceURL: "https://db-ip.com",
		Version:   "2024.11",
		License:   "CC BY 4.0",
	}
	entry, err := Install(dir, src, meta)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if entry.DBType != "country" {
		t.Fatalf("db_type = %q, want country", entry.DBType)
	}
	if entry.SHA256 == "" {
		t.Fatal("sha256 not recorded")
	}
	if entry.License != "CC BY 4.0" || entry.Version != "2024.11" {
		t.Fatalf("provenance not preserved: %+v", entry)
	}

	// Round-trip: reload the registry and compare.
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	got, ok := reg.Entries["country"]
	if !ok {
		t.Fatal("country entry missing after reload")
	}
	if got.SHA256 != entry.SHA256 || got.Path != entry.Path {
		t.Fatalf("reloaded entry differs: %+v vs %+v", got, entry)
	}

	// Verify the installed copy by sha256.
	if _, err := Verify(dir, entry.Path); err != nil {
		t.Fatalf("Verify installed file: %v", err)
	}

	// A tampered copy must fail verification.
	bad := filepath.Join(t.TempDir(), "bad.mmdb")
	b, _ := os.ReadFile(entry.Path)
	b = append(b, 0xFF)
	if err := os.WriteFile(bad, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(dir, bad); err != ErrChecksumMismatch {
		t.Fatalf("Verify(tampered) = %v, want ErrChecksumMismatch", err)
	}
}

// TestOpenNotInstalled asserts the honest not-installed state (no crash, no
// fabricated data) when no database is present.
func TestOpenNotInstalled(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err != ErrNotInstalled {
		t.Fatalf("Open(empty) = %v, want ErrNotInstalled", err)
	}
	// A registry read of an empty dir is not an error; it is empty.
	reg, err := LoadRegistry(dir)
	if err != nil {
		t.Fatalf("LoadRegistry(empty): %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Fatalf("expected empty registry, got %d entries", len(reg.Entries))
	}
}

// TestLookupCountry opens the installed fixture and resolves a known IP.
func TestLookupCountry(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "dbip-country-lite.mmdb")
	writeFixtureCountryMMDB(t, src)
	if _, err := Install(dir, src, RegistryEntry{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	d, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	// The fixture maps the whole IPv4 space to US.
	res, err := d.LookupCountry(netip.MustParseAddr("1.2.3.4"))
	if err != nil {
		t.Fatalf("LookupCountry: %v", err)
	}
	if res.ISOCode != "US" || res.Name != "United States" {
		t.Fatalf("LookupCountry = %+v, want US/United States", res)
	}

	// ASN DB not installed -> empty result, not an error.
	asn, err := d.LookupASN(netip.MustParseAddr("1.2.3.4"))
	if err != nil {
		t.Fatalf("LookupASN: %v", err)
	}
	if asn.Number != 0 {
		t.Fatalf("expected empty ASN result, got %+v", asn)
	}

	if d.Status().DBType != "country" {
		t.Fatalf("Status db_type = %q, want country", d.Status().DBType)
	}
}

// TestCorruptRegistry asserts a present-but-invalid registry.json is reported
// honestly rather than silently treated as empty.
func TestCorruptRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "registry.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(dir); err == nil {
		t.Fatal("expected error for corrupt registry")
	}
}
