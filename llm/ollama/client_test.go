package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
)

// sampleInput is a fixed projection reused across the adapter tests.
func sampleInput() llm.SummaryInput {
	return llm.SummaryInput{
		Subject:   "WFIXTURE",
		RiskScore: 72,
		RiskBand:  "HIGH (50-74)",
		Evidence:  []string{"rapid fan-out"},
		Patterns:  []string{"rapid_flow: burst"},
		Related:   5,
		Txs:       41,
	}
}

// deterministicText is what the fallback produces for sampleInput; the degrade
// paths must return exactly this so the pane shows the fact-grounded summary.
func deterministicText(t *testing.T) string {
	t.Helper()
	s, err := llm.NewDeterministic().Summarize(context.Background(), sampleInput())
	if err != nil {
		t.Fatalf("deterministic: %v", err)
	}
	return s
}

// TestOllamaSuccessRephrases asserts a 200 response's text is returned verbatim
// (the model's rephrasing), and that the prompt we sent carries the
// deterministic facts and the "invent nothing" instruction.
func TestOllamaSuccessRephrases(t *testing.T) {
	var gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var req generateRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotPrompt = req.Prompt
		_ = json.NewEncoder(w).Encode(generateResponse{Response: "Rephrased: subject is high risk."})
	}))
	defer srv.Close()

	cfg := configs.Default()
	cfg.LLM.Enabled = true
	cfg.LLM.Endpoint = srv.URL
	cfg.LLM.Model = "test-model"

	s := New(cfg)
	got, err := s.Summarize(context.Background(), sampleInput())
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got != "Rephrased: subject is high risk." {
		t.Fatalf("expected rephrased text, got %q", got)
	}
	// The prompt must carry the already-computed facts and the guardrail.
	for _, want := range []string{"WFIXTURE", "72/100", "do not add", "invent"} {
		if !strings.Contains(gotPrompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, gotPrompt)
		}
	}
}

// TestOllamaNon200Degrades asserts a non-200 reply degrades to the deterministic
// summary plus a surfaced error (never empty, never fabricated).
func TestOllamaNon200Degrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := configs.Default()
	cfg.LLM.Enabled = true
	cfg.LLM.Endpoint = srv.URL

	got, err := New(cfg).Summarize(context.Background(), sampleInput())
	if err == nil {
		t.Fatal("expected an error on non-200 so the pane shows a 'not reachable' note")
	}
	if got != deterministicText(t) {
		t.Fatalf("non-200 must degrade to the deterministic summary, got %q", got)
	}
}

// TestOllamaTimeoutDegrades asserts a slow endpoint degrades to the
// deterministic summary + error rather than hanging the UI.
func TestOllamaTimeoutDegrades(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(generateResponse{Response: "late"})
	}))
	defer srv.Close()

	cfg := configs.Default()
	cfg.LLM.Enabled = true
	cfg.LLM.Endpoint = srv.URL

	// Clamp the caller context tighter than the server delay to force a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	got, err := New(cfg).Summarize(ctx, sampleInput())
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if got != deterministicText(t) {
		t.Fatalf("timeout must degrade to the deterministic summary, got %q", got)
	}
}

// TestNewDisabledIsDeterministic asserts a disabled or endpoint-less config
// yields the plain deterministic summarizer and constructs no socket.
func TestNewDisabledIsDeterministic(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*configs.Config)
	}{
		{"disabled", func(c *configs.Config) { c.LLM.Enabled = false }},
		{"no-endpoint", func(c *configs.Config) { c.LLM.Enabled = true; c.LLM.Endpoint = "" }},
		{"nil-cfg", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg *configs.Config
			if tc.mut != nil {
				cfg = configs.Default()
				tc.mut(cfg)
			}
			got, err := New(cfg).Summarize(context.Background(), sampleInput())
			if err != nil {
				t.Fatalf("Summarize: %v", err)
			}
			if got != deterministicText(t) {
				t.Fatalf("disabled config must return the deterministic summary, got %q", got)
			}
		})
	}
}
