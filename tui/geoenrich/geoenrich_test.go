package geoenrich

import (
	"net/netip"
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/geoip"
)

// fakeResolver maps a few fixture IPs to deterministic City/ASN results so the
// enrichment core can be tested without an on-disk MMDB. Unknown IPs resolve to
// an empty (no-coords) result — mirroring an honest DB miss.
func fakeResolver(table map[string]geoip.CityResult, asns map[string]geoip.ASNResult) lookupFn {
	return func(ip netip.Addr) (geoip.CityResult, geoip.ASNResult) {
		return table[ip.String()], asns[ip.String()]
	}
}

func obsAt(id, tx, src, dst, country, asn string, ts time.Time) schema.NetworkObservation {
	return schema.NetworkObservation{ID: id, TxID: tx, SrcIP: src, DstIP: dst, Country: country, ASN: asn, Timestamp: ts}
}

// TestEnrichBinsDeterministically asserts the enrichment core aggregates per IP,
// resolves coordinates, clusters nearby points into one cell, and produces
// stable, sorted output across runs.
func TestEnrichBinsDeterministically(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	table := map[string]geoip.CityResult{
		// Two London IPs cluster together; one New York IP is its own cluster.
		"81.2.69.142":  {ISOCode: "GB", CountryName: "United Kingdom", City: "East Finchley", Subdivision: "England", Latitude: 51.59, Longitude: -0.15, HasCoords: true},
		"81.2.69.143":  {ISOCode: "GB", CountryName: "United Kingdom", City: "London", Subdivision: "England", Latitude: 51.50, Longitude: -0.12, HasCoords: true},
		"151.101.1.69": {ISOCode: "US", CountryName: "United States", Latitude: 40.71, Longitude: -74.0, HasCoords: true},
	}
	asns := map[string]geoip.ASNResult{
		"81.2.69.142": {Number: 2856, Org: "BT"},
	}
	obs := []schema.NetworkObservation{
		obsAt("o1", "TX1", "81.2.69.142", "151.101.1.69", "GB", "", t0),
		obsAt("o2", "TX2", "81.2.69.143", "151.101.1.69", "GB", "", t0.Add(time.Hour)),
		obsAt("o3", "TX1", "81.2.69.142", "81.2.69.143", "GB", "", t0.Add(2*time.Hour)),
	}

	r := enrichWith(obs, 8.0, true, true, fakeResolver(table, asns))

	if !r.Installed || !r.HasCity {
		t.Fatalf("expected installed city DB result, got %+v", r)
	}
	if r.DistinctIPs != 3 {
		t.Fatalf("distinct IPs = %d, want 3", r.DistinctIPs)
	}
	// London IP .142 appears in o1 and o3 -> 2 observations.
	var london142 *IPLocation
	for i := range r.Locations {
		if r.Locations[i].IP == "81.2.69.142" {
			london142 = &r.Locations[i]
		}
	}
	if london142 == nil {
		t.Fatal("missing 81.2.69.142 location")
	}
	if london142.Observations != 2 {
		t.Errorf("81.2.69.142 observations = %d, want 2", london142.Observations)
	}
	if london142.City != "East Finchley" || london142.ISOCode != "GB" {
		t.Errorf("81.2.69.142 city/iso = %q/%q", london142.City, london142.ISOCode)
	}
	if london142.ASN != "AS2856 BT" {
		t.Errorf("81.2.69.142 asn = %q, want AS2856 BT", london142.ASN)
	}
	if len(london142.RelatedTx) != 2 { // TX1 (o1,o3) + TX2? no, .142 only in TX1
		// .142 is in o1(TX1) and o3(TX1) -> only TX1 distinct.
		if !(len(london142.RelatedTx) == 1 && london142.RelatedTx[0] == "TX1") {
			t.Errorf("81.2.69.142 related tx = %v, want [TX1]", london142.RelatedTx)
		}
	}

	// Two GB IPs within 8 degrees cluster into one cell; US is separate -> 2 clusters.
	if len(r.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2 (GB cluster + US cluster)", len(r.Clusters))
	}
	// Determinism: a second run yields identical cluster centroids/order.
	r2 := enrichWith(obs, 8.0, true, true, fakeResolver(table, asns))
	for i := range r.Clusters {
		if r.Clusters[i].Lat != r2.Clusters[i].Lat || r.Clusters[i].Lon != r2.Clusters[i].Lon ||
			r.Clusters[i].Observations != r2.Clusters[i].Observations {
			t.Fatalf("cluster %d not deterministic: %+v vs %+v", i, r.Clusters[i], r2.Clusters[i])
		}
	}

	// Country rollup: GB has the most observations (4: o1 src, o2 src, o3 src+dst).
	if len(r.Countries) == 0 || r.Countries[0].ISOCode != "GB" {
		t.Errorf("top country = %+v, want GB first", r.Countries)
	}
}

// TestEnrichDegradesWithoutDB asserts that with no resolver (DB absent) the core
// still aggregates per IP using the observations' own labels and plots NO
// coordinates — never a fabricated point.
func TestEnrichDegradesWithoutDB(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	obs := []schema.NetworkObservation{
		obsAt("o1", "TX1", "203.0.113.9", "198.51.100.2", "DE", "AS3320", t0),
	}
	r := enrichWith(obs, 0, false, false, nil)
	if r.Installed || r.HasCity {
		t.Fatalf("expected not-installed result, got %+v", r)
	}
	if len(r.Clusters) != 0 {
		t.Errorf("no DB must yield no plotted clusters, got %d", len(r.Clusters))
	}
	if r.DistinctIPs != 2 {
		t.Errorf("distinct IPs = %d, want 2", r.DistinctIPs)
	}
	for _, l := range r.Locations {
		if l.HasCoords {
			t.Errorf("no DB must not produce coordinates: %+v", l)
		}
	}
	// The observation's own country label still rolls up (honest label-only).
	if len(r.Countries) == 0 || r.Countries[0].ISOCode != "DE" {
		t.Errorf("label-only country rollup = %+v, want DE", r.Countries)
	}
}

// TestEnrichSkipsEmptyIPs asserts blank src/dst IPs are ignored, not binned as
// an empty IP.
func TestEnrichSkipsEmptyIPs(t *testing.T) {
	obs := []schema.NetworkObservation{
		obsAt("o1", "TX1", "203.0.113.9", "", "DE", "", time.Now()),
		obsAt("o2", "TX2", "", "", "", "", time.Now()),
	}
	r := enrichWith(obs, 0, false, false, nil)
	if r.DistinctIPs != 1 {
		t.Fatalf("distinct IPs = %d, want 1 (blanks skipped)", r.DistinctIPs)
	}
}
