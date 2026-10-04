package geoip

import (
	"os"
	"testing"
)

// TestGenerateCommittedCityFixture is a one-off generator (guarded by an env
// var) that writes the synthetic City-edition fixture .mmdb used by the screens
// golden tests. It is NOT MaxMind data — it is BCTX's own tiny structurally
// valid MMDB. Run manually with:
//
//	BCTX_GEN_CITY_FIXTURE=../screens/testdata/city-fixture.mmdb go test -run TestGenerateCommittedCityFixture ./tui/geoip
func TestGenerateCommittedCityFixture(t *testing.T) {
	out := os.Getenv("BCTX_GEN_CITY_FIXTURE")
	if out == "" {
		t.Skip("set BCTX_GEN_CITY_FIXTURE=<path> to (re)generate the committed city fixture")
	}
	b := buildCityMMDB("GB", "United Kingdom", "East Finchley", "England", 51.5967, -0.1593)
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatalf("write committed city fixture: %v", err)
	}
	t.Logf("wrote %d-byte city fixture to %s", len(b), out)
}
