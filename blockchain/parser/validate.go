// Package parser provides streaming ingestion parsers and shared validation for
// CSV, JSON, NDJSON and XML sources. All parsing is local and streaming; no
// network access occurs.
package parser

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// ValidationError describes a rejected field with a bounded source fragment.
type ValidationError struct {
	RecordNo int    `json:"record_no"`
	Field    string `json:"field"`
	Reason   string `json:"reason"`
	Fragment string `json:"fragment,omitempty"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("record %d field %q: %s", e.RecordNo, e.Field, e.Reason)
}

// maxFragment bounds how much raw source is retained in a diagnostic.
const maxFragment = 160

// Fragment truncates s to a bounded, log-safe diagnostic.
func Fragment(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxFragment {
		return s[:maxFragment] + "…"
	}
	return s
}

// ValidIP accepts IPv4 and IPv6. Empty is treated as "not supplied" by callers.
func ValidIP(s string) bool {
	return net.ParseIP(strings.TrimSpace(s)) != nil
}

// ValidPort accepts 0..65535.
func ValidPort(p int) bool { return p >= 0 && p <= 65535 }

// ValidTxID accepts a non-empty id. BCTX datasets use synthetic ids (e.g. TXnnn)
// and real datasets use 64-hex; we accept both and only reject empties here.
func ValidTxID(s string) bool { return strings.TrimSpace(s) != "" }

// ParseTimestamp accepts RFC3339 and a few common fallbacks.
func ParseTimestamp(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05",
		"2006-01-02 15:04:05", "2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	// Unix seconds fallback.
	if len(s) >= 10 {
		if secs, err := parseUnix(s); err == nil {
			return time.Unix(secs, 0).UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp %q", s)
}

func parseUnix(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
