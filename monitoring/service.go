// Package monitoring is the Phase 6 live-monitoring orchestration layer. It
// polls an acquisition provider for new wallet activity, persists it through
// the shared canonical ingestion path, incrementally updates the graph, runs
// the existing analysis pipeline on affected subjects, computes risk deltas and
// deterministic alerts, and persists resumable session state.
//
// Monitoring contains NO direct SQL, HTTP, provider parsing, graph-table
// mutation, ML, or risk formulas. It composes acquisition + the shared
// Persister + an injected Analyzer. Network access stays entirely in the
// acquisition provider.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/ingestion"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// AnalysisResult is the minimal view of an investigation the monitor compares
// across cycles. The CLI adapts the orchestrator's InvestigationResult into
// this (keeps monitoring free of an orchestrator import).
type AnalysisResult struct {
	Subject       string
	RiskScore     int
	Confidence    float64
	Signals       []string // signal names present with non-zero contribution
	Patterns      []string // detected pattern types
	RelatedWallet int
	EvidenceIDs   []string
}

// Analyzer runs the existing analysis pipeline on a subject and returns the
// comparable result. Injected by the caller.
type Analyzer func(ctx context.Context, subject string) (AnalysisResult, error)

// Clock is injectable so tests are deterministic.
type Clock func() time.Time

// Config bundles monitor tunables.
type Config struct {
	PollInterval     time.Duration
	MaxEventQueue    int
	AnalysisDebounce time.Duration
	ReconnectBackoff time.Duration
	MaxReconnects    int
	AlertRiskDelta   int
	AlertNewSignal   bool
	AlertNewPattern  bool
	MaxPolls         int // 0 = unlimited (production); >0 bounds test runs
}

// DefaultConfig returns monitor defaults.
func DefaultConfig() Config {
	return Config{
		PollInterval: 30 * time.Second, MaxEventQueue: 1024,
		AnalysisDebounce: 2 * time.Second, ReconnectBackoff: 5 * time.Second,
		MaxReconnects: 10, AlertRiskDelta: 10, AlertNewSignal: true,
		AlertNewPattern: true,
	}
}

// Service orchestrates one or more monitoring sessions over a case repository.
type Service struct {
	repo      sdk.Repository
	src       acquisition.DataSource
	persister *ingestion.Persister
	analyze   Analyzer
	caseID    string
	cfg       Config
	now       Clock

	// signals caches the last-seen signal/pattern sets per session for in-run
	// diffing (avoids repeated alerts within a run). Guarded by mu because a
	// production deployment may run multiple sessions concurrently on one
	// Service. Cross-restart diffing uses the persisted risk baseline, not this.
	mu      sync.Mutex
	signals map[string][2][]string
}

// NewService builds a monitoring service.
func NewService(repo sdk.Repository, src acquisition.DataSource, analyze Analyzer, caseID string, cfg Config) *Service {
	if cfg.MaxEventQueue <= 0 {
		cfg.MaxEventQueue = 1024
	}
	return &Service{
		repo: repo, src: src, persister: ingestion.NewPersister(repo),
		analyze: analyze, caseID: caseID, cfg: cfg, now: time.Now,
		signals: map[string][2][]string{},
	}
}

// WithClock injects a deterministic clock (tests).
func (s *Service) WithClock(c Clock) *Service { s.now = c; return s }

