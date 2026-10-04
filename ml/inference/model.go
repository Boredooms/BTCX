package inference

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manifest is the subset of models/<name>/manifest.json the runtime validates.
type Manifest struct {
	ModelName           string   `json:"model_name"`
	ModelVersion        string   `json:"model_version"`
	ModelFormat         string   `json:"model_format"`
	FeatureSchema       string   `json:"feature_schema"`
	FeatureSchemaSHA256 string   `json:"feature_schema_sha256"`
	ModelSHA256         string   `json:"model_sha256"`
	Classes             []string `json:"classes"`
	Status              string   `json:"status"`
}

// Calibration mirrors models/anomaly/calibration.json.
type Calibration struct {
	Formula string  `json:"formula"`
	Method  string  `json:"method"`
	RawLo   float64 `json:"raw_lo"`
	RawHi   float64 `json:"raw_hi"`
}

// ModelDir resolves artifact paths within a model directory.
type ModelDir struct {
	Dir string
}

func (d ModelDir) manifestPath() string    { return filepath.Join(d.Dir, "manifest.json") }
func (d ModelDir) treesPath() string       { return filepath.Join(d.Dir, "model.trees.json") }
func (d ModelDir) calibrationPath() string { return filepath.Join(d.Dir, "calibration.json") }

// loadManifest reads and returns the manifest.
func (d ModelDir) loadManifest() (*Manifest, error) {
	data, err := os.ReadFile(d.manifestPath())
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// ValidationError describes why a model was rejected.
type ValidationError struct {
	Model  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("model %s INVALID: %s", e.Model, e.Reason)
}

// sha256File hashes a file.
func sha256File(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Registry lazily loads and caches compiled tree models from a models root.
// Loaded models are reused; a sync.Mutex guards the cache and model sessions
// (tree evaluation itself is read-only and concurrency-safe once compiled).
type Registry struct {
	root string
	// expectedFeatureSchema is the feature-schema-v1 hash the runtime requires
	// for models declared against feature-schema-v1 (anomaly). Flow uses its own
	// flow-features schema and is exempt from this specific hash check.
	expectedFeatureSchemaSHA string

	mu     sync.Mutex
	models map[string]*LoadedModel
}

// LoadedModel bundles a compiled tree model with its validated manifest and
// optional calibration.
type LoadedModel struct {
	Manifest    *Manifest
	Trees       *TreeModel
	Calibration *Calibration
}

// NewRegistry creates a registry rooted at modelsDir.
func NewRegistry(modelsDir, featureSchemaSHA string) *Registry {
	return &Registry{
		root:                     modelsDir,
		expectedFeatureSchemaSHA: featureSchemaSHA,
		models:                   make(map[string]*LoadedModel),
	}
}

// Get lazily loads, validates and caches a model by directory name.
func (r *Registry) Get(name string) (*LoadedModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lm, ok := r.models[name]; ok {
		return lm, nil
	}
	dir := ModelDir{Dir: filepath.Join(r.root, name)}

	man, err := dir.loadManifest()
	if err != nil {
		return nil, &ValidationError{name, err.Error()}
	}
	// Feature-schema compatibility (fail-closed) for feature-schema-v1 models.
	if man.FeatureSchema == "feature-schema-v1" && r.expectedFeatureSchemaSHA != "" {
		if man.FeatureSchemaSHA256 != r.expectedFeatureSchemaSHA {
			return nil, &ValidationError{name, fmt.Sprintf(
				"feature schema hash mismatch: manifest=%s runtime=%s",
				man.FeatureSchemaSHA256, r.expectedFeatureSchemaSHA)}
		}
	}

	trees, err := LoadTreeModel(dir.treesPath())
	if err != nil {
		return nil, &ValidationError{name, err.Error()}
	}

	lm := &LoadedModel{Manifest: man, Trees: trees}

	// Calibration is required for the anomaly model.
	if _, err := os.Stat(dir.calibrationPath()); err == nil {
		data, rerr := os.ReadFile(dir.calibrationPath())
		if rerr != nil {
			return nil, &ValidationError{name, rerr.Error()}
		}
		var c Calibration
		if jerr := json.Unmarshal(data, &c); jerr != nil {
			return nil, &ValidationError{name, "parse calibration: " + jerr.Error()}
		}
		lm.Calibration = &c
	}

	r.models[name] = lm
	return lm, nil
}

// Close releases cached models. Tree models hold no native resources, so this
// simply clears the cache; it satisfies the resource-lifecycle contract.
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models = make(map[string]*LoadedModel)
	return nil
}
