package screens

import (
	"context"
	"strings"
	"testing"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
)

// fakeAcquire is a deterministic AcquireService for the acquire-flow test. It
// records the request and returns a canned result; it dials nothing.
type fakeAcquire struct {
	avail   bool
	gotReq  app.AcquireRequest
	called  bool
	subject string
}

func (f *fakeAcquire) Available() bool      { return f.avail }
func (f *fakeAcquire) ProviderName() string { return "fake" }
func (f *fakeAcquire) Acquire(_ context.Context, req app.AcquireRequest) (app.AcquireResult, error) {
	f.called = true
	f.gotReq = req
	return app.AcquireResult{Provider: "fake", Target: req.Target, Status: "completed",
		Discovered: 3, New: 3, Subject: f.subject}, nil
}

// TestSearchAcquireFlowOffersThenAcquires drives the local-first search ->
// explicit acquire flow at the model level: an unknown block height with no
// local match offers acquisition; pressing Enter runs the (fake) acquire and
// emits a navigation to the acquired subject.
func TestSearchAcquireFlowOffersThenAcquires(t *testing.T) {
	acq := &fakeAcquire{avail: true, subject: "0000000000000000000000000000000000000000000000000000000000abcdef"}
	ctx := &ScreenCtx{Ctx: context.Background(), Styles: theme.Build(theme.Default()), Acquire: acq}
	s := NewSearch(ctx)

	// Simulate a submitted query for a block height with no local data.
	s.query = "800000"
	// No repo -> lookups can't run; drive the "all empty" path directly by
	// marking lookups done and applying an empty result set.
	s.pending = 0
	s.resultRows = rowCache{}
	s.maybeOfferAcquire()

	if s.offerStatus != "offer" {
		t.Fatalf("expected acquire offer for an online unknown subject, got %q", s.offerStatus)
	}
	if s.acquireKind != "height" {
		t.Fatalf("expected classified kind 'height', got %q", s.acquireKind)
	}

	// Press Enter -> start acquire (returns a cmd that runs the fake service).
	cmd := s.startAcquire()
	if s.offerStatus != "acquiring" {
		t.Fatalf("expected 'acquiring' state, got %q", s.offerStatus)
	}
	if cmd == nil {
		t.Fatal("startAcquire returned no command")
	}
	msg := cmd()
	done, ok := msg.(acquireDoneMsg)
	if !ok {
		t.Fatalf("expected acquireDoneMsg, got %T", msg)
	}
	if !acq.called || acq.gotReq.Kind != app.AcquireBlock || !acq.gotReq.ByHeight {
		t.Fatalf("acquire not invoked as a by-height block: %+v", acq.gotReq)
	}

	// Applying the result should set 'done' and emit a nav to the subject.
	navCmd := s.applyAcquire(done)
	if s.offerStatus != "done" {
		t.Fatalf("expected 'done', got %q", s.offerStatus)
	}
	if navCmd == nil {
		t.Fatal("expected a navigation command after acquisition")
	}
	nav, ok := navCmd().(NavSearch)
	if !ok || nav.Kind != "block" || nav.ID != acq.subject {
		t.Fatalf("expected NavSearch{block, %s}, got %+v", acq.subject, navCmd())
	}
}

// TestSearchAcquireUnavailableOffline asserts that when acquisition is NOT
// available (offline), an unknown subject shows the honest "unavailable" state
// and NEVER calls the service — no network when offline.
func TestSearchAcquireUnavailableOffline(t *testing.T) {
	acq := &fakeAcquire{avail: false}
	ctx := &ScreenCtx{Ctx: context.Background(), Styles: theme.Build(theme.Default()), Acquire: acq}
	s := NewSearch(ctx)
	s.query = "800000"
	s.pending = 0
	s.maybeOfferAcquire()
	if s.offerStatus != "unavailable" {
		t.Fatalf("expected 'unavailable' offline, got %q", s.offerStatus)
	}
	// The offer panel must render the honest offline message.
	panel := s.acquirePanel(80)
	if !strings.Contains(panel, "UNAVAILABLE") {
		t.Fatalf("offline panel should say UNAVAILABLE, got:\n%s", panel)
	}
	if acq.called {
		t.Fatal("acquire must NOT be called when offline")
	}
}

var _ = components.Frame{}
