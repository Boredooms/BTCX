package reporting

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/html"
	"github.com/bctx/bctx/reporting/json"
	"github.com/bctx/bctx/reporting/markdown"
	"github.com/bctx/bctx/reporting/models"
	"github.com/bctx/bctx/reporting/pdf"
	"github.com/bctx/bctx/sdk"
)

// ErrUnknownFormat is returned by Render for an unsupported report format.
var ErrUnknownFormat = errors.New("reporting: unknown report format")

// ErrRenderNotImplemented is returned by the render/export/verify stubs in this
// feature. FEAT-003 fills these bodies; the signatures and the interface exist
// now so the whole tree compiles and later features build against stable
// contracts. Returning an explicit error (never fake output) matches the
// "no fake implementations / never silently swallow failures" rule.
var ErrRenderNotImplemented = errors.New("reporting: renderer not implemented in this feature")

// BuildInfo carries the identity of the generator for report metadata. It is
// supplied by the caller (CLI/engine wiring) so the reporting package never
// reads global build state itself.
type BuildInfo struct {
	// GeneratedBy identifies the operator/tool that generated the report, e.g.
	// "bctx <version>". It is a non-hashed metadata stamp.
	GeneratedBy string
}

// Generator is the reporting-local service contract for the Phase-7 snapshot /
// render / timeline / export / verify operations. It intentionally lives in the
// reporting package (not sdk) so that reporting/models can depend on the sdk
// persistence Row types without creating an import cycle. The engine wires the
// concrete *Service into sdk.Options.Reports (which only needs the smaller
// sdk.ReportService); callers that need the richer operations type-assert or
// hold a *Service / Generator directly.
type Generator interface {
	BuildSnapshot(ctx context.Context, result schema.InvestigationResult) (models.ReportSnapshot, error)
	Render(ctx context.Context, snap models.ReportSnapshot, format schema.ReportFormat) ([]byte, error)
	Timeline(ctx context.Context, snap models.ReportSnapshot) ([]models.TimelineEvent, error)
	ExportBundle(ctx context.Context, snap models.ReportSnapshot, formats []schema.ReportFormat, outDir string) (models.Manifest, error)
	Verify(ctx context.Context, bundleDir string) (models.VerifyResult, error)
}

// Service is the concrete reporting service. It implements both the minimal
// sdk.ReportService (Build/Export, used for engine wiring) and the richer
// reporting.Generator (snapshot/render/timeline/export/verify).
type Service struct {
	repo  sdk.Repository
	build BuildInfo
	clock func() time.Time
}

// Compile-time assertions: the same concrete value satisfies both contracts.
var (
	_ sdk.ReportService = (*Service)(nil)
	_ Generator         = (*Service)(nil)
)

// NewService constructs a reporting Service over a local repository. The clock
// defaults to time.Now (UTC applied at use); callers may not override it except
// in tests via NewServiceWithClock.
func NewService(repo sdk.Repository, build BuildInfo) *Service {
	return &Service{repo: repo, build: build, clock: time.Now}
}

// NewServiceWithClock is a test seam that injects a deterministic clock. The
// clock only affects the non-hashed generated_at stamp, never the snapshot
// hash.
func NewServiceWithClock(repo sdk.Repository, build BuildInfo, clock func() time.Time) *Service {
	s := NewService(repo, build)
	if clock != nil {
		s.clock = clock
	}
	return s
}

// --- sdk.ReportService (legacy Build/Export) -------------------------------

// Build captures a snapshot for result, fills the extended schema.Report
// metadata and persists it (snapshot_json + snapshot_sha256) via the
// repository. The returned Report embeds the investigation result and the
// stable report identity so callers can render or re-export it later. The
// generated_at stamp comes from the service clock (never folded into the
// snapshot hash).
func (s *Service) Build(ctx context.Context, result schema.InvestigationResult) (schema.Report, error) {
	snap, err := s.BuildSnapshot(ctx, result)
	if err != nil {
		return schema.Report{}, err
	}

	snapJSON, err := CanonicalJSON(snap)
	if err != nil {
		return schema.Report{}, fmt.Errorf("reporting: canonical snapshot: %w", err)
	}
	snapHash := SnapshotHash(snap)
	reportID := ReportID(snap)
	generatedAt := s.clock().UTC()

	modelVersionsJSON, err := CanonicalJSON(snap.ModelVersions)
	if err != nil {
		return schema.Report{}, fmt.Errorf("reporting: canonical model versions: %w", err)
	}

	report := schema.Report{
		ID:              reportID,
		CaseID:          result.CaseID,
		Version:         1,
		Result:          result,
		ModelVersions:   snap.ModelVersions,
		DatasetSnapshot: "",
		GeneratedAt:     generatedAt,
		InvestigationID: result.ID,
		SubjectType:     result.SubjectType,
		SchemaVersion:   snap.SchemaVersion,
		ReportVersion:   snap.GeneratorVersion,
		GeneratedBy:     s.build.GeneratedBy,
		SnapshotSHA256:  snapHash,
	}

	row := sdk.ReportRow{
		ID:                  reportID,
		CaseID:              result.CaseID,
		Version:             1,
		Subject:             result.Subject,
		ResultJSON:          string(mustResultJSON(result)),
		GeneratedAt:         generatedAt.Format(rfc3339),
		ModelVersions:       string(modelVersionsJSON),
		DatasetSnapshot:     "",
		ReportSchemaVersion: snap.SchemaVersion,
		GeneratorVersion:    snap.GeneratorVersion,
		InvestigationID:     result.ID,
		SubjectType:         string(result.SubjectType),
		SnapshotSHA256:      snapHash,
		SnapshotJSON:        string(snapJSON),
		GeneratedBy:         s.build.GeneratedBy,
	}
	if err := s.repo.SaveReport(ctx, row); err != nil {
		return schema.Report{}, fmt.Errorf("reporting: persist report: %w", err)
	}
	return report, nil
}

