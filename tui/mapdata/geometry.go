package mapdata

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// Point is a lon/lat coordinate in degrees (WGS84), the native Natural Earth
// coordinate space. The renderer (FEAT-004) projects these to terminal cells.
type Point struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

// Polyline is one coastline / country-boundary stroke as an ordered list of
// points. The asset stores many of these.
type Polyline struct {
	Points []Point `json:"points"`
}

// Centroid is the representative point of one admin-0 country, derived
// deterministically from the same Natural Earth polygons. It is not an
// independently sourced dataset; it is covered by the single asset sha256.
type Centroid struct {
	ISOCode string `json:"iso"`
	Name    string `json:"name"`
	Point   Point  `json:"point"`
}

// Geometry is the loaded world asset: coastline polylines + the derived
// country-centroid table. It is read-only presentation data.
type Geometry struct {
	Coastlines []Polyline `json:"coastlines"`
	Centroids  []Centroid `json:"centroids"`
}

// Asset file format (world-110m.asset): a deterministic, line-oriented text
// format so the generator output is reproducible byte-for-byte (same source +
// generator version -> identical bytes -> identical sha256). Each record is one
// line: a type tag, a tab, then a compact JSON payload. Blank lines and lines
// beginning with '#' are ignored. The generator MUST emit records in a stable
// order (coastlines first in input order, then centroids sorted by ISO code).
//
//	# comment
//	line\t{"points":[{"lon":..,"lat":..},...]}
//	centroid\t{"iso":"US","name":"United States","point":{"lon":..,"lat":..}}
const (
	tagLine     = "line"
	tagCentroid = "centroid"
)

// LoadGeometry parses an asset file into a Geometry. It performs no network
// access and no checksum verification (callers that need integrity use the
// registry's Verify). A structurally invalid file returns ErrCorruptAsset.
func LoadGeometry(path string) (*Geometry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotInstalled
		}
		return nil, err
	}
	defer f.Close()

	g := &Geometry{}
	sc := bufio.NewScanner(f)
	// Allow long polyline lines (default token size is 64KiB).
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag, payload, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, ErrCorruptAsset
		}
		switch tag {
		case tagLine:
			var pl Polyline
			if err := json.Unmarshal([]byte(payload), &pl); err != nil {
				return nil, errors.Join(ErrCorruptAsset, err)
			}
			g.Coastlines = append(g.Coastlines, pl)
		case tagCentroid:
			var c Centroid
			if err := json.Unmarshal([]byte(payload), &c); err != nil {
				return nil, errors.Join(ErrCorruptAsset, err)
			}
			g.Centroids = append(g.Centroids, c)
		default:
			return nil, ErrCorruptAsset
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(g.Coastlines) == 0 && len(g.Centroids) == 0 {
		return nil, ErrCorruptAsset
	}
	return g, nil
}

// Load opens the installed asset in dir, verifies its sha256 against the
// registry, and returns the parsed Geometry. A missing asset or a sha256
// mismatch returns ErrNotInstalled / ErrChecksumMismatch respectively so the
// renderer degrades to the label-only country list — it never returns a partial
// coastline. On a mismatch the registry's recorded centroid names are still
// available via LoadRegistry for the degraded label list, but the geometry is
// deliberately withheld.
func Load(dir string) (*Geometry, RegistryEntry, error) {
	reg, err := LoadRegistry(dir)
	if err != nil {
		return nil, RegistryEntry{}, err
	}
	if reg.Entry.Name == "" {
		return nil, RegistryEntry{}, ErrNotInstalled
	}
	sum, err := SHA256File(reg.Entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, reg.Entry, ErrNotInstalled
		}
		return nil, reg.Entry, err
	}
	if sum != reg.Entry.SHA256 {
		return nil, reg.Entry, ErrChecksumMismatch
	}
	g, err := LoadGeometry(reg.Entry.Path)
	if err != nil {
		return nil, reg.Entry, err
	}
	return g, reg.Entry, nil
}
