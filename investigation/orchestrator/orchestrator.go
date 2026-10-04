// Package orchestrator runs the BCTX investigation pipeline, producing a single
// schema.InvestigationResult consumed identically by CLI, TUI and reports.
//
// Pipeline (fixed order, offline):
//
//	local data -> feature engine -> anomaly ONNX -> entity (common-input) ->
//	flow ONNX + structural detectors -> risk -> evidence -> result
//
// No network access occurs. Models are local tree-ensemble artifacts executed
// by ml/inference.
package orchestrator

import (
	"context"
	"fmt"
	"time"

	entitydet "github.com/bctx/bctx/detection/entity"
	flowdet "github.com/bctx/bctx/detection/flow"
	"github.com/bctx/bctx/evidence/collector"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/ml/features"
	"github.com/bctx/bctx/ml/inference"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/risk/scoring"
	"github.com/bctx/bctx/sdk"
)

// Orchestrator wires the concrete analysis components.
type Orchestrator struct {
	repo     sdk.Repository
	features *features.Engine
	graph    *graph.Service
	models   *inference.Registry
	entity   *entitydet.Detector
	risk     *scoring.Engine
	caseID   string
}

// Options configures the orchestrator.
type Options struct {
	Repo      sdk.Repository
	ModelsDir string
	// FeatureSchemaSHA is the runtime's feature-schema-v1 hash for fail-closed
	// model compatibility checks.
	FeatureSchemaSHA string
	CaseID           string
	Weights          scoring.Weights
}

// New builds an orchestrator.
func New(opts Options) *Orchestrator {
	w := opts.Weights
	if (w == scoring.Weights{}) {
		w = scoring.DefaultWeights()
	}
	gsvc := graph.NewService(opts.Repo)
	// Feature engine sources graph features from the real persisted graph.
	feat := features.NewEngine(opts.Repo, gsvc).
		WithGraphMetrics(graph.NewMetricsAdapter(gsvc))
	return &Orchestrator{
		repo:     opts.Repo,
		features: feat,
		graph:    gsvc,
		models:   inference.NewRegistry(opts.ModelsDir, opts.FeatureSchemaSHA),
		entity:   entitydet.NewDetector(opts.Repo),
		risk:     scoring.New(w),
		caseID:   opts.CaseID,
	}
}

// Close releases model resources.
func (o *Orchestrator) Close() error { return o.models.Close() }

// AnalyzeWallet executes the full pipeline for a wallet.
func (o *Orchestrator) AnalyzeWallet(ctx context.Context, address string, offline bool) (*schema.InvestigationResult, error) {
	result := &schema.InvestigationResult{
		ID:          "inv-" + address,
		CaseID:      o.caseID,
		Subject:     address,
		SubjectType: schema.NodeWallet,
		Offline:     offline,
		CreatedAt:   time.Now().UTC(),
	}
	eb := collector.NewBuilder(address)

	// 1. Local data check.
	txs, err := o.repo.WalletTransactions(ctx, address, 0)
	if err != nil {
		return nil, fmt.Errorf("local lookup: %w", err)
	}
	result.RelevantTxs = len(txs)
	if len(txs) == 0 {
		// Honest: nothing to analyze locally. Return an empty, low-risk result.
		result.Risk = schema.RiskAssessment{
			Subject: address, SubjectType: schema.NodeWallet,
			Score: 0, Confidence: 0, CreatedAt: time.Now().UTC(),
		}
		return result, nil
	}

	// 2. Features (feature-schema-v1).
	fv, err := o.features.WalletFeatures(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("features: %w", err)
	}
	featVec := features.Vector(fv)

	// 3. Anomaly inference (local ONNX tree model).
	var (
		anomalyScore float64
		anomalyVer   string
		riskIn       = scoring.Inputs{Subject: address, SubjectType: schema.NodeWallet}
	)
	if am, merr := o.models.Get("anomaly"); merr == nil {
		ar, perr := am.PredictAnomaly(featVec)
		if perr != nil {
			return nil, fmt.Errorf("anomaly inference: %w", perr)
		}
		anomalyScore = ar.AnomalyScore
		anomalyVer = ar.ModelVersion
		result.Predictions = append(result.Predictions, schema.Prediction{
			Model: "anomaly", ModelVersion: ar.ModelVersion,
			FeatureSchema: features.FeatureSchemaVersion, Subject: address,
			Score: ar.AnomalyScore, Confidence: ar.AnomalyScore,
			Timestamp: time.Now().UTC(),
		})
		ev := eb.Anomaly(collector.AnomalyInput{Score: ar.AnomalyScore, ModelVersion: ar.ModelVersion})
		result.Evidence = append(result.Evidence, ev)
		riskIn.AnomalyScore = ar.AnomalyScore
		riskIn.AnomalyConf = ar.AnomalyScore
		riskIn.AnomalyEvidIDs = []string{ev.ID}
	} else {
		return nil, fmt.Errorf("load anomaly model: %w", merr)
	}
	_ = anomalyVer

	// 4. Entity (deterministic common-input heuristic).
	ent, eerr := o.entity.Analyze(ctx, address)
	if eerr != nil {
		return nil, fmt.Errorf("entity: %w", eerr)
	}
	result.RelatedWallets = len(ent.Members)
	if len(ent.Members) > 1 {
		result.Clusters = append(result.Clusters, schema.EntityCluster{
			ID: ent.ClusterID, Members: ent.Members,
			Confidence: ent.Confidence, Basis: ent.Basis,
			CreatedAt: time.Now().UTC(),
		})
	}
	if evp := eb.Entity(collector.EntityInput{
		ClusterID: ent.ClusterID, Members: ent.Members,
		Confidence: ent.Confidence, SourceTxIDs: ent.SourceTxIDs,
	}); evp != nil {
		result.Evidence = append(result.Evidence, *evp)
		riskIn.EntityEvidIDs = []string{evp.ID}
	}
	riskIn.EntityStrength = ent.Confidence
	riskIn.EntityMembers = len(ent.Members)

	// 5. Flow: structural features from the wallet's transactions + ML score.
	ff := flowFeaturesFromTxs(txs, fv)
	structural := flowdet.Detect(ff)
	var structEv []collector.StructuralEvidence
	var maxStructural float64
	for _, s := range structural {
		structEv = append(structEv, collector.StructuralEvidence{
			Pattern: s.Pattern, Score: s.Score, Reason: s.Reason})
		if s.Score > maxStructural {
			maxStructural = s.Score
		}
	}
	// The strongest deterministic structural score corroborates (or fails to
	// corroborate) the ML anomaly in risk aggregation (see scoring.Inputs).
	riskIn.Structural = maxStructural
	if fm, merr := o.models.Get("flow"); merr == nil {
		fr, perr := fm.PredictFlow(ff.Vector())
		if perr != nil {
			return nil, fmt.Errorf("flow inference: %w", perr)
		}
		suspicion := 1.0 - fr.Probabilities["normal"]
		result.Patterns = append(result.Patterns, schema.PatternResult{
			Type: schema.PatternType(fr.TopClass), Score: fr.TopScore,
			Confidence: fr.TopScore, Subject: address,
			SourceTxIDs: txIDs(txs),
			Description: fmt.Sprintf("%s-like (model)", fr.TopClass),
		})
		flowItems := eb.Flow(collector.FlowInput{
			TopClass: fr.TopClass, Suspicion: suspicion,
			ModelVersion: fr.ModelVersion, SourceTxIDs: txIDs(txs),
			Structural: structEv,
		})
		result.Evidence = append(result.Evidence, flowItems...)
		riskIn.FlowSuspicion = suspicion
		riskIn.FlowTopClass = fr.TopClass
		riskIn.FlowConf = fr.TopScore
		for _, it := range flowItems {
			riskIn.FlowEvidIDs = append(riskIn.FlowEvidIDs, it.ID)
		}
	} else {
		return nil, fmt.Errorf("load flow model: %w", merr)
	}

	// 6. Risk aggregation.
	result.Risk = o.risk.Score(riskIn)
	_ = anomalyScore

	// 7. Attach a bounded subgraph for presentation/evidence drill-down.
	if sg, serr := o.graph.Subgraph(ctx, address, 2); serr == nil {
		result.Subgraph = sg
	}

	return result, nil
}

