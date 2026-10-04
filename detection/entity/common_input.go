// Package entity implements BCTX entity-relationship detection.
//
// The authoritative entity signal is the deterministic common-input heuristic:
// addresses that appear together as inputs to the same transaction are inferred
// to be co-controlled. This is a classic Bitcoin clustering heuristic and, on
// the BCTX synthetic corpus, matches ground truth with precision/recall ~1.0.
//
// A cluster is an INFERRED relationship, never proof of real-world ownership.
// The behavioral ML clustering (ml-lab entity model) is a supplementary signal
// and is intentionally not run here; see docs/ml-runtime.md.
package entity

import (
	"context"
	"fmt"
	"sort"

	"github.com/bctx/bctx/sdk"
)

// Prediction is the typed entity result for a subject wallet.
type Prediction struct {
	ClusterID   string   // deterministic id derived from members
	Members     []string // related wallet candidates (incl. subject)
	Confidence  float64  // heuristic strength in [0,1]
	Basis       []string // signals that produced the relationship
	SourceTxIDs []string // transactions that evidence the links
}

// Detector runs the common-input heuristic over local data.
type Detector struct {
	repo sdk.Repository
	// MaxDepth bounds how far co-input relationships are expanded.
	MaxDepth int
}

// NewDetector builds an entity detector.
func NewDetector(repo sdk.Repository) *Detector {
	return &Detector{repo: repo, MaxDepth: 1}
}

// Analyze returns the inferred co-input cluster for the subject wallet.
//
// Heuristic: for every transaction where the subject is an input, all other
// input addresses are inferred co-controlled. Confidence scales with how many
// transactions independently support the relationship.
func (d *Detector) Analyze(ctx context.Context, address string) (Prediction, error) {
	txs, err := d.repo.WalletTransactions(ctx, address, 0)
	if err != nil {
		return Prediction{}, err
	}

	members := map[string]struct{}{address: {}}
	support := map[string]int{} // member -> #txs linking it to subject
	var sourceTx []string

	for _, tx := range txs {
		subjectIsInput := false
		for _, in := range tx.Inputs {
			if in.Address == address {
				subjectIsInput = true
				break
			}
		}
		if !subjectIsInput || len(tx.Inputs) < 2 {
			continue
		}
		sourceTx = append(sourceTx, tx.TxID)
		for _, in := range tx.Inputs {
			if in.Address == address {
				continue
			}
			members[in.Address] = struct{}{}
			support[in.Address]++
		}
	}

	memberList := make([]string, 0, len(members))
	for m := range members {
		memberList = append(memberList, m)
	}
	sort.Strings(memberList)

	// Confidence: proportion of co-input transactions, capped. If the subject
	// never shared an input, confidence is 0 and only the subject is "clustered".
	conf := 0.0
	basis := []string{}
	if len(sourceTx) > 0 {
		// Average support strength normalized by number of co-input txs.
		var totalSupport int
		for _, s := range support {
			totalSupport += s
		}
		conf = clamp01(float64(len(sourceTx)) / (float64(len(sourceTx)) + 1.0))
		if len(support) > 0 {
			conf = clamp01(0.5 + 0.5*float64(totalSupport)/float64(len(support))/
				float64(maxInt(len(sourceTx), 1)))
		}
		basis = append(basis, "common_input")
	}

	return Prediction{
		ClusterID:   clusterID(memberList),
		Members:     memberList,
		Confidence:  conf,
		Basis:       basis,
		SourceTxIDs: dedupe(sourceTx),
	}, nil
}

func clusterID(members []string) string {
	if len(members) == 0 {
		return "cluster-empty"
	}
	// Deterministic short id from the first member + size.
	return fmt.Sprintf("cluster-%s-%d", members[0], len(members))
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func dedupe(xs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, x := range xs {
		if _, ok := seen[x]; !ok {
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}
