package geoip

import (
	"encoding/binary"
	"math"
	"os"
	"testing"
)

// This file builds a *tiny but structurally valid* MaxMind DB (.mmdb) in memory
// so the tests exercise the real maxminddb reader against a real file — not a
// mock. It implements just enough of the MaxMind DB binary format
// (https://maxmind.github.io/MaxMind-DB/) to encode a country record and the
// required metadata. It is TEST-ONLY and never compiled into the binary.
//
// Layout produced:
//
//	[ search tree ][ 16-byte separator ][ data section ][ metadata marker ][ metadata ]
//
// The search tree has a single node (node_count = 1, record_size = 32,
// ip_version = 4) whose left and right records both point at the one data
// record, so every IPv4 address resolves to the same country.

const (
	mmTypePointer = 1
	mmTypeString  = 2
	mmTypeDouble  = 3
	mmTypeUint16  = 5
	mmTypeUint32  = 6
	mmTypeMap     = 7
	mmTypeArray   = 11
)

// encCtrl writes a control byte (plus extended-type and extended-size bytes as
// needed) for a value of the given MaxMind DB type and element size. Types < 8
// are encoded in the top 3 bits of the control byte; types >= 8 use the
// extended form (top 3 bits = 0, then a byte holding type-7).
func encCtrl(typ byte, size int) []byte {
	var sizeBytes []byte
	low := byte(size)
	if size < 29 {
		low = byte(size)
	} else if size < 29+256 {
		low = 29
		sizeBytes = []byte{byte(size - 29)}
	} else {
		low = 30
		s := size - (29 + 256)
		sizeBytes = []byte{byte(s >> 8), byte(s)}
	}

	var out []byte
	if typ < 8 {
		out = append(out, (typ<<5)|low)
	} else {
		out = append(out, (0<<5)|low) // extended type marker
		out = append(out, typ-7)
	}
	return append(out, sizeBytes...)
}

func encString(s string) []byte {
	out := encCtrl(mmTypeString, len(s))
	return append(out, []byte(s)...)
}

func encUint16(v uint16) []byte {
	// Encode minimal bytes.
	var b []byte
	if v>>8 != 0 {
		b = []byte{byte(v >> 8), byte(v)}
	} else if v != 0 {
		b = []byte{byte(v)}
	}
	out := encCtrl(mmTypeUint16, len(b))
	return append(out, b...)
}

func encUint32(v uint32) []byte {
	var b []byte
	switch {
	case v>>24 != 0:
		b = []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
	case v>>16 != 0:
		b = []byte{byte(v >> 16), byte(v >> 8), byte(v)}
	case v>>8 != 0:
		b = []byte{byte(v >> 8), byte(v)}
	case v != 0:
		b = []byte{byte(v)}
	}
	out := encCtrl(mmTypeUint32, len(b))
	return append(out, b...)
}

func encMapHeader(n int) []byte { return encCtrl(mmTypeMap, n) }

// buildCountryMMDB returns the bytes of a minimal valid country .mmdb.
func buildCountryMMDB() []byte {
	// ---- data section ----
	// record = { "country": { "iso_code": "US", "names": { "en": "United States" } } }
	names := append(encMapHeader(1), encString("en")...)
	names = append(names, encString("United States")...)

	country := append(encMapHeader(2), encString("iso_code")...)
	country = append(country, encString("US")...)
	country = append(country, encString("names")...)
	country = append(country, names...)

	record := append(encMapHeader(1), encString("country")...)
	record = append(record, country...)

	// ---- metadata section ----
	meta := encMapHeader(9)
	meta = append(meta, encString("node_count")...)
	meta = append(meta, encUint32(1)...)
	meta = append(meta, encString("record_size")...)
	meta = append(meta, encUint16(32)...)
	meta = append(meta, encString("ip_version")...)
	meta = append(meta, encUint16(4)...)
	meta = append(meta, encString("database_type")...)
	meta = append(meta, encString("DBIP-Country-Lite-Fixture")...)
	meta = append(meta, encString("languages")...)
	meta = append(meta, encCtrl(mmTypeArray, 1)...) // array of 1
	meta = append(meta, encString("en")...)
	meta = append(meta, encString("binary_format_major_version")...)
	meta = append(meta, encUint16(2)...)
	meta = append(meta, encString("binary_format_minor_version")...)
	meta = append(meta, encUint16(0)...)
	meta = append(meta, encString("build_epoch")...)
	meta = append(meta, encUint32(0)...)
	meta = append(meta, encString("description")...)
	meta = append(meta, encMapHeader(1)...)
	meta = append(meta, encString("en")...)
	meta = append(meta, encString("BCTX test fixture")...)

	nodeCount := uint(1)
	recordSize := uint(32)
	nodeBytes := recordSize * 2 / 8 // 8 bytes per node
	searchTreeSize := uint(nodeCount) * nodeBytes

	// data pointer value: resolved = pointer - nodeCount - 16 == 0 (first byte
	// of data section). So pointer = nodeCount + 16.
	ptr := uint32(nodeCount + 16)

	// node 0: left=right=ptr (32-bit big-endian each)
	tree := make([]byte, 0, searchTreeSize)
	tree = append(tree, byte(ptr>>24), byte(ptr>>16), byte(ptr>>8), byte(ptr))
	tree = append(tree, byte(ptr>>24), byte(ptr>>16), byte(ptr>>8), byte(ptr))

	var out []byte
	out = append(out, tree...)
	out = append(out, make([]byte, 16)...) // data section separator
	out = append(out, record...)
	out = append(out, []byte("\xAB\xCD\xEFMaxMind.com")...)
	out = append(out, meta...)
	return out
}

