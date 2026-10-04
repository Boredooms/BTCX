// Package geoip is a STRICTLY-OFFLINE leaf: it opens a user-installed MaxMind
// DB (.mmdb) read-only and answers country/ASN lookups for a single IP. It
// NEVER downloads, dials, or reaches acquisition/monitoring. It imports no
// net/http, crypto/tls, or transport package; the only net dependency is the
// value-type IP parsing inside the MMDB reader (asserted offline-pure by the
// dependency gate and the offline-boundary guard test).
//
// The install/verify/version lifecycle is driven out-of-band by the CLI
// (`bctx geo ...`); this package only reads what is already installed under
// ~/.bctx/geoip/ and reports honest "not installed" / "corrupt" states rather
// than ever fabricating a result.
package geoip

import "errors"

// ErrNotInstalled is returned when no GeoIP database is installed. Callers
// surface the documented install message rather than crashing or faking data.
var ErrNotInstalled = errors.New("geoip database not installed")

// ErrCorruptRegistry is returned when the local registry.json exists but cannot
// be parsed or is internally inconsistent.
var ErrCorruptRegistry = errors.New("geoip registry is corrupt")

// ErrChecksumMismatch is returned by Verify when a file's recomputed sha256
// does not match the value recorded in the registry.
var ErrChecksumMismatch = errors.New("geoip database sha256 does not match registry")

// NotInstalledMessage is the exact, documented message shown to the user when a
// geoip-dependent action finds no installed database. It must never be replaced
// by a crash or fabricated lookup result.
const NotInstalledMessage = "GEOIP DATABASE NOT INSTALLED — install with: bctx geo install <file>"
