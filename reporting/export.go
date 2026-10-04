package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/sdk"
)

// Bundle layout. A report export is a self-describing directory ("report/")
// containing the rendered formats plus structured side-car files, a manifest
// and a coreutils-compatible checksum file. These names are the stable public
// contract of an export bundle.
const (
	bundleDirName   = "report"
	fileReportJSON  = "report.json"
	fileReportMD    = "report.md"
	fileReportHTML  = "report.html"
	fileReportPDF   = "report.pdf"
	fileEvidence    = "evidence.json"
	fileTimeline    = "timeline.json"
	fileManifest    = "manifest.json"
	fileChecksums   = "checksums.sha256"
	manifestVersion = ReportSchemaVersion
)

// formatFile maps a report format to its bundle filename.
func formatFile(f schema.ReportFormat) (string, bool) {
	switch f {
	case schema.FormatJSON:
		return fileReportJSON, true
	case schema.FormatMarkdown:
		return fileReportMD, true
	case schema.FormatHTML:
		return fileReportHTML, true
	case schema.FormatPDF:
		return fileReportPDF, true
	default:
		return "", false
	}
}

// ExportBundle renders the requested formats for snap into a "report/"
// directory under outDir, alongside evidence.json, timeline.json, a canonical
// manifest.json and a sorted checksums.sha256. The bundle is fully offline and
// deterministic (fixed clock => byte-identical bundle, except the generated_at
// stamp in the manifest and rendered reports).
//
// Safety: the bundle directory is validated to stay within outDir (ErrUnsafePath
// on escape). On any write failure every file written so far is removed so a
// partial bundle is never left behind. The export is recorded via
// repo.SaveReportExport.
func (s *Service) ExportBundle(ctx context.Context, snap models.ReportSnapshot, formats []schema.ReportFormat, outDir string) (models.Manifest, error) {
	if len(formats) == 0 {
		return models.Manifest{}, errors.New("reporting: no formats requested")
	}

	bundleDir, err := models.SafePath(outDir, bundleDirName)
	if err != nil {
		return models.Manifest{}, err
	}
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return models.Manifest{}, fmt.Errorf("reporting: create bundle dir: %w", err)
	}

	generatedAt := s.clock().UTC().Format(rfc3339)
	view := s.buildView(snap, generatedAt)
	snapHash := SnapshotHash(snap)
	reportID := ReportID(snap)

	// Track written files so we can roll back on failure.
	var written []string
	cleanup := func() {
		for _, p := range written {
			_ = os.Remove(p)
		}
		// Remove the bundle dir if we created it and it is now empty.
		_ = os.Remove(bundleDir)
	}
	writeBundleFile := func(name string, data []byte) error {
		p := filepath.Join(bundleDir, name)
		if err := writeFileAtomic(p, data); err != nil {
			return err
		}
		written = append(written, p)
		return nil
	}

	// 1. Rendered report formats (deduplicated, deterministic order).
	seen := map[schema.ReportFormat]bool{}
	ordered := make([]schema.ReportFormat, 0, len(formats))
	for _, f := range formats {
		if seen[f] {
			continue
		}
		seen[f] = true
		ordered = append(ordered, f)
	}
	for _, f := range ordered {
		name, ok := formatFile(f)
		if !ok {
			cleanup()
			return models.Manifest{}, fmt.Errorf("%w: %q", ErrUnknownFormat, f)
		}
		data, rerr := s.Render(ctx, snap, f)
		if rerr != nil {
			cleanup()
			return models.Manifest{}, rerr
		}
		if werr := writeBundleFile(name, data); werr != nil {
			cleanup()
			return models.Manifest{}, werr
		}
	}

	// 2. evidence.json — the snapshot's evidence with full referential
	// integrity (ordered by ID; source records sorted). No orphaned refs.
	evidenceData, err := CanonicalJSON(view.Evidence)
	if err != nil {
		cleanup()
		return models.Manifest{}, fmt.Errorf("reporting: encode evidence: %w", err)
	}
	if err := writeBundleFile(fileEvidence, append(evidenceData, '\n')); err != nil {
		cleanup()
		return models.Manifest{}, err
	}

	// 3. timeline.json — the deterministic Timeline output.
	timelineData, err := CanonicalJSON(view.Timeline)
	if err != nil {
		cleanup()
		return models.Manifest{}, fmt.Errorf("reporting: encode timeline: %w", err)
	}
	if err := writeBundleFile(fileTimeline, append(timelineData, '\n')); err != nil {
		cleanup()
		return models.Manifest{}, err
	}

	// 4. manifest.json — index with a hash for every file written so far.
	entries := make([]models.FileEntry, 0, len(written))
	for _, p := range written {
		info, serr := os.Stat(p)
		if serr != nil {
			cleanup()
			return models.Manifest{}, serr
		}
		h, herr := FileSHA256(p)
		if herr != nil {
			cleanup()
			return models.Manifest{}, herr
		}
		entries = append(entries, models.FileEntry{
			Name:   filepath.Base(p),
			Bytes:  info.Size(),
			SHA256: h,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	manifest := models.Manifest{
		ReportID:         reportID,
		CaseID:           snap.Case.ID,
		SchemaVersion:    snap.SchemaVersion,
		GeneratorVersion: snap.GeneratorVersion,
		SnapshotSHA256:   snapHash,
		GeneratedAt:      generatedAt,
		GeneratedBy:      s.build.GeneratedBy,
		Files:            entries,
	}
	manifestData, err := CanonicalJSON(manifest)
	if err != nil {
		cleanup()
		return models.Manifest{}, fmt.Errorf("reporting: encode manifest: %w", err)
	}
	manifestData = append(manifestData, '\n')
	if err := writeBundleFile(fileManifest, manifestData); err != nil {
		cleanup()
		return models.Manifest{}, err
	}

	// 5. checksums.sha256 — coreutils-compatible "<sha256>  <name>" lines,
	// sorted by name. Covers every file EXCEPT the checksum file itself.
	var sums strings.Builder
	for _, e := range entries {
		sums.WriteString(e.SHA256)
		sums.WriteString("  ")
		sums.WriteString(e.Name)
		sums.WriteByte('\n')
	}
	// Include the manifest's own hash line (manifest is in entries already).
	if err := writeBundleFile(fileChecksums, []byte(sums.String())); err != nil {
		cleanup()
		return models.Manifest{}, err
	}

	// Persist the export record (manifest_sha256 = sha256 of canonical manifest).
	exportRow := newReportExportRow(reportID, snap.Case.ID, bundleDir, ordered,
		BytesSHA256(manifestData), generatedAt)
	if err := s.repo.SaveReportExport(ctx, exportRow); err != nil {
		cleanup()
		return models.Manifest{}, fmt.Errorf("reporting: persist export: %w", err)
	}

	return manifest, nil
}

// Verify recomputes the SHA-256 of every file listed in checksums.sha256 and
// manifest.json inside bundleDir and compares them. A checksum mismatch or a
// missing file is reported as DATA (VerifyResult.OK=false with per-file
// status), NOT an error; only I/O failures reading the manifest/checksum files
// return an error. Fully offline.
func (s *Service) Verify(ctx context.Context, bundleDir string) (models.VerifyResult, error) {
	manifestPath := filepath.Join(bundleDir, fileManifest)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return models.VerifyResult{}, fmt.Errorf("reporting: read manifest: %w", err)
	}
	var manifest models.Manifest
	if err := jsonUnmarshal(manifestBytes, &manifest); err != nil {
		return models.VerifyResult{}, fmt.Errorf("reporting: parse manifest: %w", err)
	}

	// Expected hashes from the manifest.
	expected := make(map[string]string, len(manifest.Files))
	names := make([]string, 0, len(manifest.Files))
	for _, f := range manifest.Files {
		expected[f.Name] = f.SHA256
		names = append(names, f.Name)
	}

	// Cross-check against checksums.sha256 if present (coreutils format).
	checksumPath := filepath.Join(bundleDir, fileChecksums)
	if cb, cerr := os.ReadFile(checksumPath); cerr == nil {
		for name, hash := range parseChecksums(cb) {
			if _, ok := expected[name]; !ok {
				expected[name] = hash
				names = append(names, name)
			}
		}
	} else if !os.IsNotExist(cerr) {
		return models.VerifyResult{}, fmt.Errorf("reporting: read checksums: %w", cerr)
	}

	sort.Strings(names)
	names = dedupeStrings(names)

	result := models.VerifyResult{OK: true}
	for _, name := range names {
		p := filepath.Join(bundleDir, name)
		got, herr := FileSHA256(p)
		switch {
		case herr != nil && os.IsNotExist(herr):
			result.OK = false
			result.Files = append(result.Files, models.FileCheck{Name: name, Status: "MISSING"})
		case herr != nil:
			return models.VerifyResult{}, fmt.Errorf("reporting: hash %s: %w", name, herr)
		case got != expected[name]:
			result.OK = false
			result.Files = append(result.Files, models.FileCheck{Name: name, Status: "MISMATCH"})
		default:
			result.Files = append(result.Files, models.FileCheck{Name: name, Status: "OK"})
		}
	}
	return result, nil
}

// writeFileAtomic writes data to path via a temp file + rename so a reader never
// observes a partially written file. The parent directory must exist.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".report-*.tmp")
	if err != nil {
		return fmt.Errorf("reporting: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("reporting: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("reporting: close temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("reporting: rename: %w", err)
	}
	return nil
}

// parseChecksums parses coreutils "<sha256>  <name>" lines into a name->hash map.
func parseChecksums(b []byte) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		// Format: hash + two spaces + name.
		idx := strings.Index(line, "  ")
		if idx <= 0 {
			continue
		}
		hash := line[:idx]
		name := line[idx+2:]
		out[name] = hash
	}
	return out
}

// jsonUnmarshal decodes JSON into v. It is a thin wrapper kept local so the
// rest of the file reads without importing encoding/json repeatedly.
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }

// newReportExportRow builds the persistence row for an export. The export ID is
// derived from the report ID + the manifest hash so it is deterministic for a
// given bundle (no timestamp in the identity).
func newReportExportRow(reportID, caseID, bundlePath string, formats []schema.ReportFormat, manifestSHA, createdAt string) sdk.ReportExportRow {
	fs := make([]string, 0, len(formats))
	for _, f := range formats {
		fs = append(fs, string(f))
	}
	return sdk.ReportExportRow{
		ExportID:       "exp-" + reportID + "-" + shortHash(manifestSHA),
		ReportID:       reportID,
		CaseID:         caseID,
		BundlePath:     bundlePath,
		Formats:        strings.Join(fs, ","),
		ManifestSHA256: manifestSHA,
		CreatedAt:      createdAt,
	}
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func dedupeStrings(in []string) []string {
	out := in[:0]
	var last string
	for i, s := range in {
		if i == 0 || s != last {
			out = append(out, s)
		}
		last = s
	}
	return out
}
