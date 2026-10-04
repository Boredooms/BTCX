package screens

import (
	"strings"

	"github.com/bctx/bctx/app"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/tui/components"
	"github.com/bctx/bctx/tui/theme"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

// Search is the global Search screen (design §2). It resolves a typed query to
// a wallet, transaction, or IP through the indexed repository lookups
// (GetWallet / GetTransaction / NetworkObservationsByIP). It debounces on a
// minimum query length and bounds results — it never scans the full table per
// keystroke (design §2 search seam, §11.4). Selecting a result navigates to the
// matching detail screen (handled by the Root via the emitted navigation).
type Search struct {
	ctx    *ScreenCtx
	styles theme.Styles

	box        components.SearchBox
	results    components.Table
	query      string
	note       string
	resultRows rowCache

	// acquire-flow state. When a query has no local match, the screen offers an
	// explicit network acquisition (never automatic). pending tracks how many of
	// the local lookups are still outstanding so the "no match" offer only
	// appears once ALL local lookups have returned empty.
	pending     int
	offerStatus string // "" | "offer" | "acquiring" | "done" | "unavailable"
	acquireMsg  string
	acquireKind string // classified kind for the pending acquisition

	// pipeline is the live run readout shown while a subject resolves (querying
	// → classify → resolve → open). It makes the Analysis window informative
	// end-to-end instead of a bare "querying…" line.
	pipeline components.Pipeline
	runStage string // "" | "querying" | "resolving" | "resolved" | "acquire" | "none"
}

// minQueryLen is the shortest query that triggers a lookup. Below this the
// screen waits (no per-keystroke full scan).
const minQueryLen = 3

// NavSearch is emitted by the Search screen to request navigation to a result's
// detail screen. The Root translates it into a NavMsg. Defined here so the
// screens package stays tui-independent; the adapter forwards it unchanged and
// the Root recognises it.
type NavSearch struct {
	Kind string // wallet | tx | ip
	ID   string
}

// NavScreen is emitted by a screen to jump to another registered screen by its
// id (e.g. a detail screen's `g` key opening the Graph), carrying the current
// subject so the destination opens scoped to it. The Root recognises it and
// translates it into a navigation; the screens package stays tui-independent by
// using the id as a plain string. Known target: "graph".
type NavScreen struct {
	Screen  string // registry screen id (e.g. "graph")
	Subject Subject
}

// NewSearch builds the Search screen.
func NewSearch(ctx *ScreenCtx) *Search {
	cols := []table.Column{
		{Title: "kind", Width: 12},
		{Title: "id", Width: 40},
		{Title: "detail", Width: 24},
	}
	tbl := components.NewTable(ctx.styles(), cols)
	tbl.SetEmptyText("type at least 3 characters, then enter")
	s := &Search{
		ctx:      ctx,
		styles:   ctx.styles(),
		box:      components.NewSearchBox(ctx.styles(), "enter a wallet address, txid, or IP to analyze"),
		results:  tbl,
		note:     "enter a subject to run the investigation pipeline",
		pipeline: components.NewPipeline(ctx.styles(), "RUN"),
	}
	s.pipeline.SetStages(s.idleStages())
	return s
}

// analysisStageNames are the ordered stages of the Analysis-window run.
var analysisStageNames = []string{
	"accept subject",
	"classify (wallet / tx / IP / block)",
	"resolve in local case",
	"open investigation",
}

// idleStages is the all-pending readout before a query is entered.
func (s *Search) idleStages() []components.PipelineStage {
	out := make([]components.PipelineStage, len(analysisStageNames))
	for i, n := range analysisStageNames {
		out[i] = components.PipelineStage{Label: n, Status: components.StagePending}
	}
	return out
}

// runStages reflects the live run state from s.runStage + what resolved, so the
// Analysis window shows real progress (never fabricated). resolvedKind/ID are
// shown as honest detail when known.
func (s *Search) runStages() []components.PipelineStage {
	done := components.StageDone
	run := components.StageRunning
	pend := components.StagePending
	st := func(status components.StageStatus, detail string) components.PipelineStage {
		return components.PipelineStage{Status: status, Detail: detail}
	}

	stages := s.idleStages()
	switch s.runStage {
	case "querying":
		stages[0].Status, stages[0].Detail = done, shortID(s.query)
		stages[1].Status = done
		stages[2].Status, stages[2].Detail = run, "searching wallet / tx / IP indexes…"
		stages[3].Status = pend
	case "resolved":
		stages[0].Status, stages[0].Detail = done, shortID(s.query)
		stages[1].Status = done
		stages[2].Status, stages[2].Detail = done, "found in local case"
		stages[3].Status, stages[3].Detail = done, "opening — the pipeline runs on the subject screen"
	case "acquire":
		stages[0].Status, stages[0].Detail = done, shortID(s.query)
		stages[1].Status, stages[1].Detail = done, s.acquireKind
		stages[2].Status, stages[2].Detail = done, "not in local case"
		stages[3] = st(run, "acquire offered — [Enter] fetch, [L] local-only")
	case "none":
		stages[0].Status, stages[0].Detail = done, shortID(s.query)
		stages[1].Status, stages[1].Detail = done, "unclassified"
		stages[2].Status, stages[2].Detail = done, "no local match"
		stages[3] = st(components.StageSkipped, "enter a valid wallet / txid / IP")
	default:
		_ = st
	}
	return stages
}

// Init focuses the search box so typing starts immediately.
func (s *Search) Init() tea.Cmd { return s.box.Focus() }

// Seed pre-fills the search box with a query (from the global `/` overlay) and
// kicks off the bounded lookup immediately so navigating from the overlay lands
// on populated results, not an empty box (design §B.2). A query below the
// minimum length just pre-fills the box and shows the honest "too short" note.
func (s *Search) Seed(query string) tea.Cmd {
	q := strings.TrimSpace(query)
	s.box.SetValue(q)
	focus := s.box.Focus()
	if len([]rune(q)) < minQueryLen {
		s.query = q
		s.note = "query too short (min 3 chars)"
		s.results.SetEmptyText(s.note)
		s.results.SetRows(nil, nil, nil)
		return focus
	}
	s.query = q
	s.note = "analyzing — resolving subject in the active case"
	return tea.Batch(focus, s.lookup(q))
}

// Focused reports whether the search input owns the keyboard (so the Root can
// suppress single-letter nav while typing — design §5).
func (s *Search) Focused() bool { return s.box.Focused() }

// Update drives the search box, dispatches a bounded lookup on submit, and folds
// in the result.
func (s *Search) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch m := msg.(type) {
	case components.SearchSubmitted:
		s.query = strings.TrimSpace(m.Query)
		s.offerStatus = ""
		s.acquireMsg = ""
		if len([]rune(s.query)) < minQueryLen {
			s.note = "query too short (min 3 chars)"
			s.results.SetEmptyText(s.note)
			s.results.SetRows(nil, nil, nil)
			return s, nil
		}
		return s, s.lookup(s.query)
	case dataLoaded:
		return s, s.applyResult(m)
	case dataError:
		s.results.SetError(m.Err.Error())
		return s, nil
	case acquireDoneMsg:
		return s, s.applyAcquire(m)
	case components.RowSelected:
		return s, func() tea.Msg { return NavSearch{Kind: m.Kind, ID: m.ID} }
	case tea.KeyMsg:
		// In the "offer" state the whole screen is the acquire prompt: Enter
		// acquires (explicit consent), L searches local-only (dismiss), Esc
		// cancels the offer. These take precedence over box/table keys.
		if s.offerStatus == "offer" {
			switch m.String() {
			case "enter":
				return s, s.startAcquire()
			case "l", "L", "esc":
				s.offerStatus = ""
				s.note = "local-only — no network acquisition"
				return s, nil
			}
			return s, nil
		}
		if s.offerStatus == "acquiring" {
			return s, nil // acquisition in flight; ignore keys until it returns
		}
		// When the box is focused, editing keys go to it; otherwise navigation
		// keys drive the results table.
		if s.box.Focused() {
			var cmd tea.Cmd
			s.box, cmd = s.box.Update(m)
			return s, cmd
		}
		var cmd tea.Cmd
		s.results, cmd = s.results.Update(m)
		return s, cmd
	}
	// Non-key messages still flow to the box (textinput blink etc.).
	var cmd tea.Cmd
	s.box, cmd = s.box.Update(msg)
	return s, cmd
}

