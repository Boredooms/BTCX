// Command mapgen is the one-time, offline, release-time generator for the BCTX
// world geometry asset (world-110m.asset). It reads a LOCAL Natural Earth
// 1:110m admin-0 / coastline source (public domain) in GeoJSON form and emits a
// deterministic line-oriented asset: coastline polylines plus a derived
// representative-point centroid per country. Determinism (same source + same
// generator version -> identical bytes -> identical sha256) is guaranteed by a
// stable record order and fixed float formatting.
//
// mapgen is a BUILD TOOL, not a runtime dependency: it is NOT imported by any
// tui/* package and dials nothing. It only reads a local file and writes a local
// file. scripts/build_mapdata.sh invokes it at release time.
//
// Usage:
//
//	mapgen -in natural-earth-110m.geojson -out world-110m.asset
//
// The asset format matches tui/mapdata (see geometry.go): one record per line,
// "line\t{json}" for coastline polylines and "centroid\t{json}" for centroids,
// coastlines in input order followed by centroids sorted by ISO code.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
)

// geojson is the minimal subset of the Natural Earth admin-0 GeoJSON schema we
// read. We support Polygon and MultiPolygon geometries; properties carry the
// ISO code and country name under the common Natural Earth keys.
type geojson struct {
	Features []feature `json:"features"`
}

type feature struct {
	Properties map[string]any  `json:"properties"`
	Geometry   geojsonGeometry `json:"geometry"`
}

type geojsonGeometry struct {
	Type string `json:"type"`
	// Coordinates is decoded lazily per geometry type.
	Coordinates json.RawMessage `json:"coordinates"`
}

type point struct {
	Lon, Lat float64
}

func main() {
	in := flag.String("in", "", "path to a local Natural Earth 1:110m admin-0 GeoJSON file")
	out := flag.String("out", "world-110m.asset", "path to write the generated asset")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "mapgen: -in <natural-earth-110m.geojson> is required")
		os.Exit(2)
	}
	if err := run(*in, *out); err != nil {
		fmt.Fprintf(os.Stderr, "mapgen: %v\n", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	var gj geojson
	if err := json.Unmarshal(raw, &gj); err != nil {
		return fmt.Errorf("parse source geojson: %w", err)
	}

	type centroidRec struct {
		iso, name string
		pt        point
	}

	var lines [][]point
	var centroids []centroidRec

	for _, f := range gj.Features {
		rings, err := extractRings(f.Geometry)
		if err != nil {
			return err
		}
		if len(rings) == 0 {
			continue
		}
		// Coastline polylines: emit each ring in input order.
		lines = append(lines, rings...)

		iso := stringProp(f.Properties, "ISO_A2", "iso_a2", "ISO_A2_EH", "WB_A2")
		name := stringProp(f.Properties, "ADMIN", "admin", "NAME", "name", "SOVEREIGNT")
		if iso == "" || iso == "-99" {
			// Natural Earth uses "-99" for entities without an ISO code; skip the
			// centroid (geometry still contributes to coastlines) to keep the
			// centroid table meaningful and deterministic.
			continue
		}
		centroids = append(centroids, centroidRec{
			iso:  iso,
			name: name,
			pt:   representativePoint(rings),
		})
	}

	// Stable order: coastlines already in input order; centroids sorted by ISO.
	sort.Slice(centroids, func(i, j int) bool {
		if centroids[i].iso != centroids[j].iso {
			return centroids[i].iso < centroids[j].iso
		}
		return centroids[i].name < centroids[j].name
	})

	tmp := outPath + ".tmp"
	fh, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(fh)

	for _, ring := range lines {
		pts := make([]map[string]json.RawMessage, 0, len(ring))
		for _, p := range ring {
			pts = append(pts, map[string]json.RawMessage{
				"lon": fixed(p.Lon),
				"lat": fixed(p.Lat),
			})
		}
		payload, err := json.Marshal(map[string]any{"points": pts})
		if err != nil {
			fh.Close()
			return err
		}
		if _, err := fmt.Fprintf(w, "line\t%s\n", payload); err != nil {
			fh.Close()
			return err
		}
	}
	for _, c := range centroids {
		payload, err := json.Marshal(map[string]any{
			"iso":  c.iso,
			"name": c.name,
			"point": map[string]json.RawMessage{
				"lon": fixed(c.pt.Lon),
				"lat": fixed(c.pt.Lat),
			},
		})
		if err != nil {
			fh.Close()
			return err
		}
		if _, err := fmt.Fprintf(w, "centroid\t%s\n", payload); err != nil {
			fh.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		fh.Close()
		return err
	}
	if err := fh.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, outPath); err != nil {
		return err
	}

	sum, err := sha256File(outPath)
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", outPath)
	fmt.Printf("coastlines: %d\n", len(lines))
	fmt.Printf("centroids:  %d\n", len(centroids))
	fmt.Printf("sha256:     %s\n", sum)
	return nil
}

// extractRings decodes Polygon / MultiPolygon coordinates into flat rings of
// points. Interior holes are included as separate polylines (they are still
// valid coastline/boundary strokes for a low-res render).
func extractRings(g geojsonGeometry) ([][]point, error) {
	switch g.Type {
	case "Polygon":
		var poly [][][]float64
		if err := json.Unmarshal(g.Coordinates, &poly); err != nil {
			return nil, fmt.Errorf("polygon coords: %w", err)
		}
		return ringsFromPolygon(poly), nil
	case "MultiPolygon":
		var multi [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &multi); err != nil {
			return nil, fmt.Errorf("multipolygon coords: %w", err)
		}
		var out [][]point
		for _, poly := range multi {
			out = append(out, ringsFromPolygon(poly)...)
		}
		return out, nil
	case "":
		return nil, nil
	default:
		return nil, nil
	}
}

func ringsFromPolygon(poly [][][]float64) [][]point {
	var out [][]point
	for _, ring := range poly {
		pts := make([]point, 0, len(ring))
		for _, c := range ring {
			if len(c) < 2 {
				continue
			}
			pts = append(pts, point{Lon: c[0], Lat: c[1]})
		}
		if len(pts) >= 2 {
			out = append(out, pts)
		}
	}
	return out
}

// representativePoint returns a deterministic representative point for a set of
// rings: the centroid of the vertices of the largest ring (by vertex count).
// This is a simple, reproducible derivative of the admin-0 polygons; it is not
// an independently sourced dataset.
func representativePoint(rings [][]point) point {
	best := -1
	bestLen := -1
	for i, r := range rings {
		if len(r) > bestLen {
			bestLen = len(r)
			best = i
		}
	}
	if best < 0 {
		return point{}
	}
	var sumLon, sumLat float64
	r := rings[best]
	for _, p := range r {
		sumLon += p.Lon
		sumLat += p.Lat
	}
	n := float64(len(r))
	return point{Lon: sumLon / n, Lat: sumLat / n}
}

// fixed formats a coordinate with a fixed 5-decimal precision as raw JSON so the
// output bytes are deterministic across runs and platforms (strconv with -1
// precision could vary; a fixed precision does not).
func fixed(v float64) json.RawMessage {
	return json.RawMessage(strconv.FormatFloat(v, 'f', 5, 64))
}

func stringProp(props map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := props[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func sha256File(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
