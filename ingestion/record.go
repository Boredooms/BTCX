// Package ingestion is the production BCTX data-ingestion layer. It consumes
// local CSV / JSON / NDJSON / XML files via streaming parsers, normalizes rows
// into canonical records, validates and deduplicates them, persists in atomic
// batches, updates the graph incrementally, and records provenance plus
// resumable checkpoints.
//
// The entire file-ingestion path is offline: no package here imports an HTTP
// client. Only future acquisition adapters may use the network.
package ingestion

// Format enumerates supported source formats.
type Format string

const (
	FormatCSV    Format = "csv"
	FormatJSON   Format = "json"
	FormatNDJSON Format = "ndjson"
	FormatXML    Format = "xml"
	FormatAuto   Format = "auto"
)

// ParserVersion / ImporterVersion stamp provenance and gate resume validation.
const (
	ParserVersion   = "bctx-parser-v1"
	ImporterVersion = "bctx-importer-v1"
	SchemaVersion   = "bctx-schema-v1"
)

// Row is the flat, source-agnostic record a parser emits. One canonical
// transaction may span multiple rows (one per input/output). Network-only rows
// carry the net_* fields. Fields absent in a source are left zero/empty.
type Row struct {
	RecordNo int

	// Transaction fields.
	TxID       string
	Timestamp  string
	Fee        string // BTC (decimal) or empty
	FeeSats    string // integer sats, optional alternative to Fee
	ScriptType string
	BaseSize   string
	TotalSize  string
	Weight     string
	VSize      string

	// One input/output per row (either may be set).
	InputAddress  string
	InputAmount   string // BTC
	OutputAddress string
	OutputAmount  string // BTC

	// Network observation fields (optional).
	SrcIP   string
	SrcPort string
	DstIP   string
	DstPort string
	Country string
	ASN     string

	// Raw holds a bounded source fragment for error diagnostics.
	Raw string
}

// IsNetworkRow reports whether the row carries network-observation data.
func (r Row) IsNetworkRow() bool {
	return r.SrcIP != "" || r.DstIP != ""
}

// Parser streams rows from a source. Implementations must not load the whole
// file into memory. Next returns io.EOF when the source is exhausted.
type Parser interface {
	// Next returns the next row. The returned bool is false at end of stream.
	Next() (Row, bool, error)
	Close() error
}
