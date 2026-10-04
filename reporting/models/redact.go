package models

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
)

// ErrUnsafePath is returned by SafePath when a requested path would escape the
// output root (e.g. via "..") or is absolute. Imported/exported paths must
// never escape the case directory (AGENTS.md §21).
var ErrUnsafePath = errors.New("reporting: unsafe path escapes output root")

// RedactedDataset is the provenance-safe projection of a schema.Dataset. It
// exposes ONLY the allow-listed provenance fields; counts and any field that
// could carry a credential or an operator-local absolute path beyond the
// recorded source file are deliberately excluded.
type RedactedDataset struct {
	ID              string `json:"id"`
	SourceFile      string `json:"source_file"`
	SHA256          string `json:"sha256"`
	SchemaVersion   string `json:"schema_version"`
	ToolVersion     string `json:"tool_version"`
	Format          string `json:"format,omitempty"`
	ParserVersion   string `json:"parser_version,omitempty"`
	ImporterVersion string `json:"importer_version,omitempty"`
	ImportedAt      string `json:"imported_at"`
}

// RedactDataset projects a schema.Dataset onto the allow-listed provenance
// fields (review NIT #1). The allow-list is explicit: adding a field here is a
// deliberate decision, so a secret can never leak by a struct gaining a field.
func RedactDataset(d schema.Dataset) RedactedDataset {
	return RedactedDataset{
		ID:              d.ID,
		SourceFile:      d.SourceFile,
		SHA256:          d.SHA256,
		SchemaVersion:   d.SchemaVersion,
		ToolVersion:     d.ToolVersion,
		Format:          d.Format,
		ParserVersion:   d.ParserVersion,
		ImporterVersion: d.ImporterVersion,
		ImportedAt:      d.ImportedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
}

// RedactDatasets redacts a slice of datasets, preserving order.
func RedactDatasets(in []schema.Dataset) []RedactedDataset {
	out := make([]RedactedDataset, 0, len(in))
	for _, d := range in {
		out = append(out, RedactDataset(d))
	}
	return out
}

// SafePath joins rel onto root and verifies the result stays within root. It
// rejects absolute rel values and any ".." escape. The returned path is clean
// and rooted at root.
func SafePath(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", ErrUnsafePath
	}
	cleanRoot := filepath.Clean(root)
	joined := filepath.Clean(filepath.Join(cleanRoot, rel))
	// joined must be cleanRoot itself or sit under cleanRoot + separator.
	if joined != cleanRoot && !strings.HasPrefix(joined, cleanRoot+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	return joined, nil
}
