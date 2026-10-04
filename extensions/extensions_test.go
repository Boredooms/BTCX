package extensions

import (
	"os"
	"path/filepath"
	"testing"
)

// writeExt creates an extension directory with a manifest.json and optional
// payload files, returning the store root.
func writeExt(t *testing.T, root, id string, manifest string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListEmptyRoot(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "does-not-exist"))
	exts, err := s.List()
	if err != nil {
		t.Fatalf("missing root should not error: %v", err)
	}
	if len(exts) != 0 {
		t.Fatalf("expected 0 extensions, got %d", len(exts))
	}
}

func TestDiscoverAndValidate(t *testing.T) {
	root := t.TempDir()
	// A valid demo-example with its dataset present.
	writeExt(t, root, "demo-aml", `{
		"name":"AML Typologies","version":"1.0.0","kind":"demo-example",
		"publisher":"Ministry X","dataset":"data.ndjson","subject":"1Feex"}`,
		map[string]string{"data.ndjson": "{}\n"})
	// A valid knowledge-base.
	writeExt(t, root, "kb-sanctions", `{
		"name":"Sanctions Notes","version":"2.1","kind":"knowledge-base","docs":"notes.md"}`,
		map[string]string{"notes.md": "# notes\n"})
	// A model-pack missing its model_dir -> ERROR.
	writeExt(t, root, "bad-model", `{
		"name":"Bad Model","version":"0.1","kind":"model-pack"}`, nil)
	// Malformed JSON -> ERROR.
	writeExt(t, root, "broken", `{not json`, nil)
	// Missing manifest -> ERROR.
	writeExt(t, root, "nomani", "", nil)

	s := NewStore(root)
	exts, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(exts) != 5 {
		t.Fatalf("expected 5 discovered, got %d", len(exts))
	}
	byID := map[string]Extension{}
	for _, e := range exts {
		byID[e.ID] = e
	}
	if byID["demo-aml"].Status != StatusDisabled {
		t.Errorf("valid demo should be DISABLED (not yet enabled), got %s (%s)",
			byID["demo-aml"].Status, byID["demo-aml"].Err)
	}
	if byID["kb-sanctions"].Status != StatusDisabled {
		t.Errorf("valid kb should be DISABLED, got %s (%s)", byID["kb-sanctions"].Status, byID["kb-sanctions"].Err)
	}
	if byID["bad-model"].Status != StatusError {
		t.Errorf("model-pack without model_dir should be ERROR")
	}
	if byID["broken"].Status != StatusError {
		t.Errorf("malformed manifest should be ERROR")
	}
	if byID["nomani"].Status != StatusError {
		t.Errorf("missing manifest should be ERROR")
	}
}

func TestEnableDisable(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "demo-aml", `{
		"name":"AML","version":"1.0","kind":"demo-example","dataset":"d.ndjson"}`,
		map[string]string{"d.ndjson": "{}\n"})
	s := NewStore(root)

	ext, err := s.SetEnabled("demo-aml", true)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if ext.Status != StatusEnabled {
		t.Fatalf("expected ENABLED, got %s", ext.Status)
	}
	// Marker persists across a fresh store.
	s2 := NewStore(root)
	got, _ := s2.Get("demo-aml")
	if !got.Enabled() {
		t.Fatalf("enabled state should persist")
	}
	ext, err = s.SetEnabled("demo-aml", false)
	if err != nil {
		t.Fatalf("disable: %v", err)
	}
	if ext.Status != StatusDisabled {
		t.Fatalf("expected DISABLED, got %s", ext.Status)
	}
}

func TestCannotEnableBroken(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "bad", `{"name":"B","version":"1","kind":"model-pack"}`, nil)
	s := NewStore(root)
	if _, err := s.SetEnabled("bad", true); err == nil {
		t.Fatal("enabling a broken extension should error")
	}
}

func TestDatasetPathTraversalRejected(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "evil", `{
		"name":"Evil","version":"1","kind":"demo-example","dataset":"../../etc/passwd"}`, nil)
	s := NewStore(root)
	e := s.load("evil")
	if e.Status != StatusError {
		t.Fatalf("traversal dataset should be ERROR, got %s", e.Status)
	}
}

func TestDatasetPathResolves(t *testing.T) {
	root := t.TempDir()
	writeExt(t, root, "demo", `{
		"name":"D","version":"1","kind":"demo-example","dataset":"sub/data.ndjson","geo":"geo.ndjson"}`,
		map[string]string{"sub/data.ndjson": "{}\n", "geo.ndjson": "{}\n"})
	s := NewStore(root)
	e, _ := s.Get("demo")
	if e.Status != StatusDisabled {
		t.Fatalf("expected valid, got %s (%s)", e.Status, e.Err)
	}
	dp, err := e.DatasetPath()
	if err != nil {
		t.Fatalf("dataset path: %v", err)
	}
	if filepath.Base(dp) != "data.ndjson" {
		t.Fatalf("unexpected dataset path %q", dp)
	}
	gp, err := e.GeoPath()
	if err != nil || filepath.Base(gp) != "geo.ndjson" {
		t.Fatalf("geo path: %q err=%v", gp, err)
	}
}

func TestSummarize(t *testing.T) {
	exts := []Extension{
		{Status: StatusEnabled, Manifest: Manifest{Kind: KindDemoExample}},
		{Status: StatusDisabled, Manifest: Manifest{Kind: KindKnowledgeBase}},
		{Status: StatusError, Manifest: Manifest{Kind: KindModelPack}},
	}
	c := Summarize(exts)
	if c.Total != 3 || c.Enabled != 1 || c.Demos != 1 || c.Knowledge != 1 || c.Models != 1 || c.Errored != 1 {
		t.Fatalf("unexpected counts: %+v", c)
	}
}
