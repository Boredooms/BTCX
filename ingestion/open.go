package ingestion

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DetectFormat resolves the source format from extension + content inspection.
// Extension is a hint; content wins when it clearly contradicts the extension.
func DetectFormat(path string) (Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	byExt := map[string]Format{
		".csv": FormatCSV, ".json": FormatJSON,
		".ndjson": FormatNDJSON, ".jsonl": FormatNDJSON, ".xml": FormatXML,
	}[ext]

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for detect: %w", err)
	}
	defer f.Close()
	br := bufio.NewReader(f)
	// Peek the first non-space byte.
	var first byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			break
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		first = b
		break
	}

	switch first {
	case '<':
		return FormatXML, nil
	case '[':
		return FormatJSON, nil
	case '{':
		// Could be a single JSON object or NDJSON. Prefer the extension hint.
		if byExt == FormatNDJSON {
			return FormatNDJSON, nil
		}
		return FormatNDJSON, nil // treat bare-object streams as NDJSON
	}
	// Not obviously JSON/XML: trust extension, else default CSV.
	if byExt != "" {
		return byExt, nil
	}
	return FormatCSV, nil
}

// openParser opens a streaming parser for the given format.
func openParser(path string, format Format) (Parser, error) {
	rc, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open source: %w", err)
	}
	switch format {
	case FormatCSV:
		return newCSVParser(rc)
	case FormatJSON:
		return newJSONParser(rc, false)
	case FormatNDJSON:
		return newJSONParser(rc, true)
	case FormatXML:
		return newXMLParser(rc)
	default:
		_ = rc.Close()
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

// ensure io is referenced (used by parsers via the io.ReadCloser contract).
var _ io.ReadCloser = (*os.File)(nil)
