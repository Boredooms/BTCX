// Package features is the Go side of the BCTX feature engine. In this ML phase
// it provides the parity harness that proves the Go feature contract matches
// the frozen Python feature-schema-v1 golden fixtures. The full streaming
// feature computation from raw transactions is implemented in a later phase;
// the schema/order contract is locked here so that work cannot drift.
package features

import (
	"encoding/json"
	"fmt"
	"os"
)

// FeatureSchemaVersion must match ml-lab/ml_lab/schema.py and pkg/schema.
const FeatureSchemaVersion = "feature-schema-v1"

// Order is the authoritative 25-feature order. Any Go feature computation must
// emit values in exactly this order.
var Order = []string{
	"tx_count", "incoming_count", "outgoing_count", "incoming_volume",
	"outgoing_volume", "avg_amount", "amount_variance", "fee_mean",
	"tx_per_hour", "median_time_gap", "burstiness", "velocity", "degree",
	"fan_in", "fan_out", "counterparty_diversity", "graph_depth", "hop_count",
	"chain_length", "value_decay", "split_ratio", "merge_ratio", "obs_count",
	"unique_ip_count", "unique_asn_count",
}

// Golden mirrors ml-lab/evaluation/feature_golden.json.
type Golden struct {
	SchemaVersion string      `json:"schema_version"`
	SchemaSHA256  string      `json:"schema_sha256"`
	FeatureOrder  []string    `json:"feature_order"`
	Fixtures      []GoldenRow `json:"fixtures"`
}

// GoldenRow is one fixture: a subject and its expected feature vector.
type GoldenRow struct {
	SubjectID string    `json:"subject_id"`
	Vector    []float64 `json:"vector"`
}

// LoadGolden reads the golden fixtures JSON produced by the Python freezer.
func LoadGolden(path string) (*Golden, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read golden: %w", err)
	}
	var g Golden
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse golden: %w", err)
	}
	return &g, nil
}

// VerifyOrder confirms the Go feature order matches the golden feature order
// exactly (names and positions). This is the production parity gate: if the Go
// engine's order drifts from the frozen Python schema, this fails.
func (g *Golden) VerifyOrder() error {
	if g.SchemaVersion != FeatureSchemaVersion {
		return fmt.Errorf("schema version mismatch: golden=%s go=%s",
			g.SchemaVersion, FeatureSchemaVersion)
	}
	if len(g.FeatureOrder) != len(Order) {
		return fmt.Errorf("feature count mismatch: golden=%d go=%d",
			len(g.FeatureOrder), len(Order))
	}
	for i := range Order {
		if g.FeatureOrder[i] != Order[i] {
			return fmt.Errorf("feature[%d] mismatch: golden=%q go=%q",
				i, g.FeatureOrder[i], Order[i])
		}
	}
	for _, row := range g.Fixtures {
		if len(row.Vector) != len(Order) {
			return fmt.Errorf("fixture %s vector len=%d want %d",
				row.SubjectID, len(row.Vector), len(Order))
		}
	}
	return nil
}
