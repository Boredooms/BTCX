// Package json renders a report view to deterministic, machine-readable JSON.
// It is NETWORK-FORBIDDEN like its parent: it only serializes an already-built
// view and never touches the repository or the network.
package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Render serializes an arbitrary report view to deterministic JSON. The two
// version markers (schema_version, generator_version) are emitted as the first
// keys for quick, cheap contract checks; every other object key is sorted
// recursively so the output is byte-stable across runs and builds. HTML
// escaping is disabled so hashes over the bytes are reproducible.
//
// schemaVersion and generatorVersion are passed explicitly (rather than read
// out of the view) so the ordering guarantee holds regardless of the view's Go
// field order.
func Render(view any, schemaVersion, generatorVersion string) ([]byte, error) {
	generic, err := toGeneric(view)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteByte('{')

	// Version markers first, in a fixed order.
	writeKey(&out, "schema_version")
	writeScalar(&out, schemaVersion)
	out.WriteByte(',')
	writeKey(&out, "generator_version")
	writeScalar(&out, generatorVersion)

	// Remaining top-level keys in sorted order, skipping any duplicate version
	// markers so they are never emitted twice.
	if m, ok := generic.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			if k == "schema_version" || k == "generator_version" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.WriteByte(',')
			writeKey(&out, k)
			if err := writeCanonical(&out, m[k]); err != nil {
				return nil, err
			}
		}
	}

	out.WriteByte('}')
	// Trailing newline keeps the file POSIX-friendly and is part of the stable
	// byte output.
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// toGeneric round-trips v through encoding/json into a generic structure using
// json.Number so numeric literals are preserved exactly (no float reformatting).
func toGeneric(v any) (any, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("json render: marshal: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("json render: decode: %w", err)
	}
	return generic, nil
}

// writeCanonical emits v with recursively sorted object keys and no
// insignificant whitespace.
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
			writeKey(buf, k)
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
		return writeScalar(buf, t)
	}
	return nil
}

// writeKey writes a JSON object key followed by ':'.
func writeKey(buf *bytes.Buffer, k string) {
	kb, _ := json.Marshal(k)
	buf.Write(kb)
	buf.WriteByte(':')
}

// writeScalar writes a scalar value with HTML escaping disabled.
func writeScalar(buf *bytes.Buffer, v any) error {
	var sb bytes.Buffer
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	buf.Write(bytes.TrimRight(sb.Bytes(), "\n"))
	return nil
}
