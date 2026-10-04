// Package mapdata is a STRICTLY-OFFLINE leaf: it reads the installed world
// geometry asset (world-110m.asset) read-only and returns coastline polylines
// plus a derived country-centroid table. It NEVER downloads, dials, or reaches
// acquisition/monitoring, and imports no net/* package.
//
// The asset is produced out-of-band at release time by tools/mapgen (via
// scripts/build_mapdata.sh), installed under ~/.bctx/mapdata/ by the CLI
// (`bctx map install`), and checksummed in a registry. A missing or
// sha256-mismatched asset degrades to a label-only country list with the
// documented message — never a crash and never a fabricated coastline.
package mapdata

import "errors"

// ErrNotInstalled is returned when no world geometry asset is installed.
var ErrNotInstalled = errors.New("world geometry asset not installed")

// ErrCorruptRegistry is returned when registry.json exists but cannot be parsed.
var ErrCorruptRegistry = errors.New("mapdata registry is corrupt")

// ErrCorruptAsset is returned when the asset file exists but cannot be parsed.
var ErrCorruptAsset = errors.New("world geometry asset is corrupt")

// ErrChecksumMismatch is returned when the installed asset's recomputed sha256
// does not match the registry. Treated the same as missing by the renderer:
// degrade + warn, never render a partial/corrupt coastline.
var ErrChecksumMismatch = errors.New("world geometry asset sha256 does not match registry")

// NotInstalledMessage is the exact, documented message shown when the asset is
// missing or fails integrity. It must never be replaced by a crash or a
// fabricated coastline.
const NotInstalledMessage = "WORLD GEOMETRY ASSET NOT INSTALLED — install with: bctx map install <file>"
