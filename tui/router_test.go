package tui

import "testing"

// The registry mirrors the design §2 table plus the Phase 8.5 Block Detail
// screen and the Extension / Knowledge Manager tab: 15 Nav=true sections +
// three contextual screens (help, timeline, block) = 18 rows. Block is
// contextual (reached from Search/palette after a block hash/height is
// acquired), with hotkey "B"; Extensions is a Nav section with hotkey "x".
func TestRegistryMatchesDesignTable(t *testing.T) {
	if got := len(Screens()); got != 18 {
		t.Fatalf("registry should have 18 rows (16 + Block + Extensions), got %d", got)
	}
}

func TestEveryScreenHasKeyHelpFactory(t *testing.T) {
	for _, d := range Screens() {
		if d.Key == "" {
			t.Errorf("screen %q has no nav key", d.ID)
		}
		if d.Help == "" {
			t.Errorf("screen %q has no help text", d.ID)
		}
		if d.New == nil {
			t.Errorf("screen %q has no factory", d.ID)
		}
		if d.Title == "" {
			t.Errorf("screen %q has no title", d.ID)
		}
	}
}

func TestNavScreensAreReachableAndNamed(t *testing.T) {
	navs := NavScreens()
	if len(navs) != 15 {
		t.Fatalf("expected 15 nav screens (14 + Extensions), got %d", len(navs))
	}
	for _, d := range navs {
		if !d.Nav {
			t.Errorf("NavScreens returned a non-nav screen %q", d.ID)
		}
		if LookupKey(d.Key) == nil {
			t.Errorf("nav screen %q key %q not resolvable", d.ID, d.Key)
		}
	}
}

func TestContextualScreensAreNotNav(t *testing.T) {
	for _, id := range []ScreenID{ScreenHelp, ScreenTimeline} {
		def := Lookup(id)
		if def == nil {
			t.Fatalf("contextual screen %q missing", id)
		}
		if def.Nav {
			t.Errorf("screen %q must be Nav=false (contextual)", id)
		}
	}
}

func TestBuildFromRegistryProducesScreen(t *testing.T) {
	ctx := &AppCtx{}
	for _, d := range Screens() {
		s := Build(d.ID, ctx)
		if s == nil {
			t.Errorf("Build(%q) returned nil", d.ID)
			continue
		}
		// A stub screen must render without panic inside a frame and empty.
		if out := s.View(Frame{W: 20, H: 3}); out == "" {
			t.Errorf("Build(%q).View produced empty output for a sized frame", d.ID)
		}
		if out := s.View(Frame{W: 0, H: 0}); out != "" {
			t.Errorf("Build(%q).View should be empty for an empty frame", d.ID)
		}
	}
}

func TestUnregisteredLookupIsNil(t *testing.T) {
	if Lookup("nope") != nil {
		t.Error("Lookup of unregistered id should be nil")
	}
	if Build("nope", &AppCtx{}) != nil {
		t.Error("Build of unregistered id should be nil")
	}
}

func TestNavJumpKeysMatchRegistry(t *testing.T) {
	keys := NavJumpKeys()
	if len(keys) != 18 {
		t.Errorf("expected one jump key per registry screen (18 incl. Block + Extensions), got %d", len(keys))
	}
	for _, k := range keys {
		if LookupKey(k) == nil {
			t.Errorf("jump key %q not registered", k)
		}
	}
}
