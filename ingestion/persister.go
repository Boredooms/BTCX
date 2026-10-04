package ingestion

import (
	"context"

	"github.com/bctx/bctx/blockchain/normalizer"
	"github.com/bctx/bctx/graph"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Persister is the single canonical persistence path shared by file import
// (Phase 4) and provider acquisition (Phase 5). It finalizes, classifies,
// deduplicates, persists in an atomic batch, and updates the graph
// incrementally. There is exactly one place that performs these steps so that
// satoshi/size/feerate/completeness/dedup semantics never diverge between
// ingestion sources.
type Persister struct {
	repo    sdk.Repository
	builder *graph.Builder
}

// NewPersister builds a persister over a repository.
func NewPersister(repo sdk.Repository) *Persister {
	return &Persister{repo: repo, builder: graph.NewBuilder(repo)}
}

// PersistResult reports what a batch persist did.
type PersistResult struct {
	Transactions int // newly persisted transactions
	Partial      int // of the newly persisted, how many were partial
	NetworkObs   int // newly persisted observations
	Duplicates   int // records already present (skipped)
	Rejected     int // invalid records dropped
	Wallets      map[string]struct{}
}

// PersistBatch finalizes + validates + dedups + persists a batch of canonical
// transactions and observations, then updates the graph incrementally. It is
// idempotent: records already present (by txid / observation id) are counted as
// duplicates and skipped, so wallet counters and graph edges are not double
// applied.
//
// enrich controls partial-record enrichment: when true, a locally-partial
// transaction may be replaced by a more complete acquired version (merge
// semantics in mergeTransaction); when false (default file-import behavior) an
// existing txid is treated purely as a duplicate.
func (p *Persister) PersistBatch(ctx context.Context, txs []schema.Transaction, obs []schema.NetworkObservation, enrich bool) (PersistResult, error) {
	res := PersistResult{Wallets: map[string]struct{}{}}

	toSave := make([]schema.Transaction, 0, len(txs))
	for i := range txs {
		t := txs[i]
		normalizer.Finalize(&t)
		if t.Completeness == schema.CompleteInvalid {
			res.Rejected++
			continue
		}
		existing, err := p.repo.GetTransaction(ctx, t.TxID)
		if err != nil {
			return res, err
		}
		if existing != nil {
			if enrich && existing.Completeness == schema.CompletePartial &&
				t.Completeness != schema.CompletePartial {
				merged := mergeTransaction(*existing, t)
				normalizer.Finalize(&merged)
				toSave = append(toSave, merged)
				collectWallets(&res, merged)
				if merged.Completeness == schema.CompletePartial {
					res.Partial++
				}
				continue
			}
			res.Duplicates++
			continue
		}
		if t.Completeness == schema.CompletePartial {
			res.Partial++
		}
		toSave = append(toSave, t)
		collectWallets(&res, t)
	}

	newObs := make([]schema.NetworkObservation, 0, len(obs))
	for _, o := range obs {
		exists, err := p.repo.ObsExists(ctx, o.ID)
		if err != nil {
			return res, err
		}
		if exists {
			res.Duplicates++
			continue
		}
		newObs = append(newObs, o)
	}

	if len(toSave) > 0 {
		if err := p.repo.SaveTransactions(ctx, toSave); err != nil {
			return res, err
		}
		res.Transactions = len(toSave)
	}
	if len(newObs) > 0 {
		if err := p.repo.SaveNetworkObservations(ctx, newObs); err != nil {
			return res, err
		}
		res.NetworkObs = len(newObs)
	}
	if len(toSave) > 0 || len(newObs) > 0 {
		if _, err := p.builder.BuildIncremental(ctx, toSave, newObs); err != nil {
			return res, err
		}
	}
	return res, nil
}

func collectWallets(res *PersistResult, t schema.Transaction) {
	for _, in := range t.Inputs {
		res.Wallets[in.Address] = struct{}{}
	}
	for _, o := range t.Outputs {
		res.Wallets[o.Address] = struct{}{}
	}
}

// mergeTransaction enriches a partial local transaction with a more complete
// acquired one. Merge semantics are deterministic and documented:
//   - inputs/outputs: take the acquired set if it has more entries (the
//     acquired record is the more complete source); otherwise keep local.
//   - scalar fields (fee, size, script): prefer a non-zero acquired value,
//     else keep the known-good local value (never lose local data to a gap).
//   - timestamp: keep the earliest known source timestamp.
//   - provenance: record that the record was enriched.
func mergeTransaction(local, acquired schema.Transaction) schema.Transaction {
	out := local
	if len(acquired.Inputs) > len(local.Inputs) {
		out.Inputs = acquired.Inputs
	}
	if len(acquired.Outputs) > len(local.Outputs) {
		out.Outputs = acquired.Outputs
	}
	if acquired.FeeSats != 0 {
		out.FeeSats = acquired.FeeSats
		out.FeeBTC = acquired.FeeBTC
	}
	if acquired.Size.VSize != 0 {
		out.Size = acquired.Size
	}
	if acquired.FeeRateSatVB != 0 {
		out.FeeRateSatVB = acquired.FeeRateSatVB
	}
	if acquired.ScriptType != "" {
		out.ScriptType = acquired.ScriptType
		out.OriginalScriptType = acquired.OriginalScriptType
	}
	if !acquired.Timestamp.IsZero() &&
		(out.Timestamp.IsZero() || acquired.Timestamp.Before(out.Timestamp)) {
		out.Timestamp = acquired.Timestamp
	}
	// Keep the acquired provenance (records who enriched it) but mark enriched.
	if acquired.Provenance.SourceType != "" {
		out.Provenance = acquired.Provenance
	}
	return out
}
