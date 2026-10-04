// Package propagation implements graph-based risk propagation from seed
// entities. The baseline is deterministic distance-decay: a seed's risk
// contributes to nearby nodes, attenuated by graph distance. Every propagated
// contribution records its seed, path, distance and value so the analyst can
// explain why a score changed.
//
// This is the foundation; label propagation / personalized PageRank are future
// options evaluated against this baseline.
package propagation

import (
	"context"
	"math"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Seed is an entity with known elevated risk in [0,1].
type Seed struct {
	NodeID string
	Risk   float64
}

// Config controls decay behavior.
type Config struct {
	// Decay in (0,1): multiplier applied per hop. Default 0.5.
	Decay float64
	// MaxDepth bounds propagation distance. Default 3.
	MaxDepth int
	// MinContribution: contributions below this are dropped. Default 0.01.
	MinContribution float64
}

// DefaultConfig returns conservative, documented defaults.
func DefaultConfig() Config {
	return Config{Decay: 0.5, MaxDepth: 3, MinContribution: 0.01}
}

// Engine propagates risk over the graph via the repository adjacency.
type Engine struct {
	repo sdk.Repository
	cfg  Config
}

// New builds a propagation engine.
func New(repo sdk.Repository, cfg Config) *Engine {
	if cfg.Decay <= 0 || cfg.Decay >= 1 {
		cfg.Decay = 0.5
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 3
	}
	if cfg.MinContribution <= 0 {
		cfg.MinContribution = 0.01
	}
	return &Engine{repo: repo, cfg: cfg}
}

func (e *Engine) adjacency(ctx context.Context, node string) ([]schema.GraphEdge, error) {
	from, err := e.repo.EdgesFrom(ctx, node)
	if err != nil {
		return nil, err
	}
	to, err := e.repo.EdgesTo(ctx, node)
	if err != nil {
		return nil, err
	}
	return append(from, to...), nil
}

// Propagate computes propagation steps from a seed via bounded BFS with
// distance decay. Each reachable node (other than the seed) gets a step with
// contribution = seed.Risk * decay^distance, recording the discovery path.
func (e *Engine) Propagate(ctx context.Context, seed Seed) ([]schema.RiskPropagationStep, error) {
	type qitem struct {
		node string
		dist int
		path []string
	}
	visited := map[string]bool{seed.NodeID: true}
	queue := []qitem{{seed.NodeID, 0, []string{seed.NodeID}}}
	var steps []schema.RiskPropagationStep

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.dist >= e.cfg.MaxDepth {
			continue
		}
		adj, err := e.adjacency(ctx, cur.node)
		if err != nil {
			return nil, err
		}
		for _, ed := range adj {
			other := ed.To
			if other == cur.node {
				other = ed.From
			}
			if visited[other] {
				continue
			}
			visited[other] = true
			dist := cur.dist + 1
			contrib := seed.Risk * math.Pow(e.cfg.Decay, float64(dist))
			path := append(append([]string{}, cur.path...), other)
			if contrib >= e.cfg.MinContribution {
				steps = append(steps, schema.RiskPropagationStep{
					Seed:         seed.NodeID,
					Target:       other,
					Distance:     dist,
					Contribution: contrib,
					Path:         path,
				})
			}
			queue = append(queue, qitem{other, dist, path})
		}
	}
	return steps, nil
}

// PropagateAll runs propagation from multiple seeds and returns, per target,
// the maximum contribution received (a target's propagated risk is driven by
// its strongest nearby seed).
func (e *Engine) PropagateAll(ctx context.Context, seeds []Seed) (map[string]schema.RiskPropagationStep, error) {
	best := map[string]schema.RiskPropagationStep{}
	for _, s := range seeds {
		steps, err := e.Propagate(ctx, s)
		if err != nil {
			return nil, err
		}
		for _, st := range steps {
			if cur, ok := best[st.Target]; !ok || st.Contribution > cur.Contribution {
				best[st.Target] = st
			}
		}
	}
	return best, nil
}
