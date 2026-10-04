package tui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/configs"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/storage/sqlite"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/screens"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// security_redaction_test.go enforces design §12 invariant 5 / AGENTS §21: no
// screen, component, or chrome ever renders a credential, token-bearing
// endpoint, or raw secret. We plant obvious secret values into the parts of the
// configuration that are user-controlled and could plausibly carry one (the
// explorer endpoint URL, the optional LLM endpoint, and the provider cell),
// drive every registry screen plus the Root chrome against a real seeded case,
// and assert the planted secret value never appears in any rendered output.

// plantedSecret is a sentinel value stuffed into secret-carrying config fields.
// If it ever appears in view output, redaction failed.
const plantedSecret = "s3cr3t-TOKEN-DO-NOT-LEAK-abcdef0123456789"

// secretConfig returns an offline config with the planted secret in every
// field a careless presenter might surface.
func secretConfig() *configs.Config {
	cfg := configs.Default()
	cfg.Network.Mode = configs.ModeOffline // deterministic offline gate
	// Token embedded in the explorer endpoint URL (both as query param and as
	// userinfo credential).
	cfg.Network.ExplorerEndpoint = "https://user:" + plantedSecret +
		"@explorer.example/api?apikey=" + plantedSecret
	// Provider cell carries an accidental token-bearing endpoint.
	cfg.Acquisition.Provider = "explorer?token=" + plantedSecret
	// Optional LLM endpoint with an auth token.
	cfg.LLM.Enabled = true
	cfg.LLM.Endpoint = "http://localhost:11434?access_token=" + plantedSecret
	cfg.LLM.Model = "gemma3:1b"
	return cfg
}

