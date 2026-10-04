// Package acquisition is the ONLY BCTX layer permitted to use the network. It
// fetches wallet/transaction data from a provider, converts it into the exact
// canonical records produced by local file ingestion, and hands them to the
// shared ingestion Persister. Nothing here computes risk, runs ML, or mutates
// graph tables directly — it only acquires and normalizes source data.
//
// The DataSource interface is capability-oriented and provider-neutral: the
// rest of the system never depends on a single vendor's response model.
package acquisition

import (
	"context"
	"time"
)

// ProviderCapabilities describes what a provider supports. Discovery
// (bctx provider list) reads this locally — it never requires the network.
type ProviderCapabilities struct {
	WalletHistory       bool `json:"wallet_history"`
	TransactionFetch    bool `json:"transaction_fetch"`
	Pagination          bool `json:"pagination"`
	RateLimited         bool `json:"rate_limited"`
	AuthRequired        bool `json:"auth_required"`
	LiveWalletEvents    bool `json:"live_wallet_events"`
	MempoolEvents       bool `json:"mempool_events"`
	ConfirmedEvents     bool `json:"confirmed_events"`
	NetworkObservations bool `json:"network_observations"`
	BlockLookup         bool `json:"block_lookup"`       // FetchBlock / FetchBlockByHeight
	BlockTransactions   bool `json:"block_transactions"` // FetchBlockTxIDs
}

// AcquiredBlock is the provider-neutral block DTO. Only fields a provider
// actually returns are populated; missing values stay zero and render as
// "unavailable" rather than being fabricated. The sync layer maps this into the
// canonical schema.Block; the adapter performs no analysis.
type AcquiredBlock struct {
	Hash           string
	Height         int
	TimestampEpoch int64 // unix seconds; 0 = unavailable (NOT 1970)
	PrevHash       string
	TxCount        int
	Size           int
	Weight         int
	MerkleRoot     string
	Confirmations  int
	HasConfs       bool // true when Confirmations is a real provider value
	TxIDs          []string
	Metadata       AcquisitionMetadata
}

// BlockSource is the OPTIONAL block-acquisition capability. A provider that
// declares Capabilities().BlockLookup implements it; callers type-assert for it
// so providers without block support are not forced to stub it. Network use is
// confined here exactly like DataSource.
type BlockSource interface {
	// FetchBlock returns block metadata + its txids by block hash.
	FetchBlock(ctx context.Context, hash string) (AcquiredBlock, error)
	// FetchBlockByHeight resolves a height to a block (metadata + txids).
	FetchBlockByHeight(ctx context.Context, height int) (AcquiredBlock, error)
}

// AcquisitionMetadata is attached to acquired records for provenance.
type AcquisitionMetadata struct {
	Provider        string    `json:"provider"`
	ProviderVersion string    `json:"provider_version"`
	SyncID          string    `json:"sync_id"`
	SourceObjectID  string    `json:"source_object_id,omitempty"`
	SourceRoute     string    `json:"source_route,omitempty"` // no secrets
	AcquiredAt      time.Time `json:"acquired_at"`
}

// AcquiredIO is a provider-neutral input/output.
type AcquiredIO struct {
	Address   string
	AmountBTC float64
}

// AcquiredNetworkObservation is a provider-neutral network observation.
type AcquiredNetworkObservation struct {
	SrcIP   string
	SrcPort int
	DstIP   string
	DstPort int
	Country string
	ASN     string
	TxID    string
}

// AcquiredTransaction is the provider-neutral transaction DTO. The sync engine
// maps this into the canonical schema.Transaction via the shared normalizer;
// the provider adapter never performs satoshi/size/feerate math itself.
type AcquiredTransaction struct {
	TxID       string
	Timestamp  string // raw source timestamp; normalized downstream
	FeeBTC     float64
	HasFee     bool
	ScriptType string
	BaseSize   int
	TotalSize  int
	Weight     int
	VSize      int
	Inputs     []AcquiredIO
	Outputs    []AcquiredIO
	// Partial marks that the provider could not return a complete tx.
	Partial bool
	// Confirmation state. Confirmed=false means mempool/unconfirmed. BlockHeight
	// is 0 when unconfirmed or unavailable. Never fabricate confirmation.
	Confirmed   bool
	BlockHeight int
	Metadata    AcquisitionMetadata
}

// AcquiredWalletPage is one page of wallet history.
type AcquiredWalletPage struct {
	Address string
	// TxIDs discovered on this page (details may be fetched separately).
	TxIDs []string
	// Transactions optionally carries full details if the provider returned
	// them inline (saves a round-trip).
	Transactions []AcquiredTransaction
	// Observations optionally carries network observations for this page.
	Observations []AcquiredNetworkObservation
	// Next is the pagination cursor for the following page; empty = end.
	Next PaginationState
	// Done is true when the provider signals end of history.
	Done bool
}

// PaginationState is a provider-neutral cursor. A provider uses whichever of
// Cursor/Page it supports; the sync engine treats it as opaque.
type PaginationState struct {
	Cursor string `json:"cursor,omitempty"`
	Page   int    `json:"page,omitempty"`
}

// IsZero reports an empty pagination state.
func (p PaginationState) IsZero() bool { return p.Cursor == "" && p.Page == 0 }

// WalletHistoryRequest expresses a wallet-history query.
type WalletHistoryRequest struct {
	Address  string
	Cursor   PaginationState
	PageSize int
	Since    time.Time // optional lower time bound (zero = none)
	Until    time.Time // optional upper time bound (zero = none)
	Limit    int       // optional max records (0 = none)
}

// DataSource is the capability-oriented acquisition contract. Implementations
// are the ONLY code permitted to open network connections.
type DataSource interface {
	ProviderName() string
	ProviderVersion() string
	Capabilities() ProviderCapabilities

	// FetchWalletHistory returns one page of history for a wallet.
	FetchWalletHistory(ctx context.Context, req WalletHistoryRequest) (AcquiredWalletPage, error)
	// FetchTransaction returns a single transaction's details.
	FetchTransaction(ctx context.Context, txid string) (AcquiredTransaction, error)
}