// Export renders a single report to outPath for the given format. It rebuilds
// the snapshot from the report's embedded result so the rendered output is
// consistent with the persisted report identity, then writes the bytes
// atomically. The parent directory must already exist.
func (s *Service) Export(ctx context.Context, r schema.Report, format schema.ReportFormat, outPath string) error {
	snap, err := s.BuildSnapshot(ctx, r.Result)
	if err != nil {
		return err
	}
	data, err := s.Render(ctx, snap, format)
	if err != nil {
		return err
	}
	return writeFileAtomic(outPath, data)
}

// mustResultJSON canonicalizes an investigation result for the ResultJSON
// column. Canonicalization cannot fail for this fixed struct; on the impossible
// error path a visible marker is stored rather than silently writing an empty
// column.
func mustResultJSON(result schema.InvestigationResult) []byte {
	b, err := CanonicalJSON(result)
	if err != nil {
		return []byte(`{"error":"canonical result encode failed"}`)
	}
	return b
}

// --- reporting.Generator ----------------------------------------------------

// Render renders a snapshot to the given format. It is fully deterministic for
// a given snapshot: the only non-hashed input is the generated_at stamp read
// from the service clock, which is formatted once and threaded into the view's
// metadata (never into the snapshot or its hash). No repository or network
// access occurs.
func (s *Service) Render(ctx context.Context, snap models.ReportSnapshot, format schema.ReportFormat) ([]byte, error) {
	generatedAt := s.clock().UTC().Format(rfc3339)
	view := s.buildView(snap, generatedAt)

	switch format {
	case schema.FormatJSON:
		return json.Render(view, snap.SchemaVersion, snap.GeneratorVersion)
	case schema.FormatMarkdown:
		return markdown.Render(view)
	case schema.FormatHTML:
		return html.Render(view)
	case schema.FormatPDF:
		return pdf.Render(view, SnapshotHash(snap))
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, format)
	}
}

// ExportBundle and Verify are implemented in export.go.

