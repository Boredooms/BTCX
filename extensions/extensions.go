// Package extensions implements the BCTX offline extension / knowledge manager.
//
// An extension is a self-describing, fully-local package a deployment (e.g. a
// ministry) drops into the extensions root (~/.bctx/extensions/<id>/) to bring
// their OWN assets into the air-gapped workstation:
//
//   - knowledge-base : reference corpora / watchlists / typology notes the
//     analyst can consult locally.
//   - model-pack     : their own ML model + inference manifest (tree-json or
//     onnx-exported) that can run alongside the bundled models.
//   - demo-example   : a packaged case dataset (ndjson) + subject that can be
//     loaded into a new case for demonstration/training.
//
// This package is a LEAF: it only reads the local filesystem (manifest JSON +
// an enabled marker file). It performs NO network access, runs NO SQL, and
// computes NO risk — discovery and enable/disable only. Everything it reports is
// honest: a malformed or missing manifest surfaces as an error status, never a
// fabricated entry. Loading/activating an extension's payload is done by higher
// layers (the TUI/CLI) through the existing dataset-import / model-registry
// seams; this package just locates and describes what is installed.
package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Kind enumerates the supported extension payload kinds.
type Kind string

const (
	KindKnowledgeBase Kind = "knowledge-base"
	KindModelPack     Kind = "model-pack"
	KindDemoExample   Kind = "demo-example"
)

// Valid reports whether k is a recognized extension kind.
func (k Kind) Valid() bool {
	switch k {
	case KindKnowledgeBase, KindModelPack, KindDemoExample:
		return true
	default:
		return false
	}
}

// Label is a short human label for the kind, for compact table display.
func (k Kind) Label() string {
	switch k {
	case KindKnowledgeBase:
		return "KNOWLEDGE"
	case KindModelPack:
		return "MODEL"
	case KindDemoExample:
		return "DEMO"
	default:
		return "UNKNOWN"
	}
}

// Manifest is the on-disk descriptor every extension ships as manifest.json in
// its directory. Only Name, Version and Kind are required; the rest are honest,
// optional metadata. Payload paths are RELATIVE to the extension directory and
// are validated to stay inside it (no traversal) when resolved.
type Manifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Kind        Kind   `json:"kind"`
	Publisher   string `json:"publisher,omitempty"`
	Description string `json:"description,omitempty"`

	// Dataset is a demo-example's ndjson dataset (relative path).
	Dataset string `json:"dataset,omitempty"`
	// Subject is a demo-example's primary subject (wallet/tx/ip) to open.
	Subject string `json:"subject,omitempty"`
	// Geo is an optional demo-example network-observations ndjson (relative).
	Geo string `json:"geo,omitempty"`

	// ModelDir is a model-pack's directory of model assets (relative). It is
	// expected to contain a model manifest the registry can load.
	ModelDir string `json:"model_dir,omitempty"`

	// Docs is a knowledge-base's primary document / index (relative path).
	Docs string `json:"docs,omitempty"`
}

// Status is the resolved health of an installed extension.
type Status string

const (
	// StatusEnabled: manifest valid and the enabled marker present.
	StatusEnabled Status = "ENABLED"
	// StatusDisabled: manifest valid but not enabled.
	StatusDisabled Status = "DISABLED"
	// StatusError: the manifest is missing/malformed/invalid.
	StatusError Status = "ERROR"
)

// Extension is one discovered extension: its id (directory name), parsed
// manifest, resolved status, absolute directory and any load error.
type Extension struct {
	ID       string
	Dir      string
	Manifest Manifest
	Status   Status
	Err      string
	Modified time.Time
}

// Enabled reports whether the extension is enabled (valid + marker present).
func (e Extension) Enabled() bool { return e.Status == StatusEnabled }

// enabledMarker is the file whose presence marks an extension enabled. Keeping
// enablement as a sibling marker file (not a manifest field) means enabling or
// disabling never rewrites the publisher's signed manifest.
const enabledMarker = ".enabled"

// manifestName is the required descriptor filename in each extension directory.
const manifestName = "manifest.json"

// Store is the offline extension manager rooted at a directory (normally
// ~/.bctx/extensions). All methods are filesystem-only and safe in airgap mode.
type Store struct {
	root string
}

// NewStore builds a store rooted at dir. The directory need not exist yet; List
// returns an empty set until an extension is installed.
func NewStore(dir string) *Store { return &Store{root: dir} }

// Root returns the extensions root directory.
func (s *Store) Root() string { return s.root }

// EnsureRoot creates the extensions root if absent (offline, idempotent).
func (s *Store) EnsureRoot() error {
	if s.root == "" {
		return errors.New("extensions: empty root")
	}
	return os.MkdirAll(s.root, 0o755)
}

// List discovers every extension directory under the root, parses each
// manifest, resolves status, and returns them sorted by (kind, name). A missing
// root is not an error — it yields an empty list (honest "none installed").
func (s *Store) List() ([]Extension, error) {
	if s.root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("extensions: read root: %w", err)
	}
	var out []Extension
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		out = append(out, s.load(de.Name()))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Manifest.Kind != out[j].Manifest.Kind {
			return out[i].Manifest.Kind < out[j].Manifest.Kind
		}
		return strings.ToLower(out[i].Manifest.Name) < strings.ToLower(out[j].Manifest.Name)
	})
	return out, nil
}

