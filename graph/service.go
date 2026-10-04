package graph

import (
	"context"

	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Service answers bounded traversal queries over the persisted graph. It is
// database-backed: adjacency is read per-node from the repository, so a query
// never loads the entire graph into memory — only the bounded frontier.
type Service struct {
	repo sdk.Repository
	// MaxNodes caps a single traversal to keep queries interactive.
	MaxNodes int
}

// NewService builds a graph service.
func NewService(repo sdk.Repository) *Service {
	return &Service{repo: repo, MaxNodes: 5000}
}

// adjacency returns the undirected neighbor edges of a node (both directions),
// used for reachability traversal.
func (s *Service) adjacency(ctx context.Context, nodeID string) ([]schema.GraphEdge, error) {
	out, err := s.repo.EdgesFrom(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	in, err := s.repo.EdgesTo(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return append(out, in...), nil
}

// edgeOther returns the node on the other end of an edge from nodeID, plus its
// type.
func edgeOther(e schema.GraphEdge, nodeID string) (string, schema.NodeType) {
	if e.From == nodeID {
		return e.To, e.ToType
	}
	return e.From, e.FromType
}

// Neighbors returns direct (depth-1) neighbor nodes of id. Satisfies
// sdk.GraphService.Neighbors.
func (s *Service) Neighbors(ctx context.Context, id string) ([]schema.GraphNode, error) {
	adj, err := s.adjacency(ctx, id)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var out []schema.GraphNode
	for _, e := range adj {
		other, otype := edgeOther(e, id)
		if other == id {
			continue
		}
		if _, ok := seen[other]; ok {
			continue
		}
		seen[other] = struct{}{}
		out = append(out, schema.GraphNode{ID: other, Type: otype})
	}
	return out, nil
}

// NeighborsDepth returns all nodes within `depth` hops via bounded BFS.
func (s *Service) NeighborsDepth(ctx context.Context, id string, depth int) ([]schema.GraphNode, error) {
	sg, err := s.Subgraph(ctx, id, depth)
	if err != nil {
		return nil, err
	}
	out := make([]schema.GraphNode, 0, len(sg.Nodes))
	for _, n := range sg.Nodes {
		if n.ID != id {
			out = append(out, n)
		}
	}
	return out, nil
}

// Subgraph extracts a bounded subgraph of radius `depth` around center via BFS.
// Satisfies sdk.GraphService.Subgraph.
func (s *Service) Subgraph(ctx context.Context, center string, depth int) (*schema.Subgraph, error) {
	if depth < 0 {
		depth = 0
	}
	nodeType := map[string]schema.NodeType{center: inferType(center)}
	visited := map[string]int{center: 0} // node -> discovered depth
	edgeSet := map[string]schema.GraphEdge{}
	frontier := []string{center}

	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, node := range frontier {
			adj, err := s.adjacency(ctx, node)
			if err != nil {
				return nil, err
			}
			for _, e := range adj {
				edgeSet[e.ID] = e
				other, otype := edgeOther(e, node)
				if _, ok := visited[other]; !ok {
					visited[other] = d + 1
					nodeType[other] = otype
					next = append(next, other)
					if len(visited) >= s.MaxNodes {
						return s.assemble(center, depth, visited, nodeType, edgeSet), nil
					}
				}
			}
		}
		frontier = next
	}
	return s.assemble(center, depth, visited, nodeType, edgeSet), nil
}

func (s *Service) assemble(center string, depth int, visited map[string]int,
	nodeType map[string]schema.NodeType, edgeSet map[string]schema.GraphEdge) *schema.Subgraph {
	sg := &schema.Subgraph{Center: center, Depth: depth}
	for id := range visited {
		sg.Nodes = append(sg.Nodes, schema.GraphNode{ID: id, Type: nodeType[id]})
	}
	for _, e := range edgeSet {
		// Only include edges whose both endpoints are within the visited set.
		if _, ok := visited[e.From]; !ok {
			continue
		}
		if _, ok := visited[e.To]; !ok {
			continue
		}
		sg.Edges = append(sg.Edges, e)
	}
	return sg
}

// Path returns the node id sequence of a shortest path src..dst (BFS), or an
// empty slice if none exists within MaxNodes. Satisfies sdk.GraphService.Path.
func (s *Service) Path(ctx context.Context, src, dst string) ([]string, error) {
	if src == dst {
		return []string{src}, nil
	}
	prev := map[string]string{src: ""}
	queue := []string{src}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		adj, err := s.adjacency(ctx, node)
		if err != nil {
			return nil, err
		}
		for _, e := range adj {
			other, _ := edgeOther(e, node)
			if _, ok := prev[other]; ok {
				continue
			}
			prev[other] = node
			if other == dst {
				return reconstruct(prev, src, dst), nil
			}
			queue = append(queue, other)
			if len(prev) >= s.MaxNodes {
				return nil, nil
			}
		}
	}
	return nil, nil
}

func reconstruct(prev map[string]string, src, dst string) []string {
	var rev []string
	for n := dst; n != ""; n = prev[n] {
		rev = append(rev, n)
		if n == src {
			break
		}
	}
	// reverse
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// inferType guesses a node type from its id prefix for display when the center
// node's type is not otherwise known. Transactions in BCTX use TX/hex ids;
// everything else defaults to wallet.
func inferType(id string) schema.NodeType {
	if len(id) >= 2 && (id[:2] == "TX" || id[:2] == "tx") {
		return schema.NodeTransaction
	}
	return schema.NodeWallet
}

var _ sdk.GraphService = (*Service)(nil)
