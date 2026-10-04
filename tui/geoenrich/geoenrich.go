// Package geoenrich is the STRICTLY-OFFLINE presentation-layer bridge between
// the case's network observations and the installed GeoIP City DB. It resolves
// each distinct observed IP to {country, city, lat/lon, ASN} through the local
// MMDB (tui/geoip), aggregates observations per IP, and clusters the resulting
// points into terminal-scale bins so the Geo Map never plots thousands of raw
// coordinates.
//
// It adds NOTHING to the canonical schema (schema.NetworkObservation keeps no
// lat/lon): coordinates live only here, as derived presentation data. It never
// dials, never shells out, never fabricates a coordinate — a missing DB or a
// missing record degrades to country/ASN labels and HasCoords=false. Honesty is
// paramount: geolocation is observation metadata, not proof of ownership
// (AGENTS §18, design §13).
package geoenrich

import (
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/geoip"
)

// IPLocation is one distinct observed IP resolved against the GeoIP DB and
// aggregated across every observation that referenced it (as src or dst). It is
// derived presentation data — never persisted, never added to the schema.
type IPLocation struct {
	IP          string
	ISOCode     string
	Country     string
	City        string
	Subdivision string
	ASN         string
	Latitude    float64
	Longitude   float64
	HasCoords   bool

	// Observations is how many case observations referenced this IP (as src or
	// dst). RelatedTx / RelatedWallets are the distinct correlated subjects, so
	// selecting a point on the map can open the related tx / wallet / graph.
	Observations   int
	RelatedTx      []string
	RelatedWallets []string
	FirstSeen      time.Time
	LastSeen       time.Time
}

// CountryActivity is a per-country rollup used by the dashboard GEO panel.
type CountryActivity struct {
	ISOCode      string
	Country      string
	Observations int
	IPs          int
}

// Cluster is a group of IPLocations binned to one terminal-scale cell. Points
// are clustered by a coarse lat/lon grid so the map plots a bounded number of
// markers regardless of how many raw IPs the case holds (design §13: never
// thousands of raw points).
type Cluster struct {
	// Lat / Lon is the observation-weighted centroid of the member points.
	Lat, Lon float64
	// Observations is the summed observation count of all members; IPs is the
	// number of distinct IPs; Members are the underlying resolved IPs (sorted).
	Observations int
	IPs          int
	Members      []IPLocation
}

// Result is the full enrichment output for one render.
type Result struct {
	// Installed reports whether a GeoIP DB was found. When false the Locations
	// carry only the Country/ASN labels already on the observations and the
	// caller shows the documented NOT-INSTALLED note.
	Installed bool
	// HasCity reports whether the installed DB is a City edition (coordinates
	// available). A country-only DB yields Installed=true, HasCity=false.
	HasCity bool

	Locations []IPLocation
	Clusters  []Cluster
	Countries []CountryActivity

	// TotalObservations / DistinctIPs summarise the input for honest headers.
	TotalObservations int
	DistinctIPs       int
}

// lookupFn resolves a single IP to a CityResult + ASN. It is abstracted so a
// test can inject a deterministic resolver without an on-disk MMDB.
type lookupFn func(netip.Addr) (geoip.CityResult, geoip.ASNResult)

// Enrich resolves and bins obs using the GeoIP DB installed under the default
// ~/.bctx/geoip directory. It opens the DB read-only for the duration of the
// call and caches each distinct IP lookup once (per-render cache). A missing DB
// degrades honestly to label-only output. gridDeg is the clustering cell size
// in degrees (e.g. 10.0); values <= 0 default to a sane terminal-scale grid.
func Enrich(obs []schema.NetworkObservation, gridDeg float64) Result {
	db, err := openDefault()
	if err != nil || db == nil {
		// No DB: degrade to label-only aggregation using the observations' own
		// Country/ASN fields; never fabricate a coordinate.
		return enrichWith(obs, gridDeg, false, false, nil)
	}
	defer db.Close()
	return EnrichDB(db, obs, gridDeg)
}

