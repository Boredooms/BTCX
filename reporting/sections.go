package reporting

import (
	"sort"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/reporting/models"
)

// buildView turns the frozen snapshot into the render-ready view. It performs
// NO repository or network access: every value comes from the snapshot. The
// clock-derived generatedAt / build.GeneratedBy are the ONLY inputs not folded
// into the snapshot hash (see SnapshotHash).
func (s *Service) buildView(snap models.ReportSnapshot, generatedAt string) models.ReportView {
	snapHash := SnapshotHash(snap)
	reportID := ReportID(snap)

	v := models.ReportView{}

	v.Meta = models.ReportMeta{
		ReportID:        reportID,
		CaseID:          snap.Case.ID,
		InvestigationID: snap.Result.ID,
		Subject:         snap.Result.Subject,
		SubjectType:     string(snap.Result.SubjectType),
		GeneratedAt:     generatedAt,
		GeneratedBy:     s.build.GeneratedBy,
		SchemaVersion:   snap.SchemaVersion,
		ReportVersion:   snap.GeneratorVersion,
	}

	// 1. Case information.
	v.CaseInformation = models.CaseInformation{
		CaseID:    orNA(snap.Case.ID),
		CreatedAt: orNA(snap.Case.CreatedAt),
		Provider:  providerText(snap.Case.ProviderName, snap.Case.ProviderVersion),
	}

	// 2. Investigation subject.
	v.InvestigationSubject = models.InvestigationSubject{
		Subject:     orNA(snap.Result.Subject),
		SubjectType: orNA(string(snap.Result.SubjectType)),
		Offline:     snap.Result.Offline,
	}

	// 3. Acquisition summary.
	if snap.Sync != nil {
		v.AcquisitionSummary = models.AcquisitionSummary{
			Provider:        snap.Sync.Provider,
			ProviderVersion: snap.Sync.ProviderVersion,
			Status:          snap.Sync.Status,
			Discovered:      snap.Sync.Discovered,
			Acquired:        snap.Sync.Acquired,
			Persisted:       snap.Sync.Persisted,
			Available:       true,
		}
	} else {
		v.AcquisitionSummary = models.AcquisitionSummary{Available: false}
	}

	// 4. Dataset provenance (already redacted + ordered in the snapshot).
	v.DatasetProvenance = models.DatasetProvenance{Datasets: snap.Datasets}

	// 5. Transaction summary (sat-exact; BTC derived from sats).
	var inSats, outSats int64
	for _, t := range snap.Transactions {
		inSats += t.TotalInSats()
		outSats += t.TotalOutSats()
	}
	v.TransactionSummary = models.TransactionSummary{
		Count:       len(snap.Transactions),
		TotalInSats: inSats,
		TotalOutSat: outSats,
		TotalInBTC:  schema.SatsToBTC(inSats),
		TotalOutBTC: schema.SatsToBTC(outSats),
		TxIDs:       snap.TxIDs,
	}

	// 6. Flow analysis (fan-in/out aggregates).
	var fanIn, fanOut int
	for _, t := range snap.Transactions {
		fanIn += t.FanIn()
		fanOut += t.FanOut()
	}
	v.FlowAnalysis = models.FlowAnalysis{TotalFanIn: fanIn, TotalFanOut: fanOut}

	// 7. Graph summary (from the result's optional subgraph).
	if g := snap.Result.Subgraph; g != nil {
		v.GraphSummary = models.GraphSummary{
			Available: true,
			Center:    g.Center,
			Depth:     g.Depth,
			NodeCount: len(g.Nodes),
			EdgeCount: len(g.Edges),
		}
	} else {
		v.GraphSummary = models.GraphSummary{Available: false, Center: snap.Result.Subject}
	}

	// 8. Related wallets (inferred relationship, never proven ownership).
	members := clusterMembers(snap.Result.Clusters)
	v.RelatedWallets = models.RelatedWallets{
		Count:        snap.Result.RelatedWallets,
		Relationship: models.ClusterRelationshipPhrase,
		Members:      members,
	}

	// 9. Entity signals.
	v.EntitySignals = models.EntitySignals{ClusterCount: len(snap.Result.Clusters)}

	// 10. Network observations.
	v.NetworkObservations = models.NetworkObservations{Count: len(snap.NetworkObservations)}

	// 11. ML signals.
	preds := make([]models.MLPrediction, 0, len(snap.Result.Predictions))
	for _, p := range snap.Result.Predictions {
		preds = append(preds, models.MLPrediction{
			Model:         p.Model,
			ModelVersion:  p.ModelVersion,
			FeatureSchema: p.FeatureSchema,
			Score:         p.Score,
			Confidence:    p.Confidence,
		})
	}
	sort.Slice(preds, func(i, j int) bool {
		if preds[i].Model != preds[j].Model {
			return preds[i].Model < preds[j].Model
		}
		return preds[i].ModelVersion < preds[j].ModelVersion
	})
	v.MLSignals = models.MLSignals{Predictions: preds}

	// 12. Risk assessment.
	rsig := make([]models.RiskSignal, 0, len(snap.Result.Risk.Signals))
	for _, sg := range snap.Result.Risk.Signals {
		rsig = append(rsig, models.RiskSignal{
			Name:        sg.Name,
			Score:       sg.Score,
			Weight:      sg.Weight,
			Description: sg.Description,
		})
	}
	sort.Slice(rsig, func(i, j int) bool { return rsig[i].Name < rsig[j].Name })
	v.RiskAssessment = models.RiskAssessment{
		Score:      snap.Result.Risk.Score,
		Confidence: snap.Result.Risk.Confidence,
		Signals:    rsig,
	}

	// 13. Risk deltas (from monitoring history).
	deltas := make([]models.RiskDeltaEntry, 0, len(snap.RiskDeltas))
	for _, d := range snap.RiskDeltas {
		deltas = append(deltas, models.RiskDeltaEntry{
			Subject:       d.Subject,
			PreviousScore: d.PreviousScore,
			CurrentScore:  d.CurrentScore,
			Delta:         d.Delta,
			Timestamp:     d.Timestamp,
		})
	}
	v.RiskDeltas = models.RiskDeltas{Deltas: deltas}

	// 14. Alert history (approved status wording).
	alerts := make([]models.AlertEntry, 0, len(snap.Alerts))
	for _, a := range snap.Alerts {
		alerts = append(alerts, models.AlertEntry{
			ID:         a.ID,
			Type:       a.Type,
			Risk:       a.Risk,
			Confidence: a.Confidence,
			Status:     models.AlertStatusText(a.Status),
			Reason:     a.Reason,
			CreatedAt:  a.CreatedAt.UTC().Format(rfc3339),
		})
	}
	v.AlertHistory = models.AlertHistory{Alerts: alerts}

	// 15. Evidence (source-linked, ordered by ID).
	ev := make([]models.EvidenceEntry, 0, len(snap.Result.Evidence))
	for _, e := range snap.Result.Evidence {
		recs := append([]string(nil), e.SourceRecords...)
		sort.Strings(recs)
		ev = append(ev, models.EvidenceEntry{
			ID:            e.ID,
			Type:          e.Type,
			Severity:      string(e.Severity),
			Description:   e.Description,
			SourceRecords: recs,
			Confidence:    e.Confidence,
		})
	}
	sort.Slice(ev, func(i, j int) bool { return ev[i].ID < ev[j].ID })
	v.Evidence = models.Evidence{Items: ev}

	// 16. Timeline (deterministic merge; no wall-clock reads).
	tl, _ := s.Timeline(nil, snap)
	v.Timeline = models.Timeline{Events: tl}

	// 17. Graph paths (risk propagation).
	paths := make([]models.GraphPathEntry, 0, len(snap.Result.Propagation))
	for _, p := range snap.Result.Propagation {
		path := append([]string(nil), p.Path...)
		paths = append(paths, models.GraphPathEntry{
			Seed:         p.Seed,
			Target:       p.Target,
			Distance:     p.Distance,
			Contribution: p.Contribution,
			Path:         path,
		})
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Seed != paths[j].Seed {
			return paths[i].Seed < paths[j].Seed
		}
		if paths[i].Target != paths[j].Target {
			return paths[i].Target < paths[j].Target
		}
		return paths[i].Distance < paths[j].Distance
	})
	v.GraphPaths = models.GraphPaths{Paths: paths}

	// 18. Limitations (fixed caveats + Esplora size caveat).
	v.Limitations = models.Limitations{Notes: limitationNotes(snap)}

	// 19. Data quality.
	v.DataQuality = models.DataQuality{
		TransactionCount:        len(snap.Transactions),
		NetworkObservationCount: len(snap.NetworkObservations),
		HasGraph:                snap.Result.Subgraph != nil,
	}

	// 20. Offline / connected state (classified monitor state).
	state, detail := models.ClassifyMonitorState(snap.MonitorSession)
	v.OfflineConnected = models.OfflineConnectedState{
		Offline:      snap.Result.Offline,
		MonitorState: string(state),
		Detail:       detail,
	}

	// 21. Generation metadata.
	v.GenerationMetadata = models.GenerationMetadata{
		GeneratedAt:      generatedAt,
		GeneratedBy:      s.build.GeneratedBy,
		SchemaVersion:    snap.SchemaVersion,
		GeneratorVersion: snap.GeneratorVersion,
		SnapshotSHA256:   snapHash,
		ModelVersions:    snap.ModelVersions,
		FeatureSchema:    snap.FeatureSchema,
	}

	return v
}

