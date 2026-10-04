// Package graph is the BCTX relationship engine: it builds a persistent graph
// from the local case database and answers bounded traversal queries. The graph
// is an analytical data structure, not a visualization. It references canonical
// IDs already in storage and never creates duplicate wallet/transaction tables.
package graph

import (
	"context"

	"github.com/bctx/bctx/graph/edges"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Builder constructs graph edges from canonical local records. Edge ids are
// deterministic so builds are idempotent: running twice yields the same edges.
type Builder struct {
	repo sdk.Repository
}

// NewBuilder returns a graph builder over a repository.
func NewBuilder(repo sdk.Repository) *Builder { return &Builder{repo: repo} }

// Stats summarizes a build.
type Stats struct {
	Transactions int
	Observations int
	Edges        int
	ByType       map[schema.EdgeType]int
}

// edgesForTx derives the canonical edges for one transaction.
//
//	wallet --INPUT_TO--> tx         (per input address)
//	tx     --OUTPUT_TO--> wallet    (per output address)
//	inAddr --SENT_TO--> outAddr     (derived wallet-to-wallet, amount-weighted)
func edgesForTx(tx schema.Transaction) []schema.GraphEdge {
	var out []schema.GraphEdge
	ts := tx.Timestamp
	for _, in := range tx.Inputs {
		out = append(out, edges.New(schema.EdgeInputTo, in.Address, schema.NodeWallet,
			tx.TxID, schema.NodeTransaction, in.AmountBTC, ts))
	}
	for _, o := range tx.Outputs {
		out = append(out, edges.New(schema.EdgeOutputTo, tx.TxID, schema.NodeTransaction,
			o.Address, schema.NodeWallet, o.AmountBTC, ts))
	}
	// Derived wallet->wallet SENT_TO edges (one per distinct in/out pair). The
	// amount is the output value; this is a derived analytical edge, documented
	// as such (not a claim that a specific input funded a specific output).
	for _, in := range tx.Inputs {
		for _, o := range tx.Outputs {
			if in.Address == o.Address {
				continue
			}
			out = append(out, edges.New(schema.EdgeSentTo, in.Address, schema.NodeWallet,
				o.Address, schema.NodeWallet, o.AmountBTC, ts))
		}
	}
	return out
}

// edgesForObs derives IP--OBSERVED_WITH-->tx edges from a network observation.
func edgesForObs(o schema.NetworkObservation) []schema.GraphEdge {
	if o.TxID == "" || o.SrcIP == "" {
		return nil
	}
	return []schema.GraphEdge{
		edges.New(schema.EdgeObservedWith, o.SrcIP, schema.NodeIP,
			o.TxID, schema.NodeTransaction, 0, o.Timestamp),
	}
}

// BuildAll (re)derives the full graph from all local records. Idempotent.
func (b *Builder) BuildAll(ctx context.Context) (Stats, error) {
	txs, err := b.repo.AllTransactions(ctx, 0)
	if err != nil {
		return Stats{}, err
	}
	obs, err := b.repo.AllNetworkObservations(ctx, 0)
	if err != nil {
		return Stats{}, err
	}
	return b.buildFrom(ctx, txs, obs)
}

// BuildIncremental adds edges for a specific set of new transactions without
// rebuilding the whole graph. Idempotent for already-present edges.
func (b *Builder) BuildIncremental(ctx context.Context, txs []schema.Transaction, obs []schema.NetworkObservation) (Stats, error) {
	return b.buildFrom(ctx, txs, obs)
}

func (b *Builder) buildFrom(ctx context.Context, txs []schema.Transaction, obs []schema.NetworkObservation) (Stats, error) {
	st := Stats{ByType: map[schema.EdgeType]int{}}
	// Deduplicate within this batch by id before persisting.
	seen := map[string]struct{}{}
	var batch []schema.GraphEdge
	add := func(es []schema.GraphEdge) {
		for _, e := range es {
			if _, ok := seen[e.ID]; ok {
				continue
			}
			seen[e.ID] = struct{}{}
			batch = append(batch, e)
			st.ByType[e.Type]++
		}
	}
	for _, tx := range txs {
		add(edgesForTx(tx))
	}
	for _, o := range obs {
		add(edgesForObs(o))
	}
	if err := b.repo.SaveEdges(ctx, batch); err != nil {
		return Stats{}, err
	}
	st.Transactions = len(txs)
	st.Observations = len(obs)
	st.Edges = len(batch)
	return st, nil
}

// Rebuild clears the derived graph and rebuilds it from canonical data. It never
// deletes transactions, wallets or observations.
func (b *Builder) Rebuild(ctx context.Context) (Stats, error) {
	if err := b.repo.DeleteEdges(ctx); err != nil {
		return Stats{}, err
	}
	return b.BuildAll(ctx)
}

// GraphStats returns current persisted graph statistics.
func (b *Builder) GraphStats(ctx context.Context) (Stats, error) {
	all, err := b.repo.AllEdges(ctx, 0)
	if err != nil {
		return Stats{}, err
	}
	st := Stats{Edges: len(all), ByType: map[schema.EdgeType]int{}}
	for _, e := range all {
		st.ByType[e.Type]++
	}
	return st, nil
}