// BuildSnapshot captures a frozen, deterministic snapshot of all local state
// the report for result depends on. It reads ONLY the repository and the passed
// InvestigationResult: no orchestrator, no acquisition, no network. Every slice
// is stored in a total order so the snapshot (and its hash) is reproducible.
func (s *Service) BuildSnapshot(ctx context.Context, result schema.InvestigationResult) (models.ReportSnapshot, error) {
	snap := models.ReportSnapshot{
		SchemaVersion:    ReportSchemaVersion,
		GeneratorVersion: GeneratorVersion,
		Result:           result,
		FeatureSchema:    schema.FeatureSchemaVersion,
		FeatureSchemaSHA: schema.FeatureSchemaSHA256,
		ModelVersions:    map[string]string{},
	}

	// Case meta: only the CaseID is knowable from the repository boundary; the
	// repo exposes no case record. Provider/created_at are enriched by the CLI
	// layer (FEAT-004) which holds the Case. ID always present.
	snap.Case = models.CaseMeta{ID: result.CaseID}

	// Transactions for the subject (wallet history or a single tx).
	txs, err := s.subjectTransactions(ctx, result)
	if err != nil {
		return models.ReportSnapshot{}, err
	}
	sort.Slice(txs, func(i, j int) bool { return txs[i].TxID < txs[j].TxID })
	snap.Transactions = txs

	txIDs := make([]string, 0, len(txs))
	txIDSet := make(map[string]struct{}, len(txs))
	for _, t := range txs {
		txIDs = append(txIDs, t.TxID)
		txIDSet[t.TxID] = struct{}{}
	}
	sort.Strings(txIDs)
	snap.TxIDs = txIDs

	// Network observations correlated with the subject's transactions.
	allObs, err := s.repo.AllNetworkObservations(ctx, 10000)
	if err != nil {
		return models.ReportSnapshot{}, err
	}
	obs := make([]schema.NetworkObservation, 0)
	for _, o := range allObs {
		if _, ok := txIDSet[o.TxID]; ok {
			obs = append(obs, o)
		}
	}
	sort.Slice(obs, func(i, j int) bool {
		if !obs[i].Timestamp.Equal(obs[j].Timestamp) {
			return obs[i].Timestamp.Before(obs[j].Timestamp)
		}
		return obs[i].ID < obs[j].ID
	})
	snap.NetworkObservations = obs

	// Alerts for the subject (case-level list filtered to subject).
	alerts, err := s.subjectAlerts(ctx, result.Subject)
	if err != nil {
		return models.ReportSnapshot{}, err
	}
	snap.Alerts = alerts
	alertIDs := make([]string, 0, len(alerts))
	for _, a := range alerts {
		alertIDs = append(alertIDs, a.ID)
	}
	sort.Strings(alertIDs)
	snap.AlertIDs = alertIDs

	// Evidence IDs from the result (total-ordered).
	evIDs := make([]string, 0, len(result.Evidence))
	for _, e := range result.Evidence {
		evIDs = append(evIDs, e.ID)
	}
	sort.Strings(evIDs)
	snap.EvidenceIDs = evIDs

	// Datasets (redacted provenance), ordered by ID.
	datasets, err := s.repo.ListDatasets(ctx)
	if err != nil {
		return models.ReportSnapshot{}, err
	}
	sort.Slice(datasets, func(i, j int) bool { return datasets[i].ID < datasets[j].ID })
	snap.Datasets = models.RedactDatasets(datasets)

	// Acquisition sync summary for the subject (if any), redacted.
	if sum, ok, serr := s.subjectSync(ctx, result.Subject); serr != nil {
		return models.ReportSnapshot{}, serr
	} else if ok {
		snap.Sync = &sum
	}

	// Monitoring state: latest session for the subject, then its history.
	if err := s.loadMonitoring(ctx, result.Subject, &snap); err != nil {
		return models.ReportSnapshot{}, err
	}

	// Model versions from the result's predictions (sat-exact, no network).
	for _, p := range result.Predictions {
		if p.Model != "" && p.ModelVersion != "" {
			snap.ModelVersions[p.Model] = p.ModelVersion
		}
	}

	// The snapshot content is now frozen. The deterministic report ID is
	// derived from it on demand via ReportID (no timestamp), so an identical
	// snapshot always yields an identical report ID.
	return snap, nil
}

// subjectTransactions loads the transactions relevant to the subject. A wallet
// subject uses WalletTransactions; a tx subject uses GetTransaction.
func (s *Service) subjectTransactions(ctx context.Context, result schema.InvestigationResult) ([]schema.Transaction, error) {
	switch result.SubjectType {
	case schema.NodeTransaction:
		t, err := s.repo.GetTransaction(ctx, result.Subject)
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, nil
		}
		return []schema.Transaction{*t}, nil
	default:
		// Wallet (or unspecified subject) -> wallet history.
		return s.repo.WalletTransactions(ctx, result.Subject, 10000)
	}
}

