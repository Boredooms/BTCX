package mapdata

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

// assetFileName is the fixed install name of the world geometry asset.
const assetFileName = "world-110m.asset"

// RegistryEntry records the provenance + integrity of the installed world
// geometry asset. It mirrors the geoip registry shape and holds NO secrets. The
// single sha256 covers both the coastline polylines and the derived
// country-centroid table (§16: the centroid table has no row of its own).
type RegistryEntry struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	SourceURL   string `json:"source_url"`
	Version     string `json:"version"`
	License     string `json:"license"`
	InstalledAt string `json:"installed_at"`
	SHA256      string `json:"sha256"`
	RecordCount int64  `json:"record_count"` // country/centroid count
	// Path is the absolute install path of the asset this entry describes.
	Path string `json:"path"`
}

// Registry is the on-disk ~/.bctx/mapdata/registry.json. Only one world asset is
// installed at a time, so it holds a single entry.
type Registry struct {
	Entry RegistryEntry `json:"entry"`
}

// Dir returns the mapdata asset directory (~/.bctx/mapdata), creating nothing.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bctx", "mapdata"), nil
}

func registryPath(dir string) string { return filepath.Join(dir, "registry.json") }

// LoadRegistry reads the registry from dir. A missing file is not an error: it
// returns an empty registry so callers report "not installed" honestly. A
// present-but-unparseable file returns ErrCorruptRegistry.
func LoadRegistry(dir string) (*Registry, error) {
	b, err := os.ReadFile(registryPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{}, nil
		}
		return nil, err
	}
	var r Registry
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptRegistry, err)
	}
	return &r, nil
}

// Save writes the registry to dir atomically (temp file + rename).
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
// digest without loading the whole file into memory.
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

// Install copies a world-110m.asset into dir, parses it to count centroids,
// computes its sha256, and writes the registry. It performs NO download and NO
// network access. A file that does not parse as a geometry asset is rejected
// before anything is recorded. meta supplies the provenance fields.
func Install(dir, srcFile string, meta RegistryEntry) (RegistryEntry, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return RegistryEntry{}, err
	}

	g, err := LoadGeometry(srcFile)
	if err != nil {
		return RegistryEntry{}, err
	}

	destPath := filepath.Join(dir, assetFileName)
	if err := copyFile(srcFile, destPath); err != nil {
		return RegistryEntry{}, err
	}
	sum, err := SHA256File(destPath)
	if err != nil {
		return RegistryEntry{}, err
	}

	name := meta.Name
	if name == "" {
		name = "world-110m"
	}
	entry := RegistryEntry{
		Name:        name,
		Source:      meta.Source,
		SourceURL:   meta.SourceURL,
		Version:     meta.Version,
		License:     meta.License,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
		SHA256:      sum,
		RecordCount: int64(len(g.Centroids)),
		Path:        destPath,
	}
	reg := &Registry{Entry: entry}
	if err := reg.Save(dir); err != nil {
		return RegistryEntry{}, err
	}
	return entry, nil
}

// VerifyInstalled recomputes the installed asset's sha256 and compares it to the
// registry. It returns ErrNotInstalled when nothing is installed and
// ErrChecksumMismatch when the digest differs.
func VerifyInstalled(dir string) (RegistryEntry, error) {
	reg, err := LoadRegistry(dir)
	if err != nil {
		return RegistryEntry{}, err
	}
	if reg.Entry.Name == "" {
		return RegistryEntry{}, ErrNotInstalled
	}
	sum, err := SHA256File(reg.Entry.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg.Entry, ErrNotInstalled
		}
		return reg.Entry, err
	}
	if sum != reg.Entry.SHA256 {
		return reg.Entry, ErrChecksumMismatch
	}
	return reg.Entry, nil
}

// VerifyFile recomputes a candidate file's sha256 and compares it to the
// registry entry. Used to check a file before install.
func VerifyFile(dir, file string) (RegistryEntry, error) {
	reg, err := LoadRegistry(dir)
	if err != nil {
		return RegistryEntry{}, err
	}
	if reg.Entry.Name == "" {
		return RegistryEntry{}, ErrNotInstalled
	}
	sum, err := SHA256File(file)
	if err != nil {
		return RegistryEntry{}, err
	}
	if sum != reg.Entry.SHA256 {
		return reg.Entry, ErrChecksumMismatch
	}
	return reg.Entry, nil
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
