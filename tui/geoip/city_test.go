package geoip

import (
	"net/netip"
	"path/filepath"
	"testing"
)

// TestLookupCityReturnsCoordinates installs the City fixture and asserts
// LookupCity returns the city, subdivision, country and lat/lon straight from a
// real maxminddb read, and that HasCity reports true for a City edition.
func TestLookupCityReturnsCoordinates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "geolite2-city.mmdb")
	writeFixtureCityMMDB(t, src)
	if _, err := Install(dir, src, RegistryEntry{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	d, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	if !d.HasCity() {
		t.Fatal("HasCity() = false for a City-edition DB, want true")
	}

	city, err := d.LookupCity(netip.MustParseAddr("81.2.69.142"))
	if err != nil {
		t.Fatalf("LookupCity: %v", err)
	}
	if !city.HasCoords {
		t.Fatalf("expected HasCoords=true for a City DB, got %+v", city)
	}
	if city.ISOCode != "GB" || city.CountryName != "United Kingdom" {
		t.Errorf("country = %q/%q, want GB/United Kingdom", city.ISOCode, city.CountryName)
	}
	if city.City != "East Finchley" {
		t.Errorf("city = %q, want East Finchley", city.City)
	}
	if city.Subdivision != "England" {
		t.Errorf("subdivision = %q, want England", city.Subdivision)
	}
	// Coordinates come from the DB record, not fabricated: allow a tiny epsilon
	// for the float64 round-trip.
	if diff := city.Latitude - 51.5967; diff > 1e-4 || diff < -1e-4 {
		t.Errorf("latitude = %v, want ~51.5967", city.Latitude)
	}
	if diff := city.Longitude - -0.1593; diff > 1e-4 || diff < -1e-4 {
		t.Errorf("longitude = %v, want ~-0.1593", city.Longitude)
	}

	// LookupCountry must still work against the same City DB (country fields are
	// present in a City edition too).
	cr, err := d.LookupCountry(netip.MustParseAddr("81.2.69.142"))
	if err != nil {
		t.Fatalf("LookupCountry on City DB: %v", err)
	}
	if cr.ISOCode != "GB" {
		t.Errorf("LookupCountry on City DB iso = %q, want GB", cr.ISOCode)
	}
}

// TestLookupCityOnCountryDBDegradesHonestly asserts that a plain *country*
// edition DB (no location fields) yields HasCity=false and a CityResult with no
// coordinates — never a fabricated point.
func TestLookupCityOnCountryDBDegradesHonestly(t *testing.T) {
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

	if d.HasCity() {
		t.Error("HasCity() = true for a plain country DB, want false")
	}
	city, err := d.LookupCity(netip.MustParseAddr("1.2.3.4"))
	if err != nil {
		t.Fatalf("LookupCity: %v", err)
	}
	if city.HasCoords {
		t.Errorf("country DB must not yield coordinates, got %+v", city)
	}
	// The country-level fields it does have are still returned honestly.
	if city.ISOCode != "US" {
		t.Errorf("country iso = %q, want US (country field present on country DB)", city.ISOCode)
	}
}
