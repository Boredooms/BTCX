package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/tui/components"
)

// TestNetworkResolvesCityFromDB asserts the Network/IP screen resolves per-row
// country/city/ASN from the installed City DB and labels them honestly as
// geolocation metadata (estimate), never ownership.
func TestNetworkResolvesCityFromDB(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	n := NewNetwork(ctx)
	var m Model = n
	if cmd := n.Init(); cmd != nil {
		for _, msg := range runBatch(cmd) {
			m, _ = m.Update(msg)
		}
	}
	out := m.View(components.Frame{W: 160, H: 50})

	assertNoOverflow(t, "network-geo", out, components.Frame{W: 160, H: 50})

	for _, want := range []string{
		"NETWORK / IP",
		"East Finchley", // resolved city column
		"geolocation metadata (estimate)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("network screen missing %q in:\n%s", want, out)
		}
	}
	// Honest framing: never an ownership claim.
	if strings.Contains(strings.ToLower(out), "belongs to") {
		t.Errorf("network screen must not claim ownership:\n%s", out)
	}
}

// TestDashboardGeoActivityFromDB asserts the Dashboard GEO panel shows the
// country/city activity summary resolved via the shared geoenrich pipeline when
// the City DB is installed, with honest estimate wording.
func TestDashboardGeoActivityFromDB(t *testing.T) {
	ctx := geoFixtureCtx(t, seededGeoObs())
	out := renderDashboard(t, ctx, components.Frame{W: 160, H: 50})

	assertNoOverflow(t, "dashboard-geo", out, components.Frame{W: 160, H: 50})

	for _, want := range []string{
		"GEO ACTIVITY — IP GEOLOCATION (ESTIMATE)",
		"GeoIP City DB", // provenance of the resolved summary
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard GEO panel missing %q in:\n%s", want, out)
		}
	}
	// GB should appear as a resolved country in the activity summary (the city
	// fixture resolves every seeded IP to GB).
	if !strings.Contains(out, "GB") {
		t.Errorf("dashboard GEO panel should list the resolved country GB in:\n%s", out)
	}
}