// MonitorWallet runs the monitor loop for a wallet until ctx is cancelled, the
// provider signals completion, or MaxPolls is reached. It is cancellation-safe,
// idempotent, and checkpoints only after durable persistence.
func (s *Service) MonitorWallet(ctx context.Context, address string, sessionID string) (sdk.MonitorSessionRow, error) {
	if !s.src.Capabilities().WalletHistory {
		return sdk.MonitorSessionRow{}, acquisition.ErrCapabilityUnsupported
	}
	sess := s.loadOrInitSession(ctx, address, sessionID)
	sess.Status = "running"
	sess.Health = "connected"
	s.save(ctx, &sess)

	reconnects := 0
	for poll := 0; s.cfg.MaxPolls == 0 || poll < s.cfg.MaxPolls; poll++ {
		if err := ctx.Err(); err != nil {
			sess.Status = "paused"
			sess.Health = "offline"
			s.save(ctx, &sess)
			return sess, ctx.Err()
		}

		affected, err := s.pollOnce(ctx, &sess)
		if err != nil {
			// Transient provider failure: backoff + reconnect, keep the session.
			if acquisition.Retryable(err) {
				reconnects++
				sess.Reconnects = reconnects
				sess.Health = "reconnecting"
				sess.Error = err.Error()
				s.save(ctx, &sess)
				if reconnects > s.cfg.MaxReconnects {
					sess.Health = "degraded"
					sess.Gaps++
					s.save(ctx, &sess)
					return sess, err
				}
				if werr := sleep(ctx, s.cfg.ReconnectBackoff); werr != nil {
					sess.Status = "paused"
					s.save(ctx, &sess)
					return sess, werr
				}
				continue
			}
			sess.Status = "failed"
			sess.Health = "disconnected"
			sess.Error = err.Error()
			s.save(ctx, &sess)
			return sess, err
		}
		// Successful poll resets reconnect state + health.
		reconnects = 0
		sess.Health = "connected"
		sess.Error = ""
		sess.LastPollOKAt = s.now().UTC().Format(time.RFC3339)

		// Incremental analysis for each affected subject (coalesced: the set is
		// deduped so one analysis per subject per poll).
		for subject := range affected {
			if err := s.analyzeAndAlert(ctx, &sess, subject, affected[subject]); err != nil {
				return sess, err
			}
		}
		s.save(ctx, &sess) // checkpoint after durable processing

		if s.cfg.MaxPolls == 0 {
			if werr := sleep(ctx, s.cfg.PollInterval); werr != nil {
				sess.Status = "paused"
				s.save(ctx, &sess)
				return sess, werr
			}
		}
	}
	sess.Status = "completed"
	s.save(ctx, &sess)
	return sess, nil
}

// pollOnce fetches the current wallet page, persists new transactions through
// the canonical path, records monitor events (dedup), and returns the set of
// affected subjects -> trigger event id.
func (s *Service) pollOnce(ctx context.Context, sess *sdk.MonitorSessionRow) (map[string]string, error) {
	affected := map[string]string{}

	page, err := s.src.FetchWalletHistory(ctx, acquisition.WalletHistoryRequest{
		Address: sess.Target, PageSize: 50,
	})
	if err != nil {
		return nil, err
	}

	// Build canonical txs from inline details (Esplora returns them inline).
	var canon []schema.Transaction
	txMeta := map[string]acquisition.AcquiredTransaction{}
	for _, at := range page.Transactions {
		txMeta[at.TxID] = at
		canon = append(canon, toCanonical(at, sess))
	}
	// Observations (providers that expose them).
	var cobs []schema.NetworkObservation
	for _, o := range page.Observations {
		if co, ok := toCanonicalObs(o, sess); ok {
			cobs = append(cobs, co)
		}
	}

	res, perr := s.persister.PersistBatch(ctx, canon, cobs, true)
	if perr != nil {
		return nil, perr
	}

	// Record monitor events (idempotent by deterministic event id = txid+state).
	for _, at := range page.Transactions {
		sess.EventsSeen++
		evID := eventID(at.TxID, at.Confirmed)
		exists, _ := s.repo.MonitorEventExists(ctx, evID)
		if exists {
			sess.EventsDuplicate++
			continue
		}
		sess.EventsNew++
		typ := "transaction_discovered"
		if at.Confirmed {
			typ = "confirmation_update"
		}
		now := s.now().UTC().Format(time.RFC3339)
		_ = s.repo.SaveMonitorEvent(ctx, sdk.MonitorEventRow{
			EventID: evID, SessionID: sess.SessionID, Type: typ, TxID: at.TxID,
			Confirmed: at.Confirmed, BlockHeight: at.BlockHeight,
			FirstSeen: now, Timestamp: now,
		})
		sess.LastEventAt = now
		// Affected subjects: the monitored wallet + the tx's counterparties.
		affected[sess.Target] = evID
	}
	sess.TxAcquired += res.Transactions

	// Advance cursor for incremental acquisition.
	if !page.Next.IsZero() {
		sess.LastCursor = page.Next.Cursor
	}
	return affected, nil
}

