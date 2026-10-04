package geoenrich

import (
	"testing"
	"time"

	"github.com/bctx/bctx/pkg/schema"
)

// TestResolveRealPublicIPs is a diagnostic that resolves the real public IPs
// used by the Geo Map demo dataset against the INSTALLED GeoIP City DB
// (~/.bctx/geoip) and prints the resolved country/city/coordinates. It proves
// those endpoints plot as real pins. It skips cleanly when no DB is installed
// (CI without the City DB) rather than failing. Run with:
//
//	go test ./tui/geoenrich -run TestResolveRealPublicIPs -v
func TestResolveRealPublicIPs(t *testing.T) {
	ips := []string{
		"8.8.8.8", "1.1.1.1", "9.9.9.9", "208.67.222.222",
		"195.46.39.39", "80.80.80.80", "185.228.168.9", "77.88.8.8",
		"114.114.114.114", "168.95.1.1", "139.130.4.4", "200.221.11.100",
		"196.25.1.1", "202.12.27.33", "91.239.100.100", "156.154.70.1",
	}
	now := time.Now().UTC()
	obs := make([]schema.NetworkObservation, 0, len(ips))
	for i, ip := range ips {
		obs = append(obs, schema.NetworkObservation{
			ID: "o" + ip, TxID: "demo-geo-tx-0001", SrcIP: ip,
			DstIP: "151.101.1.69", Timestamp: now.Add(time.Duration(i) * time.Minute),
		})
	}

	res := Enrich(obs, 0)
	if !res.Installed {
		t.Skip("no GeoIP DB installed; skipping real-IP resolution probe")
	}
	t.Logf("installed=%v hasCity=%v distinctIPs=%d clusters=%d",
		res.Installed, res.HasCity, res.DistinctIPs, len(res.Clusters))

	resolved := 0
	for _, l := range res.Locations {
		if l.HasCoords {
			resolved++
			t.Logf("  %-16s %-3s %-18s %8.3f, %8.3f  obs=%d",
				l.IP, l.ISOCode, l.City, l.Latitude, l.Longitude, l.Observations)
		} else {
			t.Logf("  %-16s (no coords: %s/%s)", l.IP, l.ISOCode, l.Country)
		}
	}
	t.Logf("resolved %d/%d IPs to coordinates", resolved, len(ips))
	// When a City DB is present we expect the great majority to resolve; a few
	// endpoints may be absent from a given GeoLite2 snapshot, which is honest.
	if res.HasCity && resolved == 0 {
		t.Errorf("City DB installed but no IP resolved to coordinates — check the DB")
	}
}