// lookup fetches the three indexed candidates concurrently (bounded). Each is a
// cancellable command; results arrive as dataLoaded keyed by the query.
func (s *Search) lookup(q string) tea.Cmd {
	repo := s.ctx.repo()
	if repo == nil {
		s.results.SetError(errNoCase.Error())
		return nil
	}
	s.resultRows = rowCache{}
	s.pending = 3 // wallet + tx + ip lookups outstanding
	s.offerStatus = ""
	s.runStage = "querying"
	s.pipeline.SetStages(s.runStages())
	s.results.SetLoading()
	return tea.Batch(
		walletCmd(s.ctx.bgCtx(), repo, q),
		transactionCmd(s.ctx.bgCtx(), repo, q),
		observationsByIPCmd(s.ctx.bgCtx(), repo, q),
	)
}

// applyResult accumulates whichever lookups matched into the results table.
// Only non-nil hits become rows (honest: nothing fabricated). It returns a
// navigation command when all lookups are done and a single wallet/tx match can
// auto-open to run the pipeline, else nil.
func (s *Search) applyResult(m dataLoaded) tea.Cmd {
	if m.Request != s.query {
		return nil // stale result from a superseded query
	}
	// Merge with any already-displayed rows so the three async hits accumulate.
	rows, ids, kinds := s.existingRows()
	add := func(kind, id, detail string) {
		// Dedup: skip an id already present (a query can match more than one
		// lookup kind, but we show each distinct id once per kind).
		for i := range ids {
			if ids[i] == id && kinds[i] == kind {
				return
			}
		}
		rows = append(rows, table.Row{kind, shortID(id), detail})
		ids = append(ids, id)
		kinds = append(kinds, kind)
	}

	switch p := m.Payload.(type) {
	case *schema.Wallet:
		if p != nil && p.Address != "" {
			add("wallet", p.Address, "indexed wallet")
		}
	case *schema.Transaction:
		if p != nil && p.TxID != "" {
			add("tx", p.TxID, "indexed transaction")
		}
	case []schema.NetworkObservation:
		if len(p) > 0 {
			add("ip", s.query, "network observations")
		}
	}
	s.resultRows = rowCache{rows: rows, ids: ids, kinds: kinds}
	if m.Request == s.query && s.pending > 0 {
		s.pending--
	}
	if len(rows) == 0 {
		s.results.SetEmptyText("no wallet, tx, or IP match for " + shortID(s.query))
		// Once ALL local lookups have returned empty, offer explicit network
		// acquisition (never automatic). Only if a subject classifies and the
		// acquire seam is available + online.
		if s.pending == 0 {
			s.maybeOfferAcquire()
		}
	}
	s.results.SetRows(rows, ids, kinds)

	// Update the live run readout once all lookups are in.
	if s.pending == 0 {
		switch {
		case len(rows) > 0:
			s.runStage = "resolved"
		case s.offerStatus == "offer" || s.offerStatus == "unavailable":
			s.runStage = "acquire"
		default:
			s.runStage = "none"
		}
		s.pipeline.SetStages(s.runStages())
	}

	// Auto-run the pipeline: when ALL local lookups have returned and there is
	// exactly ONE wallet/tx match, open it directly so the investigation
	// pipeline runs without a second keypress (the Analysis window IS the
	// run entry point). Multiple matches keep the picker so the user chooses.
	if s.pending == 0 && len(rows) == 1 && (kinds[0] == "wallet" || kinds[0] == "tx") {
		id, kind := ids[0], kinds[0]
		return func() tea.Msg { return NavSearch{Kind: kind, ID: id} }
	}
	return nil
}