// subjectAlerts returns the case alerts whose subject matches, ordered by ID.
func (s *Service) subjectAlerts(ctx context.Context, subject string) ([]schema.Alert, error) {
	all, err := s.repo.ListAlerts(ctx, 10000)
	if err != nil {
		return nil, err
	}
	out := make([]schema.Alert, 0)
	for _, a := range all {
		if a.Subject == subject {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// subjectSync returns the most recent acquisition sync checkpoint for the
// subject as a redacted summary. ok is false when none exists.
func (s *Service) subjectSync(ctx context.Context, subject string) (models.SyncSummary, bool, error) {
	cps, err := s.repo.ListSyncCheckpoints(ctx)
	if err != nil {
		return models.SyncSummary{}, false, err
	}
	var best *sdk.SyncCheckpoint
	for i := range cps {
		if cps[i].Target != subject {
			continue
		}
		if best == nil || cps[i].UpdatedAt > best.UpdatedAt ||
			(cps[i].UpdatedAt == best.UpdatedAt && cps[i].SyncID > best.SyncID) {
			c := cps[i]
			best = &c
		}
	}
	if best == nil {
		return models.SyncSummary{}, false, nil
	}
	return models.SyncSummary{
		SyncID:          best.SyncID,
		Provider:        best.Provider,
		ProviderVersion: best.ProviderVersion,
		TargetType:      best.TargetType,
		Target:          best.Target,
		Status:          best.Status,
		Discovered:      best.Discovered,
		Acquired:        best.Acquired,
		Persisted:       best.Persisted,
		Duplicates:      best.Duplicates,
		PartialCount:    best.PartialCount,
		Rejected:        best.Rejected,
		StartedAt:       best.StartedAt,
		UpdatedAt:       best.UpdatedAt,
	}, true, nil
}

// loadMonitoring loads the latest monitor session for subject plus its events,
// risk deltas and alerts, all total-ordered. A missing session leaves the
// snapshot's monitoring fields empty (classified later as "unavailable").
func (s *Service) loadMonitoring(ctx context.Context, subject string, snap *models.ReportSnapshot) error {
	sessions, err := s.repo.ListMonitorSessions(ctx)
	if err != nil {
		return err
	}
	var sess *sdk.MonitorSessionRow
	for i := range sessions {
		if sessions[i].Target != subject {
			continue
		}
		if sess == nil || sessions[i].UpdatedAt > sess.UpdatedAt ||
			(sessions[i].UpdatedAt == sess.UpdatedAt && sessions[i].SessionID > sess.SessionID) {
			c := sessions[i]
			sess = &c
		}
	}
	snap.MonitorEvents = []sdk.MonitorEventRow{}
	snap.RiskDeltas = []sdk.RiskDeltaRow{}
	snap.MonitorAlerts = []sdk.MonitorAlertRow{}
	if sess == nil {
		return nil
	}
	snap.MonitorSession = sess

	events, err := s.repo.ListMonitorEvents(ctx, sess.SessionID, 10000)
	if err != nil {
		return err
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp != events[j].Timestamp {
			return events[i].Timestamp < events[j].Timestamp
		}
		return events[i].EventID < events[j].EventID
	})
	snap.MonitorEvents = events

	deltas, err := s.repo.ListRiskDeltas(ctx, sess.SessionID)
	if err != nil {
		return err
	}
	sort.Slice(deltas, func(i, j int) bool {
		if deltas[i].Timestamp != deltas[j].Timestamp {
			return deltas[i].Timestamp < deltas[j].Timestamp
		}
		return deltas[i].ID < deltas[j].ID
	})
	snap.RiskDeltas = deltas

	malerts, err := s.repo.ListMonitorAlerts(ctx, sess.SessionID)
	if err != nil {
		return err
	}
	sort.Slice(malerts, func(i, j int) bool {
		if malerts[i].Timestamp != malerts[j].Timestamp {
			return malerts[i].Timestamp < malerts[j].Timestamp
		}
		return malerts[i].AlertID < malerts[j].AlertID
	})
	snap.MonitorAlerts = malerts
	return nil
}

// ReportID derives the deterministic report identifier for a snapshot:
// rpt-<caseID>-<subject>-<first 12 hex of snapshot hash>. It contains no
// timestamp, so the same snapshot always yields the same report ID.
func ReportID(snap models.ReportSnapshot) string {
	h := SnapshotHash(snap)
	prefix := h
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	return strings.Join([]string{"rpt", snap.Case.ID, snap.Result.Subject, prefix}, "-")
}

// Timeline merges transaction timestamps, monitor events, risk deltas and alert
// timestamps into typed TimelineEvents sorted by (timestamp, kind, id) with a
// total order. No wall-clock is read during assembly: all timestamps come from
// the snapshot.
func (s *Service) Timeline(ctx context.Context, snap models.ReportSnapshot) ([]models.TimelineEvent, error) {
	events := make([]models.TimelineEvent, 0,
		len(snap.Transactions)+len(snap.MonitorEvents)+len(snap.RiskDeltas)+len(snap.Alerts))

	rfc := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }

	for _, t := range snap.Transactions {
		events = append(events, models.TimelineEvent{
			Kind:      models.KindTx,
			Timestamp: rfc(t.Timestamp),
			ID:        t.TxID,
			Detail:    "transaction observed",
		})
	}
	for _, e := range snap.MonitorEvents {
		events = append(events, models.TimelineEvent{
			Kind:      models.KindMonitorEvent,
			Timestamp: e.Timestamp,
			ID:        e.EventID,
			Detail:    "monitor event " + e.Type,
		})
	}
	for _, d := range snap.RiskDeltas {
		events = append(events, models.TimelineEvent{
			Kind:      models.KindRiskDelta,
			Timestamp: d.Timestamp,
			ID:        d.ID,
			Detail:    "risk delta",
		})
	}
	for _, a := range snap.Alerts {
		events = append(events, models.TimelineEvent{
			Kind:      models.KindAlert,
			Timestamp: rfc(a.CreatedAt),
			ID:        a.ID,
			Detail:    "alert " + a.Type,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].Timestamp != events[j].Timestamp {
			return events[i].Timestamp < events[j].Timestamp
		}
		if events[i].Kind != events[j].Kind {
			return events[i].Kind < events[j].Kind
		}
		return events[i].ID < events[j].ID
	})
	return events, nil
}
