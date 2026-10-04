package geoip

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/oschwald/maxminddb-golang"
)

// CountryResult is the country answer for a single IP: an ISO 3166-1 alpha-2
// code and the English display name, as carried by the DB-IP / GeoLite2 MMDB
// schema. Empty fields mean the DB had no record for that IP (not an error).
type CountryResult struct {
	ISOCode string
	Name    string
}

// ASNResult is the autonomous-system answer for a single IP.
type ASNResult struct {
	Number uint
	Org    string
}

// CityResult is the city-level answer for a single IP from a GeoLite2/DB-IP
// *City* edition MMDB: the country ISO+name, the city name, the top
// subdivision (region/state) name, and the approximate latitude/longitude the
// map renderer plots. Empty string / zero fields mean the DB carried no value
// for that IP (never fabricated). HasCoords distinguishes a genuine 0,0 (off
// the coast of Africa) from "no location record".
//
// Coordinates are a coarse, dataset-level ESTIMATE of where the IP is observed
// — never a claim about who owns the IP or the funds. The presentation layer
// labels them accordingly (design §13, AGENTS §18).
type CityResult struct {
	ISOCode     string
	CountryName string
	City        string
	Subdivision string
	Latitude    float64
	Longitude   float64
	HasCoords   bool
}

// DB is the strictly-offline GeoIP lookup contract consumed by the Geo Map
// screen (FEAT-004). It never downloads; it only reads the installed .mmdb
// files opened read-only via maxminddb.Open.
type DB interface {
	LookupCountry(ip netip.Addr) (CountryResult, error)
	LookupASN(ip netip.Addr) (ASNResult, error)
	// LookupCity resolves city/subdivision + latitude/longitude from an
	// installed *City* edition DB. When the installed country DB is NOT a City
	// edition (no location fields) it returns the country-level fields it does
	// have with HasCoords=false — honest degrade, never a fabricated coordinate.
	LookupCity(ip netip.Addr) (CityResult, error)
	// HasCity reports whether the installed DB is a City edition that can carry
	// latitude/longitude. Callers use it to decide between plotting points and
	// the country/ASN label-only fallback.
	HasCity() bool
	Status() RegistryEntry
	Close() error
}

// db is the maxminddb-backed implementation. Country and ASN live in separate
// MMDB files (DB-IP ships them separately), so db holds up to two readers plus
// the registry entries that describe them for Status().
type db struct {
	country     *maxminddb.Reader
	asn         *maxminddb.Reader
	countryMeta RegistryEntry
	asnMeta     RegistryEntry
	// hasCity is true when the country reader is a City-edition MMDB (its
	// metadata database_type contains "City"), so location fields are present.
	hasCity bool
}

// Open loads whichever installed databases are present under dir, read-only.
// If no database is installed at all it returns ErrNotInstalled so the caller
// shows the documented install message. Having only one of country/ASN is
// allowed; the missing kind simply returns empty results on lookup.
func Open(dir string) (DB, error) {
	reg, err := LoadRegistry(dir)
	if err != nil {
		return nil, err
	}
	if len(reg.Entries) == 0 {
		return nil, ErrNotInstalled
	}

	d := &db{}
	if e, ok := reg.Entries["country"]; ok {
		r, err := maxminddb.Open(e.Path)
		if err != nil {
			d.closePartial()
			return nil, fmt.Errorf("open country database %s: %w", e.Path, err)
		}
		d.country = r
		d.countryMeta = e
		// A City edition carries location.latitude/longitude + city/subdivision.
		// Detect it from the MMDB's own metadata so an installed City DB yields
		// coordinates regardless of the coarse registry db_type bucket ("country").
		d.hasCity = isCityType(r.Metadata.DatabaseType)
	}
	if e, ok := reg.Entries["asn"]; ok {
		r, err := maxminddb.Open(e.Path)
		if err != nil {
			d.closePartial()
			return nil, fmt.Errorf("open asn database %s: %w", e.Path, err)
		}
		d.asn = r
		d.asnMeta = e
	}
	if d.country == nil && d.asn == nil {
		return nil, ErrNotInstalled
	}
	return d, nil
}

// countryRecord mirrors the DB-IP / GeoLite2 "country" MMDB schema (only the
// fields BCTX needs).
type countryRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
}

// asnRecord mirrors the DB-IP / GeoLite2 "asn" MMDB schema.
type asnRecord struct {
	Number uint   `maxminddb:"autonomous_system_number"`
	Org    string `maxminddb:"autonomous_system_organization"`
}

// cityRecord mirrors the GeoLite2 / DB-IP *City* MMDB schema (only the fields
// BCTX needs). location.latitude/longitude are the approximate observation
// coordinates; the booleans are not provided by the format, so a missing
// location sub-map simply decodes to zero and HasCoords is derived in LookupCity
// from whether any location/city field was populated.
type cityRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Subdivisions []struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
}