// maybeOfferAcquire decides whether to present the explicit acquire prompt for
// a query with no local match. It classifies the subject; if it is a valid
// wallet/tx/block/height AND an acquire seam exists AND it is online, it offers.
// Offline (or no seam) shows an honest "acquisition unavailable" note instead —
// never a "fetching" spinner (design: no network when offline).
func (s *Search) maybeOfferAcquire() {
	kind, ok := classifySubject(s.query)
	if !ok {
		s.offerStatus = ""
		return
	}
	s.acquireKind = kind
	if s.ctx == nil || s.ctx.Acquire == nil || !s.ctx.Acquire.Available() {
		s.offerStatus = "unavailable"
		return
	}
	s.offerStatus = "offer"
}

// startAcquire runs the explicit, user-confirmed acquisition as a cancellable
// command off the UI thread. The ctx is the program context so Esc/quit cancels
// it. After this returns the data is LOCAL and all further processing is offline.
func (s *Search) startAcquire() tea.Cmd {
	if s.ctx == nil || s.ctx.Acquire == nil {
		return nil
	}
	s.offerStatus = "acquiring"
	req := acquireRequestFor(s.acquireKind, s.query)
	acq := s.ctx.Acquire
	ctx := s.ctx.bgCtx()
	q := s.query
	return func() tea.Msg {
		res, err := acq.Acquire(ctx, req)
		return acquireDoneMsg{query: q, res: res, err: err}
	}
}

