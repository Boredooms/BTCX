// Package ollama is the ONLY new network-capable addition in the tree (design
// §E.2/§E.3). It holds a minimal net/http adapter that asks a LOCAL Ollama
// endpoint to rephrase an already-computed deterministic summary. It is
// imported ONLY from cli/commands — never from package app, never from tui, and
// never from tui/screens — so net/http never enters the air-gap-gated closure.
// Any failure (disabled, unreachable, timeout, non-200) degrades to the
// deterministic summary plus an honest note, so the pane is never empty and
// never fabricates.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/llm"
)

// requestTimeout bounds a single generate call. It is short because the pane is
// an interactive, best-effort narration: a slow/absent model degrades fast to
// the deterministic summary rather than stalling the UI.
const requestTimeout = 3 * time.Second

// ollamaSummarizer POSTs to a local Ollama /api/generate endpoint. It composes
// the deterministic summarizer as both the prompt-context source and the
// fallback, so it can never leave the pane empty and never invent facts.
type ollamaSummarizer struct {
	endpoint string
	model    string
	fallback llm.Summarizer
	client   *http.Client
}

// New builds an llm.Summarizer backed by a local Ollama endpoint. The returned
// value ALWAYS satisfies the interface; when cfg disables the LLM or omits an
// endpoint it returns the plain deterministic summarizer directly (no socket is
// ever constructed in that case). Selection by cfg mirrors design §E.2.
func New(cfg *configs.Config) llm.Summarizer {
	det := llm.NewDeterministic()
	if cfg == nil || !cfg.LLM.Enabled || strings.TrimSpace(cfg.LLM.Endpoint) == "" {
		return det
	}
	return &ollamaSummarizer{
		endpoint: strings.TrimRight(cfg.LLM.Endpoint, "/"),
		model:    cfg.LLM.Model,
		fallback: det,
		client:   &http.Client{Timeout: requestTimeout},
	}
}

// generateRequest is the subset of the Ollama /api/generate body we send. We
// disable streaming so a single JSON object comes back.
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// generateResponse is the subset of the /api/generate reply we read.
type generateResponse struct {
	Response string `json:"response"`
}

// Summarize asks the local model to rephrase the deterministic summary. On ANY
// error it degrades to the deterministic summary — the caller sees a non-empty,
// fact-grounded string either way (the honest "degraded" note is attached by
// the screens-layer explainCmd, which distinguishes success from fallback by
// comparing against the deterministic text).
func (o *ollamaSummarizer) Summarize(ctx context.Context, in llm.SummaryInput) (string, error) {
	// The deterministic summary is the ground truth the model may only rephrase.
	facts, derr := o.fallback.Summarize(ctx, in)
	if derr != nil {
		return "", derr
	}

	text, err := o.generate(ctx, facts)
	if err != nil {
		// Degrade: return the deterministic summary and surface the error so the
		// pane can show an honest "not reachable" note. Never fabricate.
		return facts, err
	}
	return text, nil
}

// generate performs the bounded HTTP POST and returns the rephrased text. The
// prompt instructs the model to summarize ONLY the provided facts and invent
// nothing. The request shares the caller's context so Esc/quit cancels it, with
// the short client timeout as a hard ceiling.
func (o *ollamaSummarizer) generate(ctx context.Context, facts string) (string, error) {
	prompt := "Rephrase the following forensic findings into a concise, plain-language " +
		"summary for an analyst. Summarize ONLY the facts provided below; do not add, " +
		"infer, or invent any wallet, transaction, score, pattern, or evidence that is " +
		"not stated. Do not speculate about criminality.\n\nFINDINGS:\n" + facts

	body, err := json.Marshal(generateRequest{Model: o.model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", err
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost,
		o.endpoint+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: unexpected status %d", resp.StatusCode)
	}

	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	text := strings.TrimSpace(out.Response)
	if text == "" {
		return "", fmt.Errorf("ollama: empty response")
	}
	return text, nil
}