// seedSecretCase opens a temp sqlite repo and seeds a wallet tx + a network
// observation so screens have real local data to render (fixtures live only in
// tests, AGENTS §15/§16).
func seedSecretCase(t *testing.T) *sqlite.Repository {
	t.Helper()
	repo, err := sqlite.NewRepository(filepath.Join(t.TempDir(), "case.db"))
	if err != nil {
		t.Fatalf("new repo: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	ctx := context.Background()
	tx := schema.Transaction{
		TxID:      "TXFIXTURE1",
		Timestamp: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		FeeBTC:    0.0001,
		Inputs:    []schema.TransactionInput{{Address: "WFIXTURE", AmountBTC: 1.0, Index: 0}},
		Outputs:   []schema.TransactionOutput{{Address: "WOTHER", AmountBTC: 0.99, Index: 0}},
	}
	if err := repo.SaveTransactions(ctx, []schema.Transaction{tx}); err != nil {
		t.Fatalf("seed tx: %v", err)
	}
	obs := schema.NetworkObservation{
		ID: "OBS1", TxID: "TXFIXTURE1", SrcIP: "203.0.113.9", DstIP: "198.51.100.2",
		Country: "DE", ASN: "AS3320", Timestamp: time.Now().UTC(),
	}
	if err := repo.SaveNetworkObservations(ctx, []schema.NetworkObservation{obs}); err != nil {
		t.Fatalf("seed obs: %v", err)
	}
	return repo
}

// secretScreenCtx builds a screens.ScreenCtx wired to the seeded case and the
// planted-secret config.
func secretScreenCtx(t *testing.T) *screens.ScreenCtx {
	t.Helper()
	repo := seedSecretCase(t)
	return &screens.ScreenCtx{
		Ctx:     context.Background(),
		App:     &app.App{Repo: repo, CaseID: "secret-case", Cleanup: func() {}},
		Cfg:     secretConfig(),
		Styles:  theme.Build(theme.Default()),
		Subject: screens.Subject{ID: "WFIXTURE", Kind: screens.SubjectWallet},
	}
}

// screenFactories enumerates every registry screen by constructor so a single
// table-driven test exercises all of them with the planted-secret config.
func screenFactories() map[string]func(*screens.ScreenCtx) screens.Model {
	return map[string]func(*screens.ScreenCtx) screens.Model{
		"home":        func(c *screens.ScreenCtx) screens.Model { return screens.NewHome(c) },
		"search":      func(c *screens.ScreenCtx) screens.Model { return screens.NewSearch(c) },
		"wallet":      func(c *screens.ScreenCtx) screens.Model { return screens.NewWallet(c) },
		"transaction": func(c *screens.ScreenCtx) screens.Model { return screens.NewTransaction(c) },
		"entity":      func(c *screens.ScreenCtx) screens.Model { return screens.NewEntity(c) },
		"graph":       func(c *screens.ScreenCtx) screens.Model { return screens.NewGraph(c) },
		"network":     func(c *screens.ScreenCtx) screens.Model { return screens.NewNetwork(c) },
		"geomap":      func(c *screens.ScreenCtx) screens.Model { return screens.NewGeoMap(c) },
		"detection":   func(c *screens.ScreenCtx) screens.Model { return screens.NewDetection(c) },
		"alerts":      func(c *screens.ScreenCtx) screens.Model { return screens.NewAlerts(c) },
		"monitoring":  func(c *screens.ScreenCtx) screens.Model { return screens.NewMonitoring(c) },
		"data":        func(c *screens.ScreenCtx) screens.Model { return screens.NewData(c) },
		"reports":     func(c *screens.ScreenCtx) screens.Model { return screens.NewReports(c) },
		"settings":    func(c *screens.ScreenCtx) screens.Model { return screens.NewSettings(c) },
		"help":        func(c *screens.ScreenCtx) screens.Model { return screens.NewHelp(c) },
		"timeline":    func(c *screens.ScreenCtx) screens.Model { return screens.NewTimeline(c) },
	}
}

// renderFrames are the sizes each screen is rendered into for the secret scan.
var renderFrames = []components.Frame{{W: 120, H: 40}, {W: 200, H: 60}, {W: 80, H: 24}}

// TestNoScreenLeaksSecret drives every screen (including after its Init command
// resolves) with a planted-secret config and asserts the secret value never
// appears in rendered output. The Settings screen is the primary subject (it
// shows configuration) but every screen is checked.
func TestNoScreenLeaksSecret(t *testing.T) {
	for name, factory := range screenFactories() {
		ctx := secretScreenCtx(t)
		m := factory(ctx)
		// Resolve the Init command once synchronously so loaded state (not just
		// the loading placeholder) is scanned.
		if cmd := m.Init(); cmd != nil {
			if msg := cmd(); msg != nil {
				m, _ = m.Update(msg)
			}
		}
		for _, f := range renderFrames {
			out := m.View(f)
			if strings.Contains(out, plantedSecret) {
				t.Errorf("screen %q @ %dx%d leaked the planted secret:\n%s",
					name, f.W, f.H, out)
			}
		}
	}
}

// TestRootChromeLeaksNoSecret drives the full Root (TopBar + SideNav + active
// screen) against a planted-secret config and asserts the chrome never shows
// the secret. The TopBar provider cell is the specific risk the redaction guard
// covers.
func TestRootChromeLeaksNoSecret(t *testing.T) {
	repo := seedSecretCase(t)
	a := &app.App{
		Repo:    repo,
		CaseID:  "secret-case",
		Cleanup: func() {},
	}
	// Build an engine so the Root reads a live Config (with the planted secret).
	built, err := app.Build(secretConfig())
	if err != nil {
		t.Fatalf("app.Build: %v", err)
	}
	a.Cases = built.Cases
	a.Engine = built.Engine

	r := NewRoot(context.Background(), a)
	r.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	r.Init()
	// Visit a representative set of screens through nav, rendering each.
	visit := []ScreenID{ScreenHome, ScreenSettings, ScreenData, ScreenMonitoring, ScreenNetwork}
	for _, id := range visit {
		r.navTo(id, Subject{})
		out := r.View()
		if strings.Contains(out, plantedSecret) {
			t.Errorf("Root chrome on screen %q leaked the planted secret:\n%s", id, out)
		}
	}
}

// TestRedactScrubsSecretShapes is a focused unit test of the Redact helper: it
// scrubs URL userinfo credentials and sensitive query parameters while leaving
// a benign value unchanged, and is idempotent.
func TestRedactScrubsSecretShapes(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"userinfo", "https://user:" + plantedSecret + "@host/api"},
		{"apikey-query", "https://host/api?apikey=" + plantedSecret},
		{"token-query", "explorer?token=" + plantedSecret},
		{"access-token", "http://host?access_token=" + plantedSecret + "&x=1"},
		{"password-query", "db://host?password=" + plantedSecret},
	}
	for _, c := range cases {
		got := Redact(c.in)
		if strings.Contains(got, plantedSecret) {
			t.Errorf("%s: Redact left the secret in %q", c.name, got)
		}
		if !strings.Contains(got, redactedMarker) {
			t.Errorf("%s: Redact did not insert the marker in %q", c.name, got)
		}
		if Redact(got) != got {
			t.Errorf("%s: Redact is not idempotent: %q -> %q", c.name, got, Redact(got))
		}
	}

	// A benign value is unchanged.
	benign := "https://explorer.example/api/v1/wallet"
	if Redact(benign) != benign {
		t.Errorf("Redact altered a benign value: %q -> %q", benign, Redact(benign))
	}
}

// TestIsSensitiveKeyAndRedactField asserts the key-name guard and the
// non-revealing field presenter.
func TestIsSensitiveKeyAndRedactField(t *testing.T) {
	for _, k := range []string{"api_key", "ExplorerToken", "password", "AuthHeader", "signature"} {
		if !IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = false, want true", k)
		}
	}
	for _, k := range []string{"provider", "models_dir", "network mode"} {
		if IsSensitiveKey(k) {
			t.Errorf("IsSensitiveKey(%q) = true, want false", k)
		}
	}
	if got := RedactField("api_key", plantedSecret); got != "(set)" {
		t.Errorf(`RedactField("api_key", secret) = %q, want "(set)"`, got)
	}
	if got := RedactField("api_key", ""); got != "(none)" {
		t.Errorf(`RedactField("api_key", "") = %q, want "(none)"`, got)
	}
	if got := RedactField("provider", "explorer"); got != "explorer" {
		t.Errorf(`RedactField("provider", "explorer") = %q, want "explorer"`, got)
	}
}