// applyAcquire folds the acquisition result: on success it navigates to the
// now-local subject's detail screen (all offline from here); on failure it
// shows an honest error and returns to the offer so the user can retry/dismiss.
func (s *Search) applyAcquire(m acquireDoneMsg) tea.Cmd {
	if m.query != s.query {
		return nil // stale
	}
	if m.err != nil {
		s.offerStatus = "offer"
		s.acquireMsg = "acquisition failed: " + m.err.Error()
		return nil
	}
	s.offerStatus = "done"
	s.acquireMsg = acquireSummary(m.res)
	// Open the acquired subject locally (no network from here on).
	navKind := s.acquireKind
	if navKind == "height" {
		navKind = "block"
	}
	id := m.res.Subject
	if id == "" {
		id = s.query
	}
	return func() tea.Msg { return NavSearch{Kind: navKind, ID: id} }
}

// acquireDoneMsg carries the result of an explicit acquisition back to Search.
type acquireDoneMsg struct {
	query string
	res   app.AcquireResult
	err   error
}

// acquireRequestFor builds an app.AcquireRequest from a classified subject kind
// and the raw query. "height" is a block-by-height acquisition.
func acquireRequestFor(kind, query string) app.AcquireRequest {
	switch kind {
	case SubjectTx:
		return app.AcquireRequest{Kind: app.AcquireTx, Target: query}
	case SubjectBlock:
		return app.AcquireRequest{Kind: app.AcquireBlock, Target: query}
	case "height":
		h := 0
		for _, c := range query {
			if c < '0' || c > '9' {
				h = 0
				break
			}
			h = h*10 + int(c-'0')
		}
		return app.AcquireRequest{Kind: app.AcquireBlock, Height: h, ByHeight: true}
	default: // wallet / ip-as-address fallback -> wallet history
		return app.AcquireRequest{Kind: app.AcquireWallet, Target: query}
	}
}

