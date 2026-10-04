// Package edges defines deterministic graph-edge construction helpers shared by
// the builder and traversal. Edge ids are deterministic so that rebuilding the
// graph is idempotent (INSERT OR REPLACE on the same id collapses duplicates).
package edges

import (
	"crypto/sha1"
	"encoding/hex"
	"time"

	"github.com/bctx/bctx/pkg/schema"
)

// ID returns a deterministic edge id from its defining fields. Two edges with
// the same (type, from, to) collapse to one row, making graph build idempotent.
func ID(t schema.EdgeType, from, to string) string {
	h := sha1.Sum([]byte(string(t) + "|" + from + "|" + to))
	return "e_" + hex.EncodeToString(h[:10])
}

// New builds a graph edge with a deterministic id.
func New(t schema.EdgeType, from string, fromType schema.NodeType,
	to string, toType schema.NodeType, amount float64, ts time.Time) schema.GraphEdge {
	return schema.GraphEdge{
		ID:        ID(t, from, to),
		Type:      t,
		From:      from,
		FromType:  fromType,
		To:        to,
		ToType:    toType,
		AmountBTC: amount,
		Timestamp: ts,
	}
}