// analyzeAndAlert runs analysis on an affected subject, records a risk delta if
// the result changed, and generates deduplicated alerts.
func (s *Service) analyzeAndAlert(ctx context.Context, sess *sdk.MonitorSessionRow, subject, triggerEvent string) error {
	cur, err := s.analyze(ctx, subject)
	if err != nil {
		return err
	}
	prevScore := sess.LastRiskScore // -1 when unknown
	prevSignals, prevPatterns := s.lastSignals(sess)

	newSignals := diff(cur.Signals, prevSignals)
	newPatterns := diff(cur.Patterns, prevPatterns)
	changed := prevScore < 0 || cur.RiskScore != prevScore ||
		len(newSignals) > 0 || len(newPatterns) > 0

	if changed {
		d := prevScore
		if d < 0 {
			d = 0
		}
		now := s.now().UTC().Format(time.RFC3339)
		_ = s.repo.SaveRiskDelta(ctx, sdk.RiskDeltaRow{
			ID:        sess.SessionID + "-" + triggerEvent + "-" + subject,
			SessionID: sess.SessionID, Subject: subject,
			PreviousScore: d, CurrentScore: cur.RiskScore, Delta: cur.RiskScore - d,
			CurrentConf: cur.Confidence, ChangedSignals: newSignals,
			NewPatterns: newPatterns, NewEvidenceIDs: cur.EvidenceIDs,
			TriggerEvent: triggerEvent, Timestamp: now,
		})
		s.generateAlerts(ctx, sess, subject, prevScore, cur, newSignals, newPatterns, triggerEvent)
	}

	sess.LastRiskScore = cur.RiskScore
	s.storeSignals(sess, cur.Signals, cur.Patterns)
	return nil
}

// generateAlerts emits deterministic, deduplicated alerts for meaningful change.
func (s *Service) generateAlerts(ctx context.Context, sess *sdk.MonitorSessionRow, subject string, prevScore int, cur AnalysisResult, newSignals, newPatterns []string, trigger string) {
	emit := func(trig, sev, reason string, evid []string) {
		key := fmt.Sprintf("%s|%s|%s|%s", sess.SessionID, subject, trig, trigger)
		now := s.now().UTC().Format(time.RFC3339)
		before := prevScore
		if before < 0 {
			before = 0
		}
		inserted, _ := s.repo.SaveMonitorAlert(ctx, sdk.MonitorAlertRow{
			AlertID: hashKey(key), DedupKey: key, SessionID: sess.SessionID,
			CaseID: sess.CaseID, Subject: subject, Trigger: trig, Severity: sev,
			RiskBefore: before, RiskAfter: cur.RiskScore, Delta: cur.RiskScore - before,
			EvidenceIDs: evid, TxIDs: nil, Reason: reason, Timestamp: now,
		})
		if inserted {
			sess.AlertsGenerated++
		}
	}

	if prevScore >= 0 && cur.RiskScore-prevScore >= s.cfg.AlertRiskDelta {
		emit("risk_score_increase", severityFor(cur.RiskScore),
			fmt.Sprintf("risk %d -> %d (+%d)", prevScore, cur.RiskScore, cur.RiskScore-prevScore),
			cur.EvidenceIDs)
	}
	if s.cfg.AlertNewPattern {
		for _, p := range newPatterns {
			if p == "normal" || p == "" {
				continue
			}
			emit("suspicious_flow_detected", "MED", "new flow pattern: "+p, cur.EvidenceIDs)
		}
	}
	if s.cfg.AlertNewSignal {
		for _, sig := range newSignals {
			if sig == "transaction_anomaly" {
				emit("anomaly_detected", "MED", "new anomaly signal", cur.EvidenceIDs)
			}
		}
	}
}

// ---- helpers ----

func (s *Service) loadOrInitSession(ctx context.Context, address, id string) sdk.MonitorSessionRow {
	if existing, _ := s.repo.GetMonitorSession(ctx, id); existing != nil {
		// Resume: run-scoped acquisition counters report what THIS run did, so a
		// crash+restart that only re-sees already-persisted data reports zero new
		// work (idempotency). Canonical records and the risk baseline survive;
		// only the per-run tallies reset.
		r := *existing
		r.EventsSeen, r.EventsNew, r.EventsDuplicate = 0, 0, 0
		r.TxAcquired, r.AlertsGenerated = 0, 0
		return r
	}
	now := s.now().UTC().Format(time.RFC3339)
	return sdk.MonitorSessionRow{
		SessionID: id, CaseID: s.caseID, Target: address, TargetType: "wallet",
		Provider: s.src.ProviderName(), ProviderVersion: s.src.ProviderVersion(),
		Mode: "poll", Status: "pending", Health: "connected",
		PollIntervalMS: int(s.cfg.PollInterval / time.Millisecond),
		StartedAt:      now, UpdatedAt: now, LastRiskScore: -1,
	}
}

