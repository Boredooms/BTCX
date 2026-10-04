# BCTX Phase 8 — Offline World Geometry (Map) Runtime

The map runtime supplies the coastline polylines and country centroids the Geo
Map screen rasterizes. Like GeoIP it is a **strictly-offline leaf**
(`tui/mapdata`): it dials nothing, downloads nothing, and reaches no acquisition
or monitoring code. BCTX never fetches tiles or geometry at runtime — the asset
is built once at release time and installed locally; rendering reads only that
installed file.

Grounded in `tui/mapdata/{geometry,registry,errors}.go`, `cli/commands/map.go`,
`tools/mapgen/main.go`, and `scripts/build_mapdata.sh`. The no-network property
is enforced by `tests/offline/boundary_test.go:TestTUIPackagesHaveNoHTTPTransport`
(the strictly-offline leaf branch); the renderer in `tui/components/mapview.go`
is an in-repo Braille/half-block rasterizer with no remote-tile dependency.

---

## 1. The asset

A single file, `world-110m.asset`, is a line-oriented text format: one record
per line, `line\t{json}` for a coastline polyline and `centroid\t{json}` for a
derived country centroid. It carries **both** the coastline geometry and the
country-centroid table; the centroid table is a deterministic derivative of the
Natural Earth admin-0 polygons produced in the same build step, so it has no
independent provenance and is covered by the asset's single sha256.

| name | source | version | license | install path |
|------|--------|---------|---------|--------------|
| world-110m | Natural Earth 1:110m admin-0 / coastline | natural-earth-110m | public domain | `~/.bctx/mapdata/world-110m.asset` |

---

## 2. Install location and registry

The asset installs under `~/.bctx/mapdata/` (resolved by `mapdata.Dir()`). The
registry at `~/.bctx/mapdata/registry.json` mirrors the geoip registry shape and
holds **no secrets**:

```
{ name ("world-110m"), source, source_url, version, license,
  installed_at, sha256, record_count (centroid count), path }
```

A single entry; one sha256 covers both the coastline polylines and the derived
centroids.

---

## 3. Lifecycle (`bctx map …`, fully offline)

| Verb | Behavior |
|------|----------|
| `map status` | Prints the installed asset's provenance + centroid count + sha256 + path, or the honest `WORLD GEOMETRY ASSET NOT INSTALLED — install with: bctx map install <file>` with the install path. |
| `map install <file.asset>` | Copies the asset into `~/.bctx/mapdata/`, computes its sha256 and centroid count, and writes the registry atomically. Provenance defaults to the Natural Earth public-domain metadata (overridable via flags). |
| `map verify [file.asset]` | Recomputes the sha256 of the installed asset (no argument) or a given file and compares it to the registry: `VERIFY OK`, `VERIFY FAILED` on mismatch, or the not-installed message. |

---

## 4. Build-time generation (release only)

`scripts/build_mapdata.sh` → `tools/mapgen` reads a local Natural Earth 1:110m
admin-0 GeoJSON and emits `world-110m.asset` **deterministically**: fixed
5-decimal float formatting and a stable record order (coastlines in input order,
centroids sorted by ISO code), so the same source + generator always yields an
identical sha256. The representative point per country is the vertex-centroid of
its largest ring; `-99` ISO entities contribute coastlines but no centroid.

`tools/mapgen` is a build tool and is **not** imported by any `tui/*` package, so
it never enters the TUI import closure. The asset is not built at runtime and is
not required for the build/test gate — tests use tiny fixture assets.

---

## 5. Honest degrade (AGENTS §16/§19)

- No asset installed → `ErrNotInstalled`, surfaced as the documented install
  message; the Geo Map screen degrades to a label-only country list with no
  plotted coastline.
- sha256 mismatch → `ErrChecksumMismatch`, treated like missing — never a
  partial or fabricated coastline.

The map plots **observation metadata** (country/ASN of network observations); it
never implies ownership or the physical location of a person (design §9; AGENTS
§18).

---

## 6. Interactive viewport (Geo Map screen)

When the Geo Map screen holds Body focus (press `Tab` to move focus from the
SideNav to the body), the plotted viewport is pannable and zoomable. Keys are
owned by `tui/components/mapview.go` and advertised in the screen's help:

| Key(s) | Action |
|---|---|
| arrows / `h` `j` `k` `l` | pan the viewport (step scales with zoom) |
| `+` / `=` | zoom in (clamped to ×16) |
| `-` / `_` | zoom out (clamped to ×1) |
| `0` | reset to the home view (center 0°/20°N, zoom ×1, selection cleared) |
| `n` / `N` | select next / previous cluster (recenters on it) |
| `Enter` | open/confirm the selection detail panel (does **not** navigate) |
| `g` | "go" — cross-link to the selected cluster's related tx, else its IP |
| `c` | toggle the density bin between country and ASN (fallback list) |

`Tab` is deliberately **not** a map key — it is the global Nav↔Body focus cycle,
consumed before the map sees it. `g` is handled only on this screen (it is not a
global binding), so table-based screens keep `g` = "jump to top."

### Viewport / region indicator

The plotted header is an honest status line:

```
IP geolocation (estimate) · zoom x4 · center 0.2°W, 51.6°N · <region> · N clusters · M IPs
```

Center longitude/latitude are shown with hemisphere letters (E/W, N/S) derived
from the signed center. The `<region>` segment is the nearest country centroid
to the current center, looked up over the asset's small centroid table; it is
**omitted honestly** when no geometry/centroids are installed. Under width
pressure the header sheds segments left-to-right in a fixed order — region
first, then the center coordinates — so the zoom factor and the cluster/IP
counts always survive.

### Coarse-at-high-zoom honesty

The shipped asset is Natural Earth **1:110m** (`world-110m`): a world-scale
coastline. Zooming in does not add detail the asset does not contain, so at high
zoom the coastline is intentionally coarse and the plot is a density/location
*estimate*, never a street-level or sub-country claim. This is consistent with
the metadata-only framing: the map answers "roughly where were these
observations seen," not "where is this person."

A finer, bounded region asset is **not shipped** in this change. The documented,
offline, release-time path to generate one is `scripts/build_mapdata.sh` →
`tools/mapgen` (§4): it emits a deterministic, checksummed `.asset` from a local
Natural Earth source, installed with `bctx map install`. Nothing is fetched at
runtime; the renderer only ever reads an installed, sha256-verified file.