// EnrichDB is Enrich against a caller-supplied open DB (used by tests and the
// Geo Map preview for deterministic output without touching ~/.bctx). A nil db
// degrades to label-only, exactly like a missing installation.
func EnrichDB(db geoip.DB, obs []schema.NetworkObservation, gridDeg float64) Result {
	if db == nil {
		return enrichWith(obs, gridDeg, false, false, nil)
	}
	cache := map[string]struct {
		city geoip.CityResult
		asn  geoip.ASNResult
	}{}
	resolve := func(ip netip.Addr) (geoip.CityResult, geoip.ASNResult) {
		key := ip.String()
		if hit, ok := cache[key]; ok {
			return hit.city, hit.asn
		}
		city, _ := db.LookupCity(ip)
		asn, _ := db.LookupASN(ip)
		cache[key] = struct {
			city geoip.CityResult
			asn  geoip.ASNResult
		}{city, asn}
		return city, asn
	}
	return enrichWith(obs, gridDeg, true, db.HasCity(), resolve)
}

// openDefault opens the installed GeoIP DB, returning (nil, nil) when none is
// installed so Enrich can degrade without treating "not installed" as an error.
func openDefault() (geoip.DB, error) {
	dir, err := geoip.Dir()
	if err != nil {
		return nil, nil
	}
	db, err := geoip.Open(dir)
	if err != nil {
		// ErrNotInstalled (and any open failure) degrades to label-only; the
		// caller shows the documented note rather than crashing.
		return nil, nil
	}
	return db, nil
}

// defaultGridDeg is the clustering cell size when the caller passes <= 0. Ten
// degrees keeps the number of plotted markers bounded at terminal scale.
const defaultGridDeg = 8.0

// enrichWith is the pure core: it aggregates observations per distinct IP,
// resolves each once via resolve (nil = label-only), rolls up per-country
// activity, and clusters coordinate-bearing points. It is deterministic: all
// output slices are sorted so golden tests are stable.
func enrichWith(obs []schema.NetworkObservation, gridDeg float64, installed, hasCity bool, resolve lookupFn) Result {
	if gridDeg <= 0 {
		gridDeg = defaultGridDeg
	}

	// Aggregate per distinct IP. Each observation contributes its src and dst
	// IP (both are observed endpoints); related tx/wallet subjects are threaded
	// so a selected point can open the right screen.
	type agg struct {
		loc       IPLocation
		txSet     map[string]struct{}
		walletSet map[string]struct{}
	}
	byIP := map[string]*agg{}

	touch := func(ip string, o schema.NetworkObservation, fallbackCountry, fallbackASN string) {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			return
		}
		a := byIP[ip]
		if a == nil {
			a = &agg{
				loc:       IPLocation{IP: ip, ISOCode: fallbackCountry, ASN: fallbackASN},
				txSet:     map[string]struct{}{},
				walletSet: map[string]struct{}{},
			}
			byIP[ip] = a
		}
		a.loc.Observations++
		if o.Country != "" && a.loc.ISOCode == "" {
			a.loc.ISOCode = o.Country
		}
		if o.ASN != "" && a.loc.ASN == "" {
			a.loc.ASN = o.ASN
		}
		if o.TxID != "" {
			a.txSet[o.TxID] = struct{}{}
		}
		for _, w := range relatedWallets(o) {
			a.walletSet[w] = struct{}{}
		}
		if a.loc.FirstSeen.IsZero() || o.Timestamp.Before(a.loc.FirstSeen) {
			a.loc.FirstSeen = o.Timestamp
		}
		if o.Timestamp.After(a.loc.LastSeen) {
			a.loc.LastSeen = o.Timestamp
		}
	}

	total := 0
	for _, o := range obs {
		total++
		touch(o.SrcIP, o, o.Country, o.ASN)
		touch(o.DstIP, o, o.Country, o.ASN)
	}

	// Resolve each distinct IP once and finalise related sets.
	locations := make([]IPLocation, 0, len(byIP))
	for _, a := range byIP {
		if resolve != nil {
			if addr, err := netip.ParseAddr(a.loc.IP); err == nil {
				city, asn := resolve(addr)
				if city.ISOCode != "" {
					a.loc.ISOCode = city.ISOCode
				}
				a.loc.Country = city.CountryName
				a.loc.City = city.City
				a.loc.Subdivision = city.Subdivision
				if city.HasCoords {
					a.loc.Latitude = city.Latitude
					a.loc.Longitude = city.Longitude
					a.loc.HasCoords = true
				}
				if asn.Number != 0 {
					a.loc.ASN = formatASN(asn, a.loc.ASN)
				}
			}
		}
		a.loc.RelatedTx = sortedKeys(a.txSet)
		a.loc.RelatedWallets = sortedKeys(a.walletSet)
		locations = append(locations, a.loc)
	}

	// Deterministic order: observations desc, then IP asc.
	sort.Slice(locations, func(i, j int) bool {
		if locations[i].Observations != locations[j].Observations {
			return locations[i].Observations > locations[j].Observations
		}
		return locations[i].IP < locations[j].IP
	})

	return Result{
		Installed:         installed,
		HasCity:           hasCity,
		Locations:         locations,
		Clusters:          clusterPoints(locations, gridDeg),
		Countries:         rollupCountries(locations),
		TotalObservations: total,
		DistinctIPs:       len(locations),
	}
}

