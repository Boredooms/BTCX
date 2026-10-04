package geoip

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RegistryEntry is one record in ~/.bctx/geoip/registry.json. It records
// provenance and a sha256 for integrity checking. It holds NO secrets: only
// public dataset metadata and local install bookkeeping.
type RegistryEntry struct {
	DBName       string `json:"db_name"`
	DBType       string `json:"db_type"` // "country" | "asn"
	Source       string `json:"source"`
	SourceURL    string `json:"source_url"`
	Version      string `json:"version"`
	License      string `json:"license"`
	DownloadedAt string `json:"downloaded_at,omitempty"`
	InstalledAt  string `json:"installed_at"`
	SHA256       string `json:"sha256"`
	RecordCount  int64  `json:"record_count"`
	// Path is the absolute install path of the .mmdb this entry describes. It is
	// derived at install time and persisted so Status/Verify need no second
	// lookup.
	Path string `json:"path"`
}

// Registry is the on-disk collection of installed geoip databases, keyed by
// db_type ("country"/"asn") so a country DB and an ASN DB coexist.
type Registry struct {
	Entries map[string]RegistryEntry `json:"entries"`
}

// Dir returns the geoip asset directory (~/.bctx/geoip), creating nothing.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bctx", "geoip"), nil
}

func registryPath(dir string) string { return filepath.Join(dir, "registry.json") }

// LoadRegistry reads the registry from dir. A missing file is not an error: it
// returns an empty registry so callers can report "not installed" honestly. A
// present-but-unparseable file returns ErrCorruptRegistry.
func LoadRegistry(dir string) (*Registry, error) {
	b, err := os.ReadFile(registryPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Entries: map[string]RegistryEntry{}}, nil
		}
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptRegistry, err)
	}
	if r.Entries == nil {
		r.Entries = map[string]RegistryEntry{}
	}
	return &r, nil
}

// Save writes the registry to dir atomically (temp file + rename), creating the
// directory if needed. The file is written 0o644 and contains no secrets.
func (r *Registry) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := registryPath(dir) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, registryPath(dir))
}

// SHA256File streams a file through SHA-256 and returns the lowercase hex
// digest. It never loads the whole file into memory.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Install copies a user-supplied .mmdb into dir, computes its sha256, reads its
// MMDB metadata for db_type/record_count, and records an entry in the registry.
// It performs NO download and NO network access. meta supplies the provenance
// fields (source/version/license/url) the file format cannot carry itself.
func Install(dir, srcFile string, meta RegistryEntry) (RegistryEntry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return RegistryEntry{}, err
	}

	dbType, recordCount, err := inspectMMDB(srcFile)
	if err != nil {
		return RegistryEntry{}, err
	}

	name := meta.DBName
	if name == "" {
		name = baseNameNoExt(srcFile)
	}
	destPath := filepath.Join(dir, name+".mmdb")

	if err := copyFile(srcFile, destPath); err != nil {
		return RegistryEntry{}, err
	}

	sum, err := SHA256File(destPath)
	if err != nil {
		return RegistryEntry{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	entry := RegistryEntry{
		DBName:       name,
		DBType:       pickType(meta.DBType, dbType),
		Source:       meta.Source,
		SourceURL:    meta.SourceURL,
		Version:      meta.Version,
		License:      meta.License,
		DownloadedAt: meta.DownloadedAt,
		InstalledAt:  now,
		SHA256:       sum,
		RecordCount:  recordCount,
		Path:         destPath,
	}

	reg, err := LoadRegistry(dir)
	if err != nil {
		return RegistryEntry{}, err
	}
	reg.Entries[entry.DBType] = entry
	if err := reg.Save(dir); err != nil {
		return RegistryEntry{}, err
	}
	return entry, nil
}

// Verify recomputes a candidate file's sha256 and compares it to the entry of
// the matching db_type recorded in the registry. A file that matches no
// recorded entry, or whose digest differs, returns ErrChecksumMismatch.
func Verify(dir, file string) (RegistryEntry, error) {
	reg, err := LoadRegistry(dir)
	if err != nil {
		return RegistryEntry{}, err
	}
	if len(reg.Entries) == 0 {
		return RegistryEntry{}, ErrNotInstalled
	}
	sum, err := SHA256File(file)
	if err != nil {
		return RegistryEntry{}, err
	}
	for _, e := range reg.Entries {
		if e.SHA256 == sum {
			return e, nil
		}
	}
	return RegistryEntry{}, ErrChecksumMismatch
}

func pickType(explicit, detected string) string {
	if explicit != "" {
		return explicit
	}
	if detected != "" {
		return detected
	}
	return "country"
}

func baseNameNoExt(p string) string {
	b := filepath.Base(p)
	ext := filepath.Ext(b)
	return b[:len(b)-len(ext)]
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
