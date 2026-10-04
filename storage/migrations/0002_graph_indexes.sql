-- Phase 3 graph engine: composite indexes for adjacency queries.
-- The graph is queried by (from_type, from_id) and (to_type, to_id) during
-- bounded traversal, and by edge type for filtered expansion.

CREATE INDEX IF NOT EXISTS idx_edge_from_type ON graph_edges(from_type, from_id);
CREATE INDEX IF NOT EXISTS idx_edge_to_type   ON graph_edges(to_type, to_id);
CREATE INDEX IF NOT EXISTS idx_edge_type       ON graph_edges(type);
CREATE INDEX IF NOT EXISTS idx_edge_timestamp  ON graph_edges(timestamp);
