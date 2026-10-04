package models

// FileEntry is one file inside an export bundle, with its byte length and
// content hash so the bundle can be verified offline.
type FileEntry struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// Manifest is the deterministic index of an export bundle. It records the
// report identity, the snapshot hash the files were rendered from, and a hash
// for every file so Verify can confirm the bundle is intact and unmodified.
type Manifest struct {
	ReportID         string      `json:"report_id"`
	CaseID           string      `json:"case_id"`
	SchemaVersion    string      `json:"schema_version"`
	GeneratorVersion string      `json:"generator_version"`
	SnapshotSHA256   string      `json:"snapshot_sha256"`
	GeneratedAt      string      `json:"generated_at"`
	GeneratedBy      string      `json:"generated_by"`
	Files            []FileEntry `json:"files"`
}

// FileCheck is the per-file outcome of verifying a bundle.
type FileCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// VerifyResult is the outcome of verifying an export bundle against its
// manifest.
type VerifyResult struct {
	OK    bool        `json:"ok"`
	Files []FileCheck `json:"files"`
}
