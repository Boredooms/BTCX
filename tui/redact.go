package tui

import "github.com/bctx/bctx/tui/redact"

// redact.go re-exports the pure tui/redact leaf helper into the tui package so
// the Root chrome (TopBar) can scrub user-controlled config strings before
// display (design §12 invariant 5; AGENTS §21). The real logic lives in
// tui/redact so the tui/screens package can reuse it without importing tui
// (which would create an import cycle).

// redactedMarker is the fixed replacement text for a redacted secret.
const redactedMarker = redact.Marker

// Redact returns s with any secret-shaped substring replaced by the marker. See
// redact.String.
func Redact(s string) string { return redact.String(s) }

// IsSensitiveKey reports whether a config key name denotes a secret value that
// must not be displayed. See redact.IsSensitiveKey.
func IsSensitiveKey(name string) bool { return redact.IsSensitiveKey(name) }

// RedactField returns a non-revealing presentation value for a (key, value)
// pair. See redact.Field.
func RedactField(key, value string) string { return redact.Field(key, value) }
