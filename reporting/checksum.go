package reporting

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/bctx/bctx/reporting/models"
)

// BytesSHA256 returns the hex-encoded SHA-256 of b.
func BytesSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// FileSHA256 returns the hex-encoded SHA-256 of the file at path. It is used by
// Verify to recompute bundle hashes fully offline.
func FileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return BytesSHA256(b), nil
}

// CanonicalJSON produces a deterministic, byte-stable JSON encoding of v: HTML
// escaping is disabled, all object keys are sorted recursively, and no extra
// whitespace is emitted. Two values that are deeply equal (ignoring Go map
// iteration order) always produce identical bytes, which is what makes the
// snapshot hash reproducible across runs and builds.
func CanonicalJSON(v any) ([]byte, error) {
	// First marshal with HTML escaping disabled.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("canonical json: marshal: %w", err)
	}

	// Decode into a generic structure using json.Number so numeric literals are
	// preserved exactly as written (no float reformatting).
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonical json: decode: %w", err)
	}

	var out bytes.Buffer
	if err := writeCanonical(&out, generic); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// writeCanonical emits v with sorted object keys and no insignificant
// whitespace.
func writeCanonical(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := writeCanonical(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		// Scalars (string, bool, json.Number, nil): re-marshal with HTML
		// escaping disabled for determinism.
		var sb bytes.Buffer
		enc := json.NewEncoder(&sb)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(t); err != nil {
			return err
		}
		// json.Encoder appends a newline; trim it.
		buf.Write(bytes.TrimRight(sb.Bytes(), "\n"))
	}
	return nil
}

// SnapshotHash returns the hex-encoded SHA-256 over the canonical JSON of the
// snapshot. The snapshot has no wall-clock field, so generated_at/generated_by
// are structurally absent from the hash input: an identical snapshot always
// yields the same hash regardless of when or by whom it was generated.
func SnapshotHash(snap models.ReportSnapshot) string {
	canon, err := CanonicalJSON(snap)
	if err != nil {
		// CanonicalJSON only fails if the snapshot cannot be JSON-encoded,
		// which cannot happen for this fixed struct; hash the error marker so
		// the failure is visible rather than silently producing a zero hash.
		sum := sha256.Sum256([]byte("snapshot-hash-error:" + err.Error()))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(canon)
	return hex.EncodeToString(sum[:])
}
