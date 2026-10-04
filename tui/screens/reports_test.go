package screens

import (
	"strings"
	"testing"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
	"github.com/bctx/bctx/tui/components"
)

// driveReports runs a Reports screen through Init + its async list load.
func driveReports(t *testing.T, ctx *ScreenCtx) *Reports {
	t.Helper()
	r := NewReports(ctx)
	if cmd := r.Init(); cmd != nil {
		for _, msg := range runBatch(cmd) {
			var m Model = r
			m, _ = m.Update(msg)
			r = m.(*Reports)
		}
	}
	return r
}

// TestReportsListAndSelectPreview proves the report picker end to end: a
// persisted report appears in the list, and selecting its row (Enter ->
// RowSelected) loads + previews the stored metadata. It persists the report row
// directly through the repo so the test does not depend on ML model files being
// installed in the test HOME (the build path is exercised by the CLI/offline
// gates); the SCREEN behaviour under test is list + select + preview.
func TestReportsListAndSelectPreview(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})

	// Persist a report row directly so the list has a real, selectable entry.
	row := sdk.ReportRow{
		ID:                  "rpt-fixture-WFIXTURE-abc123",
		CaseID:              "fixture-case",
		Version:             1,
		Subject:             "WFIXTURE",
		SubjectType:         "wallet",
		ReportSchemaVersion: "report-schema-v1",
		GeneratedAt:         "2024-01-02T03:04:05Z",
		GeneratedBy:         "bctx test",
		SnapshotSHA256:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		ResultJSON:          "{}",
	}
	if err := ctx.repo().SaveReport(ctx.bgCtx(), row); err != nil {
		t.Fatalf("persist report row: %v", err)
	}

	r := driveReports(t, ctx)

	// The listed report row should render in the table.
	listOut := r.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(listOut, shortID(row.ID)) {
		t.Errorf("report list should show the persisted report; got:\n%s", listOut)
	}

	// Select it (Enter on the table -> RowSelected) and preview.
	_, selCmd := r.Update(components.RowSelected{ID: row.ID, Kind: "report"})
	if selCmd == nil {
		t.Fatal("selecting a report row should load its preview")
	}
	for _, msg := range runBatch(selCmd) {
		var m Model = r
		m, _ = m.Update(msg)
		r = m.(*Reports)
	}

	out := r.View(components.Frame{W: 160, H: 50})
	for _, want := range []string{"selected report", "report-schema-v1", "bctx test"} {
		if !strings.Contains(out, want) {
			t.Errorf("preview of the selected report missing %q; got:\n%s", want, out)
		}
	}
}

// TestReportsViewRendersContent proves the in-TUI report content viewer: after
// selecting a report, pressing 'v' renders the report BODY (not just metadata)
// via the offline reporting seam, and 'f' switches the format. The rendered
// bytes are shown in a scrollable pane.
func TestReportsViewRendersContent(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	row := sdk.ReportRow{
		ID: "rpt-view-abc", CaseID: "fixture-case", Version: 1,
		Subject: "WFIXTURE", SubjectType: "wallet",
		ReportSchemaVersion: "report-schema-v1", GeneratedAt: "2024-01-02T03:04:05Z",
		GeneratedBy: "bctx test",
		ResultJSON:  `{"subject":"WFIXTURE","subject_type":"wallet"}`,
	}
	if err := ctx.repo().SaveReport(ctx.bgCtx(), row); err != nil {
		t.Fatalf("persist: %v", err)
	}
	r := driveReports(t, ctx)
	// Press 'v' DIRECTLY on the highlighted row — no separate Enter/select step.
	// This is the fix: v views whatever report the cursor is on.
	_, vcmd := r.Update(keyMsg("v"))
	if vcmd == nil {
		t.Fatal("'v' should render the report content")
	}
	for _, msg := range runBatch(vcmd) {
		var m Model = r
		m, _ = m.Update(msg)
		r = m.(*Reports)
	}
	if !r.viewing || len(r.viewLines) == 0 {
		t.Fatalf("viewer should be active with rendered lines; viewing=%v lines=%d", r.viewing, len(r.viewLines))
	}
	out := r.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out, "REPORT CONTENT [MD]") {
		t.Errorf("viewer title should show the MD format; got:\n%s", firstLines(out, 3))
	}
	// Switch to JSON and confirm the format label updates.
	_, fcmd := r.Update(keyMsg("f"))
	for _, msg := range runBatch(fcmd) {
		var m Model = r
		m, _ = m.Update(msg)
		r = m.(*Reports)
	}
	if r.viewFormat != schema.FormatJSON {
		t.Errorf("'f' should cycle to JSON, got %q", r.viewFormat)
	}
}

// firstLines returns the first n lines of s for compact test output.
func firstLines(s string, n int) string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}

// TestReportsInlineSubjectControl proves the Reports page is self-contained:
// pressing i focuses the subject input, a valid submitted subject triggers a
// build (a command), and an unrecognized one shows an honest note (no build).
func TestReportsInlineSubjectControl(t *testing.T) {
	ctx := fixtureCtx(t, Subject{}) // no preset subject
	r := driveReports(t, ctx)

	// i focuses the inline subject box.
	_, _ = r.Update(keyMsg("i"))
	if !r.Focused() {
		t.Fatal("pressing i should focus the subject input")
	}

	// A valid wallet subject submitted should trigger a build command and set
	// the subject.
	_, cmd := r.Update(components.SearchSubmitted{Query: "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF"})
	if cmd == nil {
		t.Fatal("a valid submitted subject should start a build")
	}
	if r.subject != "1FeexV6bAHb8ybZjqQMjJrcCrHGW9sb6uF" {
		t.Errorf("subject not set from inline input, got %q", r.subject)
	}
	if r.Focused() {
		t.Error("box should blur after submit")
	}

	// An unrecognized subject shows an honest note and does NOT build.
	_, cmd2 := r.Update(components.SearchSubmitted{Query: "???"})
	if cmd2 != nil {
		t.Error("an invalid subject must not start a build")
	}
	out := r.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out, "unrecognized") {
		t.Errorf("invalid subject should show an honest note; got:\n%s", firstLines(out, 2))
	}
}

// TestReportsSelectMissingIsHonest asserts selecting a non-existent report id
// surfaces an honest error rather than fabricating a preview.
func TestReportsSelectMissingIsHonest(t *testing.T) {
	ctx := fixtureCtx(t, Subject{ID: "WFIXTURE", Kind: SubjectWallet})
	r := driveReports(t, ctx)
	_, cmd := r.Update(components.RowSelected{ID: "does-not-exist", Kind: "report"})
	if cmd == nil {
		t.Fatal("selecting a row should attempt a load")
	}
	for _, msg := range runBatch(cmd) {
		var m Model = r
		m, _ = m.Update(msg)
		r = m.(*Reports)
	}
	out := r.View(components.Frame{W: 160, H: 50})
	if !strings.Contains(out, "not found") {
		t.Errorf("missing report should surface an honest not-found error; got:\n%s", out)
	}
}
