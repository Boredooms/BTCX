package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
)

// secretSubstrings are credential-looking tokens that must never appear in any
// rendered format.
var secretSubstrings = []string{"api_key", "apikey", "secret", "token", "password", "authorization", "bearer"}

func assertNoSecrets(t *testing.T, format string, data []byte) {
	t.Helper()
	low := bytes.ToLower(data)
	for _, s := range secretSubstrings {
		if bytes.Contains(low, []byte(s)) {
			t.Fatalf("%s output contains secret-like substring %q", format, s)
		}
	}
}

func TestJSONRenderDeterministic(t *testing.T) {
	svc, snap := newFixedService(t)
	ctx := context.Background()

	a, err := svc.Render(ctx, snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render a: %v", err)
	}
	b, err := svc.Render(ctx, snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("JSON render not byte-identical across two renders")
	}
}

func TestJSONVersionKeysFirst(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	s := string(data)
	if !strings.HasPrefix(s, `{"schema_version":`) {
		t.Fatalf("schema_version is not the first key: %.60s", s)
	}
	idx := strings.Index(s, `"generator_version":`)
	if idx < 0 || idx > strings.Index(s, `"case_information"`) {
		t.Fatal("generator_version must be among the first keys, before section data")
	}
}

func TestJSONValidAndSchemaVersionPresent(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if generic["schema_version"] != "report-schema-v1" {
		t.Fatalf("schema_version missing/wrong: %v", generic["schema_version"])
	}
	// Trace IDs must be present so every conclusion is traceable.
	meta, _ := generic["meta"].(map[string]any)
	if meta == nil || meta["report_id"] == "" {
		t.Fatal("meta.report_id missing from JSON output")
	}
}

func TestJSONNoSecrets(t *testing.T) {
	svc, snap := newFixedService(t)
	data, err := svc.Render(context.Background(), snap, schema.FormatJSON)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	assertNoSecrets(t, "json", data)
}