func (d *db) LookupCountry(ip netip.Addr) (CountryResult, error) {
	if d.country == nil {
		return CountryResult{}, nil
	}
	var rec countryRecord
	if err := d.country.Lookup(toNetIP(ip), &rec); err != nil {
		return CountryResult{}, err
	}
	name := rec.Country.Names["en"]
	return CountryResult{ISOCode: rec.Country.ISOCode, Name: name}, nil
}

func (d *db) LookupASN(ip netip.Addr) (ASNResult, error) {
	if d.asn == nil {
		return ASNResult{}, nil
	}
	var rec asnRecord
	if err := d.asn.Lookup(toNetIP(ip), &rec); err != nil {
		return ASNResult{}, err
	}
	return ASNResult{Number: rec.Number, Org: rec.Org}, nil
}

// HasCity reports whether the installed country DB is a City edition carrying
// location fields. A non-City DB (plain country) returns false, so the caller
// uses the label-only fallback rather than ever synthesizing a coordinate.
func (d *db) HasCity() bool { return d.hasCity }

// LookupCity reads the City-edition record for ip. It is a pure value
// conversion — no network, no name resolution. On a non-City DB, or an IP the
// DB has no record for, it returns whatever country-level fields are present
// (possibly empty) with HasCoords=false; it never fabricates coordinates.
func (d *db) LookupCity(ip netip.Addr) (CityResult, error) {
	if d.country == nil {
		return CityResult{}, nil
	}
	var rec cityRecord
	if err := d.country.Lookup(toNetIP(ip), &rec); err != nil {
		return CityResult{}, err
	}
	res := CityResult{
		ISOCode:     rec.Country.ISOCode,
		CountryName: rec.Country.Names["en"],
		City:        rec.City.Names["en"],
		Latitude:    rec.Location.Latitude,
		Longitude:   rec.Location.Longitude,
	}
	if len(rec.Subdivisions) > 0 {
		res.Subdivision = rec.Subdivisions[0].Names["en"]
	}
	// HasCoords is true only for a City-edition DB that actually populated a
	// location sub-map for this IP. A plain country DB (no location fields) or
	// an IP with no record leaves lat/lon at zero and HasCoords false, so the
	// renderer never plots a fabricated (0,0) point.
	if d.hasCity && (rec.Location.Latitude != 0 || rec.Location.Longitude != 0 ||
		res.City != "" || res.Subdivision != "") {
		res.HasCoords = true
	}
	return res, nil
}

// Status returns the registry entry for the country DB if present, else the ASN
// DB entry, so a caller always sees provenance for at least one installed asset.
func (d *db) Status() RegistryEntry {
	if d.country != nil {
		return d.countryMeta
	}
	return d.asnMeta
}

func (d *db) Close() error {
	var first error
	if d.country != nil {
		if err := d.country.Close(); err != nil && first == nil {
			first = err
		}
	}
	if d.asn != nil {
		if err := d.asn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (d *db) closePartial() {
	if d.country != nil {
		_ = d.country.Close()
	}
	if d.asn != nil {
		_ = d.asn.Close()
	}
}

// toNetIP converts a netip.Addr (the TUI-facing type) to the net.IP the MMDB
// reader expects. This is a pure value conversion — no name resolution, no
// socket, no network.
func toNetIP(ip netip.Addr) net.IP {
	return net.IP(ip.AsSlice())
}

// inspectMMDB opens a candidate file read-only to read its metadata, returning
// the detected db_type ("country"/"asn") and node count. It is used at install
// time to populate the registry and to reject non-MMDB files early. No network.
func inspectMMDB(path string) (dbType string, recordCount int64, err error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("not a readable MMDB file: %w", err)
	}
	defer r.Close()
	recordCount = int64(r.Metadata.NodeCount)
	return classifyDBType(r.Metadata.DatabaseType), recordCount, nil
}

// classifyDBType maps the MMDB metadata database_type string to BCTX's coarse
// "country"/"asn" bucket. Unknown types default to "country" (the common case
// for DB-IP IP-to-Country Lite).
func classifyDBType(metaType string) string {
	if containsFold(metaType, "ASN") {
		return "asn"
	}
	return "country"
}

// isCityType reports whether an MMDB metadata database_type names a City
// edition (e.g. "GeoLite2-City", "DBIP-City-Lite"), which carries
// location.latitude/longitude + city/subdivision records. The coarse registry
// db_type bucket stays "country" for either edition; this metadata check is how
// an installed City DB is recognised as coordinate-capable.
func isCityType(metaType string) bool {
	return containsFold(metaType, "City")
}

func containsFold(s, sub string) bool {
	// small, dependency-free case-insensitive substring check
	ls, lsub := toLower(s), toLower(sub)
	if len(lsub) == 0 {
		return true
	}
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
