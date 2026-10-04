package screens

import (
	"fmt"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/geoip"
)

// status.go computes the honest live status strings the TopBar and Dashboard
// display (design §8). None of these values is ever hard-coded: MODELS comes
// from the ml/inference registry built with app.ModelsDir(cfg), GEOIP from the
// local geoip registry. The hard-coded "PENDING" the legacy home used is gone.

// Model status presentation constants (design §8).
const (
	modelsLoaded         = "LOADED"
	modelsMissing        = "MISSING"
	modelsSchemaMismatch = "SCHEMA-MISMATCH"
)

// ModelsStatus loads the two fail-closed models the orchestrator requires
// (anomaly + flow) through a registry rooted at app.ModelsDir(cfg) and reports
// the aggregate state. A clean load of both -> LOADED; a feature-schema hash
// mismatch -> SCHEMA-MISMATCH; anything else (missing dir, missing manifest,
// unreadable trees) -> MISSING. The registry reads local files only; it opens
// no socket.
func ModelsStatus(cfg *configs.Config) string {
	if cfg == nil {
		return modelsMissing
	}
	reg := app.NewRegistry(app.ModelsDir(cfg))
	defer reg.Close()
	for _, name := range []string{"anomaly", "flow"} {
		if _, err := reg.Get(name); err != nil {
			// A feature-schema hash mismatch is a distinct, honest state.
			if isSchemaMismatch(err) {
				return modelsSchemaMismatch
			}
			return modelsMissing
		}
	}
	return modelsLoaded
}

// isSchemaMismatch reports whether a registry load error is specifically a
// feature-schema hash mismatch (vs a missing/unreadable model).
func isSchemaMismatch(err error) bool {
	if err == nil {
		return false
	}
	return containsFold(err.Error(), "feature schema hash mismatch")
}

// GeoIPStatus reports the installed GeoIP database version, or "NOT INSTALLED"
// when none is present. It reads the local ~/.bctx/geoip registry only; it
// never downloads (the geoip package is NETWORK-FORBIDDEN, design §15).
func GeoIPStatus() string {
	dir, err := geoip.Dir()
	if err != nil {
		return "NOT INSTALLED"
	}
	reg, err := geoip.LoadRegistry(dir)
	if err != nil || len(reg.Entries) == 0 {
		return "NOT INSTALLED"
	}
	// Prefer the country DB version for the badge; fall back to any entry.
	if e, ok := reg.Entries["country"]; ok && e.Version != "" {
		return "v" + e.Version
	}
	for _, e := range reg.Entries {
		if e.Version != "" {
			return "v" + e.Version
		}
		return "INSTALLED"
	}
	return "NOT INSTALLED"
}

// containsFold is a small, dependency-free case-insensitive substring check.
func containsFold(s, sub string) bool {
	ls, lsub := toLowerASCII(s), toLowerASCII(sub)
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

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// Compile-time reference so the schema import stays if later trimmed.
var _ = schema.FeatureSchemaSHA256
var _ = fmt.Sprintf