// clusterPoints bins coordinate-bearing locations into a coarse lat/lon grid and
// returns one Cluster per non-empty cell, as an observation-weighted centroid.
// Points without coordinates are omitted from clusters (they still appear in
// Locations and the country rollup) — never plotted at a fabricated position.
func clusterPoints(locs []IPLocation, gridDeg float64) []Cluster {
	type cell struct {
		sumLatW, sumLonW float64
		obs, ips         int
		members          []IPLocation
	}
	cells := map[[2]int]*cell{}
	for _, l := range locs {
		if !l.HasCoords {
			continue
		}
		key := [2]int{int(l.Latitude / gridDeg), int(l.Longitude / gridDeg)}
		c := cells[key]
		if c == nil {
			c = &cell{}
			cells[key] = c
		}
		w := float64(l.Observations)
		if w <= 0 {
			w = 1
		}
		c.sumLatW += l.Latitude * w
		c.sumLonW += l.Longitude * w
		c.obs += l.Observations
		c.ips++
		c.members = append(c.members, l)
	}
	out := make([]Cluster, 0, len(cells))
	for _, c := range cells {
		wsum := float64(c.obs)
		if wsum <= 0 {
			wsum = float64(c.ips)
		}
		sort.Slice(c.members, func(i, j int) bool {
			if c.members[i].Observations != c.members[j].Observations {
				return c.members[i].Observations > c.members[j].Observations
			}
			return c.members[i].IP < c.members[j].IP
		})
		out = append(out, Cluster{
			Lat:          c.sumLatW / wsum,
			Lon:          c.sumLonW / wsum,
			Observations: c.obs,
			IPs:          c.ips,
			Members:      c.members,
		})
	}
	// Deterministic: observations desc, then lat, then lon.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Observations != out[j].Observations {
			return out[i].Observations > out[j].Observations
		}
		if out[i].Lat != out[j].Lat {
			return out[i].Lat < out[j].Lat
		}
		return out[i].Lon < out[j].Lon
	})
	return out
}

// rollupCountries aggregates per-country observation + distinct-IP counts for
// the dashboard GEO panel. IPs with no country label fall into "(unknown)".
func rollupCountries(locs []IPLocation) []CountryActivity {
	type ca struct {
		name string
		obs  int
		ips  int
	}
	byISO := map[string]*ca{}
	for _, l := range locs {
		iso := l.ISOCode
		if strings.TrimSpace(iso) == "" {
			iso = "(unknown)"
		}
		c := byISO[iso]
		if c == nil {
			c = &ca{name: l.Country}
			byISO[iso] = c
		}
		if c.name == "" {
			c.name = l.Country
		}
		c.obs += l.Observations
		c.ips++
	}
	out := make([]CountryActivity, 0, len(byISO))
	for iso, c := range byISO {
		out = append(out, CountryActivity{ISOCode: iso, Country: c.name, Observations: c.obs, IPs: c.ips})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Observations != out[j].Observations {
			return out[i].Observations > out[j].Observations
		}
		return out[i].ISOCode < out[j].ISOCode
	})
	return out
}

// relatedWallets extracts wallet-ish subjects correlated to an observation.
// NetworkObservation carries only TxID directly; wallet correlation is done at
// the graph layer, so today this is empty. The seam is kept so a future
// enrichment can thread wallet subjects without changing callers.
func relatedWallets(o schema.NetworkObservation) []string { return nil }

// formatASN renders an ASN result as "AS<n> <org>", falling back to the
// observation's own ASN label when the DB has no ASN record.
func formatASN(a geoip.ASNResult, fallback string) string {
	if a.Number == 0 {
		return fallback
	}
	s := "AS" + itoa(int(a.Number))
	if strings.TrimSpace(a.Org) != "" {
		s += " " + a.Org
	}
	return s
}

func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// itoa is a tiny dependency-free int formatter (keeps the leaf lean).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