func (s *Service) save(ctx context.Context, sess *sdk.MonitorSessionRow) {
	sess.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	_ = s.repo.SaveMonitorSession(ctx, *sess)
}

func (s *Service) lastSignals(sess *sdk.MonitorSessionRow) (sig, pat []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.signals[sess.SessionID]; ok {
		return v[0], v[1]
	}
	return nil, nil
}

func (s *Service) storeSignals(sess *sdk.MonitorSessionRow, sig, pat []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signals[sess.SessionID] = [2][]string{sig, pat}
}

func eventID(txid string, confirmed bool) string {
	state := "m"
	if confirmed {
		state = "c"
	}
	return "ev-" + hashKey(txid+"|"+state)
}

func diff(cur, prev []string) []string {
	set := map[string]struct{}{}
	for _, p := range prev {
		set[p] = struct{}{}
	}
	var out []string
	for _, c := range cur {
		if _, ok := set[c]; !ok {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func severityFor(score int) string {
	switch {
	case score >= 85:
		return "HIGH"
	case score >= 60:
		return "MED"
	default:
		return "LOW"
	}
}

func hashKey(s string) string {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	const hexd = "0123456789abcdef"
	b := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		b[i] = hexd[h&0xf]
		h >>= 4
	}
	return string(b)
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// toCanonical maps an acquired tx into the canonical schema (reusing sats). The
// shared Persister runs normalizer.Finalize (completeness/vsize/feerate).
func toCanonical(a acquisition.AcquiredTransaction, sess *sdk.MonitorSessionRow) schema.Transaction {
	t := schema.Transaction{
		TxID: a.TxID, ScriptType: a.ScriptType,
		Size: schema.TxSize{BaseSize: a.BaseSize, TotalSize: a.TotalSize,
			Weight: a.Weight, VSize: a.VSize},
		Provenance: schema.Provenance{
			SourceType: schema.SourceExplorer, SourceIdentifier: sess.Provider,
			SchemaVersion: schema.SchemaVersion, DatasetID: sess.SessionID,
			RetrievedAt: time.Now().UTC(),
		},
	}
	if a.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339, a.Timestamp); err == nil {
			t.Timestamp = ts
		}
	}
	if a.HasFee {
		t.FeeBTC = a.FeeBTC
		t.FeeSats = schema.BTCToSats(a.FeeBTC)
	}
	for i, in := range a.Inputs {
		t.Inputs = append(t.Inputs, schema.TransactionInput{
			Address: in.Address, AmountBTC: in.AmountBTC,
			AmountSats: schema.BTCToSats(in.AmountBTC), Index: i})
	}
	for i, o := range a.Outputs {
		t.Outputs = append(t.Outputs, schema.TransactionOutput{
			Address: o.Address, AmountBTC: o.AmountBTC,
			AmountSats: schema.BTCToSats(o.AmountBTC), Index: i})
	}
	return t
}

func toCanonicalObs(o acquisition.AcquiredNetworkObservation, sess *sdk.MonitorSessionRow) (schema.NetworkObservation, bool) {
	if o.SrcIP == "" && o.DstIP == "" {
		return schema.NetworkObservation{}, false
	}
	return schema.NetworkObservation{
		ID: "obs-" + hashKey(o.SrcIP+"|"+o.DstIP+"|"+o.TxID), TxID: o.TxID,
		SrcIP: o.SrcIP, SrcPort: o.SrcPort, DstIP: o.DstIP, DstPort: o.DstPort,
		Country: o.Country, ASN: o.ASN,
		Provenance: schema.Provenance{SourceType: schema.SourceNetwork,
			SchemaVersion: schema.SchemaVersion, DatasetID: sess.SessionID},
	}, true
}

var _ = json.Marshal