// clusterMembers returns the deduplicated, sorted union of all cluster members.
func clusterMembers(clusters []schema.EntityCluster) []string {
	set := map[string]struct{}{}
	for _, c := range clusters {
		for _, m := range c.Members {
			set[m] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for m := range set {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// limitationNotes assembles the fixed limitation caveats a forensic report must
// always state, plus the Esplora transaction-size caveat.
func limitationNotes(snap models.ReportSnapshot) []string {
	notes := []string{
		"This report describes observed, detected and inferred signals from local evidence only; it does not establish real-world identity or intent.",
		"Entity clusters are an " + models.ClusterRelationshipPhrase + ".",
		"Pattern detectors are named with a \"-like\" suffix and indicate a pattern match, not a proven classification.",
		esploraSizeCaveat,
	}
	if snap.Result.Offline {
		notes = append(notes, "Generated offline: no network acquisition occurred during report generation.")
	}
	return notes
}

// providerText renders a provider name/version pair, or NotAvailable when both
// are empty.
func providerText(name, version string) string {
	switch {
	case name == "" && version == "":
		return models.NotAvailable
	case version == "":
		return name
	default:
		return name + " " + version
	}
}

// orNA returns NotAvailable for an empty string.
func orNA(s string) string {
	if s == "" {
		return models.NotAvailable
	}
	return s
}

const (
	// rfc3339 is the canonical timestamp layout used across rendered reports.
	rfc3339 = "2006-01-02T15:04:05Z07:00"

	// esploraSizeCaveat is the fixed Esplora transaction-size caveat required in
	// both the Transaction Summary and Limitations sections.
	esploraSizeCaveat = "Transaction size/weight fields may be absent when the Esplora source did not supply them; absent sizes are reported as not available and never estimated."
)
