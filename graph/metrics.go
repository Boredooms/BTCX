package graph

import (
	"context"
	"math"

	"github.com/bctx/bctx/pkg/schema"
)

// Metrics are the graph-derived values the feature engine consumes. They are
// computed from real persisted edges, replacing the earlier nominal values.
//
// Canonical definitions (feature-schema-v1):
//   - degree:                 distinct neighbor nodes (undirected)
//   - fan_in:                 distinct source wallets with edges INTO subject
//   - fan_out:                distinct destination wallets with edges FROM subject
//   - graph_depth:            max BFS depth reached within the bounded radius
//   - hop_count:              hops in the longest explored chain from subject
//   - chain_length:           nodes in the longest wallet->...->wallet chain
//   - value_decay:            terminal/initial value along the dominant chain
//   - counterparty_diversity: normalized entropy over counterparty edge weights
type Metrics struct {
	Degree                int
	FanIn                 int
	FanOut                int
	GraphDepth            int
	HopCount              int
	ChainLength           int
	ValueDecay            float64
	CounterpartyDiversity float64
}

// WalletMetrics computes graph metrics for a wallet within a bounded radius.
func (s *Service) WalletMetrics(ctx context.Context, address string, radius int) (Metrics, error) {
	var m Metrics

	from, err := s.repo.EdgesFrom(ctx, address)
	if err != nil {
		return m, err
	}
	to, err := s.repo.EdgesTo(ctx, address)
	if err != nil {
		return m, err
	}

	// Degree: distinct neighbors (undirected).
	neigh := map[string]struct{}{}
	// Fan-out: distinct wallet destinations of SENT_TO edges from subject.
	fanOut := map[string]struct{}{}
	// Fan-in: distinct wallet sources of SENT_TO edges into subject.
	fanIn := map[string]struct{}{}
	// Counterparty weights for diversity (by SENT_TO amount).
	cpWeight := map[string]float64{}

	for _, e := range from {
		other, _ := edgeOther(e, address)
		neigh[other] = struct{}{}
		if e.Type == schema.EdgeSentTo && e.ToType == schema.NodeWallet {
			fanOut[e.To] = struct{}{}
			cpWeight[e.To] += e.AmountBTC
		}
	}
	for _, e := range to {
		other, _ := edgeOther(e, address)
		neigh[other] = struct{}{}
		if e.Type == schema.EdgeSentTo && e.FromType == schema.NodeWallet {
			fanIn[e.From] = struct{}{}
			cpWeight[e.From] += e.AmountBTC
		}
	}

	m.Degree = len(neigh)
	m.FanIn = len(fanIn)
	m.FanOut = len(fanOut)
	m.CounterpartyDiversity = normalizedEntropy(cpWeight)

	// Depth / chain via bounded BFS over wallet->wallet SENT_TO edges.
	depth, hop, chain, decay, err := s.walletChain(ctx, address, radius)
	if err != nil {
		return m, err
	}
	m.GraphDepth = depth
	m.HopCount = hop
	m.ChainLength = chain
	m.ValueDecay = decay
	return m, nil
}

// walletChain explores the forward SENT_TO chain from a wallet (bounded), and
// returns max BFS depth, longest hop count, chain length (nodes), and value
// decay along the dominant (highest-value-first) chain.
func (s *Service) walletChain(ctx context.Context, start string, radius int) (depth, hop, chain int, decay float64, err error) {
	if radius < 1 {
		radius = 1
	}
	// BFS depth over forward wallet edges.
	visited := map[string]int{start: 0}
	frontier := []string{start}
	maxDepth := 0
	for d := 0; d < radius && len(frontier) > 0; d++ {
		var next []string
		for _, node := range frontier {
			fromEdges, e := s.repo.EdgesFrom(ctx, node)
			if e != nil {
				return 0, 0, 0, 1.0, e
			}
			for _, ed := range fromEdges {
				if ed.Type != schema.EdgeSentTo || ed.ToType != schema.NodeWallet {
					continue
				}
				if _, ok := visited[ed.To]; !ok {
					visited[ed.To] = d + 1
					if d+1 > maxDepth {
						maxDepth = d + 1
					}
					next = append(next, ed.To)
					if len(visited) >= s.MaxNodes {
						next = nil
						break
					}
				}
			}
		}
		frontier = next
	}

	// Dominant chain: greedily follow the highest-value SENT_TO edge forward.
	initial, terminal := 0.0, 0.0
	cur := start
	chainNodes := 1
	seen := map[string]struct{}{start: {}}
	for step := 0; step < radius; step++ {
		fromEdges, e := s.repo.EdgesFrom(ctx, cur)
		if e != nil {
			return 0, 0, 0, 1.0, e
		}
		var best *schema.GraphEdge
		for i := range fromEdges {
			ed := fromEdges[i]
			if ed.Type != schema.EdgeSentTo || ed.ToType != schema.NodeWallet {
				continue
			}
			if _, ok := seen[ed.To]; ok {
				continue
			}
			if best == nil || ed.AmountBTC > best.AmountBTC {
				best = &fromEdges[i]
			}
		}
		if best == nil {
			break
		}
		if step == 0 {
			initial = best.AmountBTC
		}
		terminal = best.AmountBTC
		seen[best.To] = struct{}{}
		cur = best.To
		chainNodes++
	}

	decay = 1.0
	if initial > 0 {
		decay = math.Min(terminal/initial, 1.0)
		if decay <= 0 {
			decay = 1.0
		}
	}
	hop = maxDepth
	return maxDepth, hop, chainNodes, decay, nil
}

// normalizedEntropy returns H/ln(k) over positive weights; 0 for <2 entries.
func normalizedEntropy(weights map[string]float64) float64 {
	k := 0
	var total float64
	for _, w := range weights {
		if w > 0 {
			k++
			total += w
		}
	}
	if k < 2 || total == 0 {
		return 0
	}
	var h float64
	for _, w := range weights {
		if w <= 0 {
			continue
		}
		p := w / total
		h -= p * math.Log(p)
	}
	return h / math.Log(float64(k))
}
