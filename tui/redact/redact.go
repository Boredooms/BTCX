// Package redact is the TUI's defensive secret-redaction seam (design §12
// invariant 5; AGENTS §21: never log or display credentials, API keys, or
// token-bearing endpoints). It is a pure, leaf string-transformation package
// imported by both the tui package (TopBar chrome) and the tui/screens package
// (the Settings screen's config rows) WITHOUT creating an import cycle (screens
// must not import tui). It performs no network, SQL, or stateful work.
//
// The TUI already avoids rendering sensitive config by design — the Settings
// screen shows a provider NAME, not its endpoint — so this is a
// belt-and-suspenders guard on the few presentation strings that originate from
// user-controlled configuration. When a value looks like it carries a secret
// (URL userinfo credentials, a token/key/secret/password query parameter), the
// secret portion is replaced with a fixed marker while keeping enough structure
// (scheme/host) to stay useful. A value with no secret shape is returned
// unchanged.
package redact

import (
	"regexp"
	"strings"
)

// Marker is the fixed replacement text for any redacted secret. Tests assert a
// planted secret never appears in view output; the marker may.
const Marker = "***REDACTED***"

// urlUserinfo matches the "user:password@" credential segment of a URL
// authority (scheme://user:pass@host...). The whole userinfo is redacted to
// avoid leaking a username that is itself a token.
var urlUserinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^/@\s]+@`)

// sensitiveQueryParam matches a URL/query key=value pair whose key names a
// secret. The value up to the next & or whitespace is redacted.
var sensitiveQueryParam = regexp.MustCompile(
	`(?i)([?&;](?:api[_-]?key|access[_-]?token|auth[_-]?token|auth|token|secret|password|passwd|pwd|sig|signature|key)=)[^&\s;]+`)

// sensitiveKeyNames are configuration field names that must never have their
// VALUE displayed. IsSensitiveKey lets a presenter show "(set)" instead of the
// value. Matching is case-insensitive and substring-based.
var sensitiveKeyNames = []string{
	"password", "passwd", "pwd", "secret", "token", "apikey", "api_key",
	"access_key", "accesskey", "private_key", "privatekey", "credential",
	"auth", "signature",
}

// String returns s with any secret-shaped substring replaced by Marker. It
// handles URL userinfo credentials and sensitive query parameters. A value with
// no secret shape is returned unchanged. String is idempotent.
func String(s string) string {
	if s == "" {
		return s
	}
	out := urlUserinfo.ReplaceAllString(s, "$1"+Marker+"@")
	out = sensitiveQueryParam.ReplaceAllString(out, "$1"+Marker)
	return out
}

// IsSensitiveKey reports whether a configuration key name denotes a secret whose
// value must not be displayed.
func IsSensitiveKey(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, k := range sensitiveKeyNames {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// Field returns a non-revealing presentation value for a (key, value) pair:
// "(set)" when the key is sensitive and the value is non-empty, "(none)" when
// empty, otherwise the value passed through String for incidental in-value
// secrets. This is the single helper a presenter should use when it must show a
// config field whose value might be sensitive.
func Field(key, value string) string {
	if IsSensitiveKey(key) {
		if strings.TrimSpace(value) == "" {
			return "(none)"
		}
		return "(set)"
	}
	return String(value)
}
