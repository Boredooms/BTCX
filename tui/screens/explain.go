package screens

import (
	"strings"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// explain.go holds the shared presentation state + rendering for the OPTIONAL
// local-LLM context pane (design §E.4), reused by the Wallet, Alerts, and
// Reports screens. The pane is a plain-language NARRATOR of already-computed
// evidence: it never computes or renders a risk/evidence/pattern value, and it
// is only ever populated by the explicit L / :explain action — never from
// Init/View/tick. The three screens share one implementation so the honest
// states (NIT-4 no-result, disabled, unreachable, reachable) read identically.

// explainPaneTitle is the bordered panel title for the summary pane.
const explainPaneTitle = "LLM SUMMARY (optional, local)"

// summaryPane is the ephemeral state of the LLM summary pane on a screen. It is
// zero-valued (hidden) until the user presses L / runs :explain.
type summaryPane struct {
	// active reports whether the pane should be rendered (set true the moment L
	// is pressed, so a "working…" / no-result note shows immediately).
	active bool
	// loaded reports whether a SummaryLoaded (or a NIT-4 no-result note) has
	// settled, so the "working…" placeholder no longer lingers.
	loaded bool
	// text is the summary string to render (deterministic or rephrased).
	text string
	// note is the honest provenance/degrade/no-result line, muted.
	note string
}

// requestExplain handles the explicit L / :explain action, honoring the honest
// states (design §E.4 + NIT-4):
//   - no result loaded -> NIT-4 no-op: the pane opens with note "open a subject
//     first", loaded=true (so no "working…" placeholder lingers), and NO command
//     is dispatched (no summarizer call, no network).
//   - result loaded -> the pane opens in a "working…" state and explainCmd is
//     dispatched off the UI thread; SummaryLoaded lands via acceptSummary.
//
// It NEVER computes a value: BuildSummaryInput only copies/derives presentation
// fields, and the summarizer only narrates them. The command is dispatched ONLY
// here (on the explicit action), never from Init/View/tick.
func (p *summaryPane) requestExplain(c *ScreenCtx, res *schema.InvestigationResult) tea.Cmd {
	p.active = true
	if res == nil {
		// NIT-4: no subject/result loaded — honest no-op, no summarizer call.
		p.loaded = true
		p.text = ""
		p.note = "open a subject first"
		return nil
	}
	// A result is present: open the pane in a working state and project it. The
	// projection copies/derives presentation fields only; it computes nothing. A
	// nil ScreenCtx.Summarizer is normalized to the deterministic summarizer by
	// explainCmd, so a bare ScreenCtx still narrates with no network call.
	p.loaded = false
	p.note = ""
	in := BuildSummaryInput(*res)
	var sum = c.summarizer()
	return explainCmd(c.bgCtx(), sum, in)
}

// acceptSummary folds a landed SummaryLoaded into the pane with the honest
// provenance/degrade note (design §E.4). It renders only what the command
// produced — a projection of already-computed values.
func (p *summaryPane) acceptSummary(c *ScreenCtx, m SummaryLoaded) {
	p.active = true
	p.loaded = true
	p.text = m.Text
	p.note = explainNote(c, m)
}

// renderSummaryPane renders the pane into frame f as a bordered Panel. It only
// ever renders the string already in state — it dispatches nothing.
func renderSummaryPane(styles theme.Styles, p summaryPane, f components.Frame) string {
	if f.Empty() {
		return ""
	}
	var b strings.Builder
	switch {
	case !p.loaded && p.text == "":
		b.WriteString(styles.Muted.Render("working… press L again if nothing appears"))
	default:
		if p.text != "" {
			b.WriteString(styles.Value.Render(p.text))
		}
	}
	if p.note != "" {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(styles.Muted.Render(p.note))
	}
	inner := f.H - 3
	if inner < 1 {
		inner = 1
	}
	return components.Panel(styles, explainPaneTitle,
		clampBlockLocal(b.String(), components.Frame{W: f.W - 2, H: inner}),
		f)
}

// explainNote returns the honest provenance/degrade line for a landed summary
// (design §E.4). It never asserts a value; it only describes the summary's
// origin so an analyst knows the LLM is a narrator, not a source.
func explainNote(c *ScreenCtx, m SummaryLoaded) string {
	if m.Note != "" {
		return m.Note
	}
	if m.Degraded {
		return "local LLM not reachable — deterministic summary"
	}
	// Not degraded: either the LLM was disabled (deterministic default) or a
	// local model actually rephrased it. Distinguish by cfg.
	if c == nil || c.Cfg == nil || !c.Cfg.LLM.Enabled {
		return "local LLM disabled — deterministic summary (enable in config)"
	}
	model := strings.TrimSpace(c.Cfg.LLM.Model)
	if model == "" {
		model = "local model"
	}
	return "summary generated locally by " + model + "; risk/evidence computed by BCTX"
}