func txIDs(txs []schema.Transaction) []string {
	out := make([]string, 0, len(txs))
	for _, t := range txs {
		out = append(out, t.TxID)
	}
	return out
}

// flowFeaturesFromTxs derives structural flow features for a wallet from its
// local transactions, reusing the wallet feature vector where aligned. Real
// per-transaction vsize/feerate are averaged from imported canonical records
// (Phase 4); nominal fallbacks apply only when a dataset lacks size metadata.
func flowFeaturesFromTxs(txs []schema.Transaction, fv schema.FeatureVector) flowdet.Features {
	var inSum, outSum, volSum float64
	var fanIn, fanOut int
	var vsizeSum, feerateSum float64
	var vsizeN, feerateN int
	for _, t := range txs {
		inSum += float64(t.FanIn())
		outSum += float64(t.FanOut())
		volSum += t.TotalOutBTC()
		fanIn += t.FanIn()
		fanOut += t.FanOut()
		if t.Size.VSize > 0 {
			vsizeSum += float64(t.Size.VSize)
			vsizeN++
		}
		if t.FeeRateSatVB > 0 {
			feerateSum += t.FeeRateSatVB
			feerateN++
		}
	}
	n := float64(len(txs))
	get := func(k string) float64 { return fv.Values[k] }

	// Real averages when available; documented nominal fallback otherwise.
	vsize := 180.0
	if vsizeN > 0 {
		vsize = vsizeSum / float64(vsizeN)
	}
	feerate := 12.0
	if feerateN > 0 {
		feerate = feerateSum / float64(feerateN)
	}
	return flowdet.Features{
		InputCount:       avg(inSum, n),
		OutputCount:      avg(outSum, n),
		ParticipantCount: get("degree"),
		HopCount:         get("hop_count"),
		ChainLength:      get("chain_length"),
		ValueDecay:       get("value_decay"),
		SplitRatio:       get("split_ratio"),
		MergeRatio:       get("merge_ratio"),
		FanIn:            float64(fanIn),
		FanOut:           float64(fanOut),
		MedianTimeGap:    get("median_time_gap"),
		Burstiness:       get("burstiness"),
		Velocity:         get("velocity"),
		TotalVolumeBTC:   volSum,
		AmountEntropy:    get("counterparty_diversity"),
		VsizeVB:          vsize,
		FeerateSatVB:     feerate,
	}
}

func avg(s, n float64) float64 {
	if n == 0 {
		return 0
	}
	return s / n
}
