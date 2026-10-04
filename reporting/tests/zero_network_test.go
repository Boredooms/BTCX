package tests

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
)

// panicRoundTripper is an http.RoundTripper that panics on any use. Installing
// it as http.DefaultTransport turns any accidental HTTP request made during
// report generation into an immediate, loud failure.
type panicRoundTripper struct{}

func (panicRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	panic("ZERO-NETWORK violation: reporting attempted an HTTP request")
}

// TestZeroNetworkDuringReporting proves the entire reporting surface
// (BuildSnapshot + Render for every format + ExportBundle + Verify) opens no
// socket. It installs a panicking RoundTripper as http.DefaultTransport for the
// duration of the test; if any reporting code path performed an HTTP request it
// would panic and fail the test. The transport is restored in a defer.
//
// Reporting imports no net/http at all (enforced by the offline boundary test),
// so this is a belt-and-suspenders runtime proof of the same invariant: even if
// a future change pulled in an HTTP client, this test would catch an actual
// request.
func TestZeroNetworkDuringReporting(t *testing.T) {
	orig := http.DefaultTransport
	http.DefaultTransport = panicRoundTripper{}
	defer func() { http.DefaultTransport = orig }()

	svc, snap := newFixedService(t)
	ctx := context.Background()

	// BuildSnapshot again under the panicking transport (reads local repo only).
	rebuilt, err := svc.BuildSnapshot(ctx, richResult())
	if err != nil {
		t.Fatalf("build snapshot under zero-network: %v", err)
	}
	_ = rebuilt

	// Render every format.
	for _, f := range allFormats() {
		if _, err := svc.Render(ctx, snap, f); err != nil {
			t.Fatalf("render %s under zero-network: %v", f, err)
		}
	}

	// Timeline.
	if _, err := svc.Timeline(ctx, snap); err != nil {
		t.Fatalf("timeline under zero-network: %v", err)
	}

	// Export bundle (all formats) then verify it — all offline.
	outDir := t.TempDir()
	if _, err := svc.ExportBundle(ctx, snap, allFormats(), outDir); err != nil {
		t.Fatalf("export bundle under zero-network: %v", err)
	}
	bundleDir := filepath.Join(outDir, "report")
	res, err := svc.Verify(ctx, bundleDir)
	if err != nil {
		t.Fatalf("verify under zero-network: %v", err)
	}
	if !res.OK {
		t.Fatalf("verify failed under zero-network: %+v", res.Files)
	}

	// A sanity assertion that the panicking transport is actually installed:
	// a direct use must panic. This guards against the test silently passing
	// because the hook was not wired.
	assertTransportPanics(t)
}

// assertTransportPanics confirms the installed DefaultTransport panics on use,
// so the zero-network guarantee above is meaningful rather than vacuous.
func assertTransportPanics(t *testing.T) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected the panicking transport to fire on direct use")
		}
	}()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	_, _ = http.DefaultTransport.RoundTrip(req)
}
