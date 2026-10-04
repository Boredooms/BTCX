package theme

import "testing"

func TestDefaultResolvesAllRoles(t *testing.T) {
	th := Default()
	if th.Degraded {
		t.Fatal("Default() must not be degraded")
	}
	roles := []Role{
		RoleTitle, RoleLabel, RoleValue, RoleInfo, RoleHealthy,
		RoleWarning, RoleCritical, RoleNeutral, RoleSelected, RoleMuted,
	}
	for _, r := range roles {
		if _, ok := th.Role[r]; !ok {
			t.Errorf("role %q not mapped in Default()", r)
		}
		// Color() must always resolve to a non-empty adaptive color.
		c := th.Color(r)
		if c.Dark == "" || c.Light == "" {
			t.Errorf("role %q resolved to an incomplete adaptive color: %+v", r, c)
		}
	}
}

func TestColorFallbackForUnmappedRole(t *testing.T) {
	th := Default()
	got := th.Color(Role("does-not-exist"))
	if got != Palette.TextPrimary {
		t.Errorf("unmapped role should fall back to TextPrimary, got %+v", got)
	}
}

func TestDegradedProfile(t *testing.T) {
	th := Degraded()
	if !th.Degraded {
		t.Fatal("Degraded() must set Degraded=true")
	}
	// Degraded keeps the full semantic role set so callers need no special case.
	for _, r := range []Role{RoleTitle, RoleHealthy, RoleCritical} {
		if _, ok := th.Role[r]; !ok {
			t.Errorf("degraded profile missing role %q", r)
		}
	}
	// The degraded border must differ from the default (ASCII vs rounded).
	if th.Border.TopLeft == Default().Border.TopLeft {
		t.Errorf("degraded border should differ from default; both have TopLeft=%q", th.Border.TopLeft)
	}
}

func TestSeverityRoleMapping(t *testing.T) {
	cases := map[string]Role{
		"HIGH":     RoleCritical,
		"critical": RoleCritical,
		"MED":      RoleWarning,
		"elevated": RoleWarning,
		"LOW":      RoleHealthy,
		"unknown":  RoleNeutral,
		"":         RoleNeutral,
	}
	for in, want := range cases {
		if got := SeverityRole(in); got != want {
			t.Errorf("SeverityRole(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildStylesDoesNotPanic(t *testing.T) {
	for _, th := range []Theme{Default(), Degraded()} {
		s := Build(th)
		if s.Theme().Degraded != th.Degraded {
			t.Error("Build did not preserve theme")
		}
		_ = s.SeverityChip("HIGH", "CRITICAL")
		_ = s.Badge(RoleInfo, "nodes 1/5000")
		_ = s.Panel.Render("x")
	}
}