// acquireSummary is the honest one-line completion summary for the modal.
func acquireSummary(r app.AcquireResult) string {
	status := strings.ToUpper(r.Status)
	return status + ": discovered " + itoa(r.Discovered) + ", new " + itoa(r.New) +
		", duplicates " + itoa(r.Duplicates) + ", partial " + itoa(r.Partial) +
		" via " + r.Provider
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// rowCache keeps the accumulated results so the three async lookups merge.
type rowCache struct {
	rows  []table.Row
	ids   []string
	kinds []string
}

func (s *Search) existingRows() ([]table.Row, []string, []string) {
	return s.resultRows.rows, s.resultRows.ids, s.resultRows.kinds
}

// View renders the Analysis window: a titled input panel, a live RUN pipeline
// panel, and a results panel — all bordered and height-filling so the window is
// informative end-to-end and never leaves an empty cut-off region. The acquire
// prompt replaces the results panel when a subject is not found locally.
func (s *Search) View(f components.Frame) string {
	if f.Empty() {
		return ""
	}
	title := s.styles.Title.Render("ANALYSIS") + " " +
		s.styles.Label.Render("live investigation run")
	bodyH := f.H - 1
	if bodyH < 1 {
		bodyH = 1
	}

	// Three stacked regions separated by 2 newlines; reserve those 2 rows.
	sep := 2
	usable := bodyH - sep
	if usable < 6 {
		// Too short for the full layout: just the input + note (honest degrade).
		short := s.box.View(components.Frame{W: f.W, H: 1}) + "\n" + s.styles.Muted.Render(s.note)
		return clampBlockLocal(title+"\n"+short, f)
	}

	// Input panel (box + note) is a fixed small height; pipeline + results split
	// the remainder.
	inputH := 4
	rest := usable - inputH
	// stages + the panel's own "RUN" title row + border(2) + the panel title row.
	pipeH := len(analysisStageNames) + 1 + 3
	if pipeH > rest-3 {
		pipeH = rest - 3
	}
	resH := rest - pipeH
	if resH < 3 {
		resH = 3
	}

	inputBody := s.box.View(components.Frame{W: f.W - 2, H: 1}) + "\n" +
		s.styles.Muted.Render(s.note)
	inputPanel := components.Panel(s.styles, "SUBJECT",
		inputBody, components.Frame{W: f.W, H: inputH})

	pipePanel := components.Panel(s.styles, "PIPELINE",
		s.pipeline.View(components.Frame{W: f.W - 2, H: paneInner(pipeH)}),
		components.Frame{W: f.W, H: pipeH})

	// Lower region: the acquire prompt (when a subject needs fetching) OR the
	// results list.
	var lower string
	if ap := s.acquireBody(); ap != "" {
		lower = components.Panel(s.styles, "ACQUIRE", ap, components.Frame{W: f.W, H: resH})
	} else {
		lower = components.Panel(s.styles, "RESULTS",
			s.results.View(components.Frame{W: f.W - 2, H: paneInner(resH)}),
			components.Frame{W: f.W, H: resH})
	}

	out := title + "\n" + inputPanel + "\n" + pipePanel + "\n" + lower
	return clampBlockLocal(out, f)
}

// acquireBody returns the acquire prompt body (plain, for embedding in the
// lower panel) or "" when there is no acquire flow active.
func (s *Search) acquireBody() string {
	switch s.offerStatus {
	case "offer":
		prov := "network"
		if s.ctx != nil && s.ctx.Acquire != nil {
			prov = s.ctx.Acquire.ProviderName()
		}
		body := s.styles.Role(theme.RoleValue).Render(
			"Not found in local case.  NETWORK: ONLINE · PROVIDER: "+prov) + "\n" +
			s.styles.Label.Render("[Enter] Acquire from network   [L] Local only   [Esc] Cancel")
		if s.acquireMsg != "" {
			body += "\n" + s.styles.Role(theme.RoleWarning).Render(s.acquireMsg)
		}
		return body
	case "acquiring":
		return s.styles.Role(theme.RoleInfo).Render(
			"Acquiring " + s.acquireKind + " " + shortID(s.query) + " … (Esc/Ctrl+C cancels)")
	case "done":
		return s.styles.Role(theme.RoleHealthy).Render(s.acquireMsg) + "\n" +
			s.styles.Muted.Render("data is now local — opening investigation (offline)")
	case "unavailable":
		return s.styles.Role(theme.RoleWarning).Render(
			"No local match.  NETWORK: OFFLINE · ACQUISITION: UNAVAILABLE") + "\n" +
			s.styles.Label.Render("Connect and retry, or analyze local data only.")
	}
	return ""
}

// acquirePanel renders the explicit acquire prompt / progress / result for a
// query with no local match. It is honest about offline state and never shows a
// "fetching" affordance when acquisition is unavailable.
func (s *Search) acquirePanel(w int) string {
	switch s.offerStatus {
	case "offer":
		prov := "network"
		if s.ctx != nil && s.ctx.Acquire != nil {
			prov = s.ctx.Acquire.ProviderName()
		}
		body := s.styles.Role(theme.RoleValue).Render(
			"Not found in local case.  NETWORK: ONLINE · PROVIDER: "+prov) + "\n" +
			s.styles.Label.Render("[Enter] Acquire from network   [L] Local only   [Esc] Cancel")
		if s.acquireMsg != "" {
			body += "\n" + s.styles.Role(theme.RoleWarning).Render(s.acquireMsg)
		}
		return components.Panel(s.styles, "ACQUIRE SUBJECT", body,
			components.Frame{W: w, H: 6})
	case "acquiring":
		body := s.styles.Role(theme.RoleInfo).Render(
			"Acquiring " + s.acquireKind + " " + shortID(s.query) + " … (Esc/Ctrl+C cancels)")
		return components.Panel(s.styles, "ACQUIRING", body, components.Frame{W: w, H: 4})
	case "done":
		body := s.styles.Role(theme.RoleHealthy).Render(s.acquireMsg) + "\n" +
			s.styles.Muted.Render("data is now local — opening investigation (offline)")
		return components.Panel(s.styles, "ACQUISITION COMPLETE", body, components.Frame{W: w, H: 5})
	case "unavailable":
		body := s.styles.Role(theme.RoleWarning).Render(
			"No local match.  NETWORK: OFFLINE · ACQUISITION: UNAVAILABLE") + "\n" +
			s.styles.Label.Render("Connect and retry, or search local data only.")
		return components.Panel(s.styles, "ACQUIRE UNAVAILABLE", body, components.Frame{W: w, H: 5})
	}
	return ""
}

// ShortHelp lists Search's context keys.
func (s *Search) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search / open")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "unfocus / back")),
	}
}
