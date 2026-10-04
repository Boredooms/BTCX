// Package schema defines the canonical domain contracts shared across every
// BCTX layer: storage, graph, features, ML, risk, evidence, reporting, CLI and
// TUI. These types are the single source of truth. No layer should define a
// duplicate version of these structures.
//
// Design rules (from AGENTS.md / CLAUDE.md):
//   - Core domain objects are strongly typed; avoid map[string]interface{}.
//   - Every finding is traceable to evidence.
//   - A risk score is always decomposable into signals + evidence + confidence.
package schema

import "time"

// SchemaVersion is the canonical data schema version stamped onto imported and
// acquired records so that model/feature compatibility can be validated.
const SchemaVersion = "schema-v1"

// FeatureSchemaVersion identifies the feature-vector layout. Models declare the
// feature schema they were trained against; a mismatch must be rejected.
const FeatureSchemaVersion = "feature-schema-v1"

// FeatureSchemaSHA256 is the authoritative hash of feature-schema-v1 (ordered
// feature names + version), produced by the ML lab
// (ml-lab/evaluation/feature_schema_v1.json). The ML runtime uses it for
// fail-closed model/feature compatibility checks.
const FeatureSchemaSHA256 = "d52d7e12ee047c207231b9c165fe64ad831e0d6ec120eea2f0e9cfede5a2b589"

// -----------------------------------------------------------------------------
// Provenance
// -----------------------------------------------------------------------------

// SourceType classifies where a record originated. Keeping provenance explicit
// lets the evidence engine distinguish blockchain, network and imported data.
type SourceType string

const (
	SourceExplorer  SourceType = "explorer_api"
	SourceFile      SourceType = "file_import"
	SourceSynthetic SourceType = "synthetic"
	SourceNetwork   SourceType = "network_telemetry"
	SourceUnknown   SourceType = "unknown"
)

// Provenance records the origin of a stored record for auditability.
type Provenance struct {
	SourceType       SourceType `json:"source_type"`
	SourceIdentifier string     `json:"source_identifier"`
	RetrievedAt      time.Time  `json:"retrieved_at"`
	SchemaVersion    string     `json:"schema_version"`
	ToolVersion      string     `json:"tool_version"`
	// DatasetID links the record back to the import/acquisition batch.
	DatasetID string `json:"dataset_id,omitempty"`
}

// -----------------------------------------------------------------------------
// Blockchain layer
// -----------------------------------------------------------------------------

// Wallet is an address/entity observed in the local dataset.
type Wallet struct {
	Address    string     `json:"address"`
	FirstSeen  time.Time  `json:"first_seen"`
	LastSeen   time.Time  `json:"last_seen"`
	TxCount    int        `json:"tx_count"`
	Provenance Provenance `json:"provenance"`
}

// SatsPerBTC is the canonical conversion factor. Monetary accounting is done in
// integer satoshis internally where exactness matters; AmountBTC/FeeBTC remain
// for compatibility and display.
const SatsPerBTC = 100_000_000

// BTCToSats converts a BTC float to integer satoshis (rounded).
func BTCToSats(btc float64) int64 {
	if btc < 0 {
		return -int64(-btc*float64(SatsPerBTC) + 0.5)
	}
	return int64(btc*float64(SatsPerBTC) + 0.5)
}

// SatsToBTC converts integer satoshis to a BTC float.
func SatsToBTC(sats int64) float64 { return float64(sats) / float64(SatsPerBTC) }

// Completeness classifies how complete a canonical record is. Downstream
// analysis distinguishes these; incomplete data is kept, not silently dropped.
type Completeness string

const (
	CompleteValid   Completeness = "valid"
	CompletePartial Completeness = "partial"
	CompleteInvalid Completeness = "invalid"
)

// TransactionInput is a spent output feeding a transaction.
type TransactionInput struct {
	Address    string  `json:"address"`
	AmountBTC  float64 `json:"amount_btc"`
	AmountSats int64   `json:"amount_sats"`
	Index      int     `json:"index"`
}

// TransactionOutput is a value destination of a transaction.
type TransactionOutput struct {
	Address    string  `json:"address"`
	AmountBTC  float64 `json:"amount_btc"`
	AmountSats int64   `json:"amount_sats"`
	Index      int     `json:"index"`
}

// TxSize holds canonical Bitcoin transaction-size metadata (BIP-141). Fields are
// optional: a value of 0 means "not supplied by the source". vsize = ceil(weight/4).
type TxSize struct {
	BaseSize  int `json:"base_size,omitempty"`  // non-witness bytes
	TotalSize int `json:"total_size,omitempty"` // base + witness bytes
	Weight    int `json:"weight,omitempty"`     // 3*base + total (BIP-141)
	VSize     int `json:"vsize,omitempty"`      // ceil(weight/4)
}

// Transaction is the canonical normalized transaction, independent of the
// source CSV/JSON/XML representation.
type Transaction struct {
	TxID       string              `json:"txid"`
	Timestamp  time.Time           `json:"timestamp"`
	Inputs     []TransactionInput  `json:"inputs"`
	Outputs    []TransactionOutput `json:"outputs"`
	FeeBTC     float64             `json:"fee_btc"`
	FeeSats    int64               `json:"fee_sats"`
	ScriptType string              `json:"script_type,omitempty"`
	// OriginalScriptType preserves the source's raw value before normalization.
	OriginalScriptType string `json:"original_script_type,omitempty"`
	Size               TxSize `json:"size"`
	// FeeRateSatVB is fee/vsize in sat/vByte (canonical fee-rate unit); 0 if
	// vsize unknown.
	FeeRateSatVB float64      `json:"feerate_sat_vb,omitempty"`
	Completeness Completeness `json:"completeness,omitempty"`
	Provenance   Provenance   `json:"provenance"`
}