func writeFixtureCountryMMDB(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, buildCountryMMDB(), 0o644); err != nil {
		t.Fatalf("write fixture mmdb: %v", err)
	}
}

// encDouble encodes an IEEE-754 float64 as the MaxMind "double" type (type 3,
// always 8 bytes, big-endian).
func encDouble(v float64) []byte {
	out := encCtrl(mmTypeDouble, 8)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(v))
	return append(out, b[:]...)
}

// buildCityMMDB returns the bytes of a minimal valid *City* edition .mmdb. The
// single data record carries country{iso_code,names.en}, city{names.en},
// location{latitude,longitude} and one subdivision, so LookupCity can assert
// coordinates + city + subdivision come straight out of a real maxminddb read.
// The metadata database_type is "GeoLite2-City" so isCityType detects it.
func buildCityMMDB(iso, countryName, cityName, subdivision string, lat, lon float64) []byte {
	// ---- data section ----
	countryNames := append(encMapHeader(1), encString("en")...)
	countryNames = append(countryNames, encString(countryName)...)
	country := append(encMapHeader(2), encString("iso_code")...)
	country = append(country, encString(iso)...)
	country = append(country, encString("names")...)
	country = append(country, countryNames...)

	cityNames := append(encMapHeader(1), encString("en")...)
	cityNames = append(cityNames, encString(cityName)...)
	city := append(encMapHeader(1), encString("names")...)
	city = append(city, cityNames...)

	location := append(encMapHeader(2), encString("latitude")...)
	location = append(location, encDouble(lat)...)
	location = append(location, encString("longitude")...)
	location = append(location, encDouble(lon)...)

	subNames := append(encMapHeader(1), encString("en")...)
	subNames = append(subNames, encString(subdivision)...)
	subEntry := append(encMapHeader(1), encString("names")...)
	subEntry = append(subEntry, subNames...)
	subs := append(encCtrl(mmTypeArray, 1), subEntry...)

	record := append(encMapHeader(4), encString("city")...)
	record = append(record, city...)
	record = append(record, encString("country")...)
	record = append(record, country...)
	record = append(record, encString("location")...)
	record = append(record, location...)
	record = append(record, encString("subdivisions")...)
	record = append(record, subs...)

	// ---- metadata section ----
	meta := encMapHeader(9)
	meta = append(meta, encString("node_count")...)
	meta = append(meta, encUint32(1)...)
	meta = append(meta, encString("record_size")...)
	meta = append(meta, encUint16(32)...)
	meta = append(meta, encString("ip_version")...)
	meta = append(meta, encUint16(4)...)
	meta = append(meta, encString("database_type")...)
	meta = append(meta, encString("GeoLite2-City")...)
	meta = append(meta, encString("languages")...)
	meta = append(meta, encCtrl(mmTypeArray, 1)...)
	meta = append(meta, encString("en")...)
	meta = append(meta, encString("binary_format_major_version")...)
	meta = append(meta, encUint16(2)...)
	meta = append(meta, encString("binary_format_minor_version")...)
	meta = append(meta, encUint16(0)...)
	meta = append(meta, encString("build_epoch")...)
	meta = append(meta, encUint32(0)...)
	meta = append(meta, encString("description")...)
	meta = append(meta, encMapHeader(1)...)
	meta = append(meta, encString("en")...)
	meta = append(meta, encString("BCTX test city fixture")...)

	nodeCount := uint(1)
	recordSize := uint(32)
	nodeBytes := recordSize * 2 / 8
	searchTreeSize := uint(nodeCount) * nodeBytes
	ptr := uint32(nodeCount + 16)

	tree := make([]byte, 0, searchTreeSize)
	tree = append(tree, byte(ptr>>24), byte(ptr>>16), byte(ptr>>8), byte(ptr))
	tree = append(tree, byte(ptr>>24), byte(ptr>>16), byte(ptr>>8), byte(ptr))

	var out []byte
	out = append(out, tree...)
	out = append(out, make([]byte, 16)...)
	out = append(out, record...)
	out = append(out, []byte("\xAB\xCD\xEFMaxMind.com")...)
	out = append(out, meta...)
	return out
}

// writeFixtureCityMMDB writes a City fixture whose entire IPv4 space resolves to
// East Finchley, GB (51.5967, -0.1593) — matching the real GeoLite2-City record
// for 81.2.69.142 so the fixture mirrors production shape.
func writeFixtureCityMMDB(t *testing.T, path string) {
	t.Helper()
	b := buildCityMMDB("GB", "United Kingdom", "East Finchley", "England", 51.5967, -0.1593)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write city fixture mmdb: %v", err)
	}
}