// Get returns a single extension by id (directory name).
func (s *Store) Get(id string) (Extension, error) {
	if s.root == "" || id == "" {
		return Extension{}, errors.New("extensions: empty root or id")
	}
	dir := filepath.Join(s.root, id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return Extension{}, fmt.Errorf("extensions: %q not found", id)
	}
	return s.load(id), nil
}

// load reads and validates one extension directory, resolving its status. It
// never returns an error: a bad manifest becomes an Extension with StatusError
// and a human reason, so the manager lists it honestly instead of hiding it.
func (s *Store) load(id string) Extension {
	dir := filepath.Join(s.root, id)
	ext := Extension{ID: id, Dir: dir}
	if info, err := os.Stat(dir); err == nil {
		ext.Modified = info.ModTime()
	}

	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		ext.Status = StatusError
		if os.IsNotExist(err) {
			ext.Err = "missing manifest.json"
		} else {
			ext.Err = "read manifest: " + err.Error()
		}
		return ext
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		ext.Status = StatusError
		ext.Err = "invalid manifest: " + err.Error()
		return ext
	}
	ext.Manifest = m
	if verr := validate(m, dir); verr != nil {
		ext.Status = StatusError
		ext.Err = verr.Error()
		return ext
	}
	if s.isEnabled(id) {
		ext.Status = StatusEnabled
	} else {
		ext.Status = StatusDisabled
	}
	return ext
}

// validate checks the manifest's required fields and that any declared payload
// path exists and stays inside the extension directory (no traversal).
func validate(m Manifest, dir string) error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("manifest.name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("manifest.version is required")
	}
	if !m.Kind.Valid() {
		return fmt.Errorf("manifest.kind %q is not one of knowledge-base/model-pack/demo-example", m.Kind)
	}
	check := func(field, rel string) error {
		if rel == "" {
			return nil
		}
		abs, err := resolveInside(dir, rel)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("%s %q not found", field, rel)
		}
		return nil
	}
	switch m.Kind {
	case KindDemoExample:
		if m.Dataset == "" {
			return errors.New("demo-example requires a dataset")
		}
		if err := check("dataset", m.Dataset); err != nil {
			return err
		}
		if err := check("geo", m.Geo); err != nil {
			return err
		}
	case KindModelPack:
		if m.ModelDir == "" {
			return errors.New("model-pack requires model_dir")
		}
		if err := check("model_dir", m.ModelDir); err != nil {
			return err
		}
	case KindKnowledgeBase:
		if err := check("docs", m.Docs); err != nil {
			return err
		}
	}
	return nil
}

// resolveInside joins rel onto dir and verifies the result stays inside dir,
// rejecting absolute paths and ".." traversal (defense against a manifest that
// points outside its own package).
func resolveInside(dir, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q must be relative", rel)
	}
	abs := filepath.Clean(filepath.Join(dir, rel))
	root := filepath.Clean(dir)
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes the extension directory", rel)
	}
	return abs, nil
}

// DatasetPath returns the absolute, validated dataset path for a demo-example
// extension, or an error if it is not a demo-example or the path is invalid.
func (e Extension) DatasetPath() (string, error) {
	if e.Manifest.Kind != KindDemoExample {
		return "", fmt.Errorf("%q is not a demo-example", e.ID)
	}
	return resolveInside(e.Dir, e.Manifest.Dataset)
}

// GeoPath returns the absolute, validated geo ndjson path for a demo-example,
// or ("", nil) when the example declares no geo dataset.
func (e Extension) GeoPath() (string, error) {
	if e.Manifest.Geo == "" {
		return "", nil
	}
	return resolveInside(e.Dir, e.Manifest.Geo)
}

// isEnabled reports whether the enabled marker exists for id.
func (s *Store) isEnabled(id string) bool {
	_, err := os.Stat(filepath.Join(s.root, id, enabledMarker))
	return err == nil
}

// SetEnabled enables or disables an extension by creating/removing its marker
// file. It validates the manifest first so a broken extension cannot be
// enabled. Returns the refreshed Extension.
func (s *Store) SetEnabled(id string, enabled bool) (Extension, error) {
	ext, err := s.Get(id)
	if err != nil {
		return Extension{}, err
	}
	if enabled && ext.Status == StatusError {
		return ext, fmt.Errorf("cannot enable %q: %s", id, ext.Err)
	}
	marker := filepath.Join(s.root, id, enabledMarker)
	if enabled {
		if err := os.WriteFile(marker, []byte("enabled by bctx\n"), 0o644); err != nil {
			return ext, fmt.Errorf("enable %q: %w", id, err)
		}
	} else {
		if err := os.Remove(marker); err != nil && !os.IsNotExist(err) {
			return ext, fmt.Errorf("disable %q: %w", id, err)
		}
	}
	return s.load(id), nil
}

// Counts summarizes an extension set for a compact header line.
type Counts struct {
	Total     int
	Enabled   int
	Knowledge int
	Models    int
	Demos     int
	Errored   int
}

// Summarize tallies a slice of extensions.
func Summarize(exts []Extension) Counts {
	var c Counts
	for _, e := range exts {
		c.Total++
		if e.Enabled() {
			c.Enabled++
		}
		if e.Status == StatusError {
			c.Errored++
		}
		switch e.Manifest.Kind {
		case KindKnowledgeBase:
			c.Knowledge++
		case KindModelPack:
			c.Models++
		case KindDemoExample:
			c.Demos++
		}
	}
	return c
}