// TotalInBTC returns the summed input value.
func (t Transaction) TotalInBTC() float64 {
	var s float64
	for _, in := range t.Inputs {
		s += in.AmountBTC
	}
	return s
}

// TotalOutBTC returns the summed output value.
func (t Transaction) TotalOutBTC() float64 {
	var s float64
	for _, out := range t.Outputs {
		s += out.AmountBTC
	}
	return s
}

// TotalInSats returns the summed input value in satoshis (exact).
func (t Transaction) TotalInSats() int64 {
	var s int64
	for _, in := range t.Inputs {
		s += in.AmountSats
	}
	return s
}

// TotalOutSats returns the summed output value in satoshis (exact).
func (t Transaction) TotalOutSats() int64 {
	var s int64
	for _, out := range t.Outputs {
		s += out.AmountSats
	}
	return s
}

// ConservationOK reports whether sum(inputs) == sum(outputs) + fee in exact
// satoshis. Only meaningful for complete transactions (both sides present).
func (t Transaction) ConservationOK() bool {
	return t.TotalInSats() == t.TotalOutSats()+t.FeeSats
}

// FanIn is the number of distinct input addresses.
func (t Transaction) FanIn() int { return len(t.Inputs) }

// FanOut is the number of distinct output addresses.
func (t Transaction) FanOut() int { return len(t.Outputs) }

// Block is a canonical Bitcoin block header + membership record. It is
// orchestration/provenance only: the block's transactions are stored as normal
// canonical Transactions (via the shared Persister), and TxIDs links them to
// this block. Fields a provider does not return stay zero/empty; HasConfs marks
// whether Confirmations is a real value (0 could be a tip block OR unavailable).
type Block struct {
	Hash          string     `json:"hash"`
	Height        int        `json:"height"`
	Timestamp     time.Time  `json:"timestamp"` // zero = unavailable
	PrevHash      string     `json:"prev_hash,omitempty"`
	TxCount       int        `json:"tx_count"`
	Size          int        `json:"size,omitempty"`
	Weight        int        `json:"weight,omitempty"`
	MerkleRoot    string     `json:"merkle_root,omitempty"`
	Confirmations int        `json:"confirmations,omitempty"`
	HasConfs      bool       `json:"has_confirmations"`
	TxIDs         []string   `json:"txids,omitempty"`
	Provenance    Provenance `json:"provenance"`
}

// -----------------------------------------------------------------------------
// Network layer
// -----------------------------------------------------------------------------

// NetworkObservation is a network-layer record correlated with a transaction.
// It is only ever populated from supplied/acquired telemetry — never invented.
type NetworkObservation struct {
	ID         string     `json:"id"`
	Timestamp  time.Time  `json:"timestamp"`
	TxID       string     `json:"txid,omitempty"`
	SrcIP      string     `json:"src_ip"`
	SrcPort    int        `json:"src_port"`
	DstIP      string     `json:"dst_ip"`
	DstPort    int        `json:"dst_port"`
	Country    string     `json:"country,omitempty"`
	ASN        string     `json:"asn,omitempty"`
	Provenance Provenance `json:"provenance"`
}

// -----------------------------------------------------------------------------
// Entities and graph
// -----------------------------------------------------------------------------

// NodeType enumerates graph node kinds.
type NodeType string

const (
	NodeWallet      NodeType = "wallet"
	NodeTransaction NodeType = "transaction"
	NodeIP          NodeType = "ip"
	NodeEntity      NodeType = "entity"
)

// EdgeType enumerates graph relationship kinds.
type EdgeType string

const (
	EdgeInputTo      EdgeType = "input_to"      // Wallet -> Transaction
	EdgeOutputTo     EdgeType = "output_to"     // Transaction -> Wallet
	EdgeSentTo       EdgeType = "sent_to"       // Wallet -> Wallet (derived)
	EdgeObservedWith EdgeType = "observed_with" // IP -> Transaction
	EdgeMemberOf     EdgeType = "member_of"     // Wallet -> Entity
)

// GraphEdge is a durable relationship between two nodes.
type GraphEdge struct {
	ID        string    `json:"id"`
	Type      EdgeType  `json:"type"`
	From      string    `json:"from"`
	FromType  NodeType  `json:"from_type"`
	To        string    `json:"to"`
	ToType    NodeType  `json:"to_type"`
	AmountBTC float64   `json:"amount_btc,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Entity is a logical grouping (person/service) backing one or more wallets.
type Entity struct {
	ID        string    `json:"id"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EntityCluster is an inferred relationship between wallets. A cluster is an
// INFERRED relationship, never proof of real-world ownership.
type EntityCluster struct {
	ID         string   `json:"id"`
	Members    []string `json:"members"`
	Confidence float64  `json:"confidence"`
	// Basis lists the signals that produced the cluster (e.g. common-input).
	Basis     []string  `json:"basis"`
	CreatedAt time.Time `json:"created_at"`
}

// Subgraph is a bounded extraction for rendering/analysis. The full graph is
// never materialized at once.
type Subgraph struct {
	Center string      `json:"center"`
	Depth  int         `json:"depth"`
	Nodes  []GraphNode `json:"nodes"`
	Edges  []GraphEdge `json:"edges"`
}

// GraphNode is a node within a subgraph view.
type GraphNode struct {
	ID    string   `json:"id"`
	Type  NodeType `json:"type"`
	Risk  float64  `json:"risk,omitempty"`
	Label string   `json:"label,omitempty"`
}
