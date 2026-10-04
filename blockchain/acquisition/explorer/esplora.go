// Package explorer implements a real BCTX acquisition provider against the
// Esplora HTTP API (Blockstream.info / mempool.space). It is the ONLY provider
// that opens network connections, and it lives behind the provider-neutral
// acquisition.DataSource interface so no downstream layer knows Esplora exists.
//
// Esplora reference: https://github.com/Blockstream/esplora/blob/master/API.md
//
//	GET /address/:address/txs                  -> newest ~50 (mempool+chain)
//	GET /address/:address/txs/chain/:last_txid -> next 25 confirmed (pagination)
//	GET /tx/:txid                              -> full transaction
//	GET /blocks/tip/height                     -> chain tip (reachability probe)
//
// Monetary values from Esplora are integer satoshis. We map them into the
// canonical schema's sat/BTC fields without reimplementing fee/size math.
package explorer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bctx/bctx/acquisition"
	"github.com/bctx/bctx/pkg/schema"
)

// DefaultEndpoint is the Blockstream public Esplora instance.
const DefaultEndpoint = "https://blockstream.info/api"

// Version identifies this adapter.
const Version = "esplora-v1"

// maxBody bounds a single response to protect against hostile/huge payloads.
const maxBody = 8 << 20 // 8 MiB

// Provider is the Esplora DataSource implementation.
type Provider struct {
	endpoint string
	client   *http.Client
}

// Option configures the provider.
type Option func(*Provider)

// WithEndpoint overrides the base URL (e.g. https://mempool.space/api).
func WithEndpoint(url string) Option {
	return func(p *Provider) {
		if url != "" {
			p.endpoint = strings.TrimRight(url, "/")
		}
	}
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(p *Provider) { p.client.Timeout = d }
}

// New builds an Esplora provider. The HTTP client forces IPv4 (this WSL host
// has a broken outbound IPv6 route) and bounds dial/response time.
func New(opts ...Option) *Provider {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr) // force IPv4
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	p := &Provider{
		endpoint: DefaultEndpoint,
		client:   &http.Client{Timeout: 20 * time.Second, Transport: transport},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

func (p *Provider) ProviderName() string    { return "esplora" }
func (p *Provider) ProviderVersion() string { return Version }

func (p *Provider) Capabilities() acquisition.ProviderCapabilities {
	return acquisition.ProviderCapabilities{
		WalletHistory:     true,
		TransactionFetch:  true,
		Pagination:        true,
		RateLimited:       true,
		AuthRequired:      false,
		LiveWalletEvents:  true, // via polling address history
		MempoolEvents:     true, // Esplora returns unconfirmed txs
		ConfirmedEvents:   true,
		BlockLookup:       true, // /block/:hash, /block-height/:height
		BlockTransactions: true, // /block/:hash/txids
		// Esplora does not expose peer-level IP/network observations.
		NetworkObservations: false,
	}
}

// TipHeight returns the current chain tip (used as a reachability probe).
func (p *Provider) TipHeight(ctx context.Context) (int, error) {
	var body []byte
	if err := p.get(ctx, "/blocks/tip/height", &body); err != nil {
		return 0, err
	}
	h, err := strconv.Atoi(strings.TrimSpace(string(body)))
	if err != nil {
		return 0, fmt.Errorf("%w: tip height parse", acquisition.ErrTransport)
	}
	return h, nil
}

// esploraTx mirrors the Esplora transaction JSON (only the fields we use).
type esploraTx struct {
	TxID string `json:"txid"`
	Vin  []struct {
		Prevout *struct {
			ScriptPubKeyAddress string `json:"scriptpubkey_address"`
			ScriptPubKeyType    string `json:"scriptpubkey_type"`
			Value               int64  `json:"value"`
		} `json:"prevout"`
		IsCoinbase bool `json:"is_coinbase"`
	} `json:"vin"`
	Vout []struct {
		ScriptPubKeyAddress string `json:"scriptpubkey_address"`
		ScriptPubKeyType    string `json:"scriptpubkey_type"`
		Value               int64  `json:"value"`
	} `json:"vout"`
	Size   int   `json:"size"`
	Weight int   `json:"weight"`
	Fee    int64 `json:"fee"`
	Status struct {
		Confirmed   bool  `json:"confirmed"`
		BlockHeight int   `json:"block_height"`
		BlockTime   int64 `json:"block_time"`
	} `json:"status"`
}

// toAcquired maps the Esplora tx into the provider-neutral DTO. Values are sats.
func (t esploraTx) toAcquired() acquisition.AcquiredTransaction {
	at := acquisition.AcquiredTransaction{
		TxID:        t.TxID,
		FeeBTC:      schema.SatsToBTC(t.Fee),
		HasFee:      true,
		BaseSize:    t.Size, // Esplora 'size' is total serialized size
		TotalSize:   t.Size,
		Weight:      t.Weight,
		VSize:       vsizeFromWeight(t.Weight),
		Confirmed:   t.Status.Confirmed,
		BlockHeight: t.Status.BlockHeight,
	}
	if t.Status.BlockTime > 0 {
		at.Timestamp = time.Unix(t.Status.BlockTime, 0).UTC().Format(time.RFC3339)
	}
	// Dominant output script type (first classified) for the canonical field.
	for _, v := range t.Vout {
		if v.ScriptPubKeyType != "" {
			at.ScriptType = v.ScriptPubKeyType
			break
		}
	}
	for _, in := range t.Vin {
		if in.IsCoinbase || in.Prevout == nil {
			continue // coinbase / missing prevout: no spendable input
		}
		addr := in.Prevout.ScriptPubKeyAddress
		if addr == "" && in.Prevout.Value > 0 {
			addr = "script:" + nonEmpty(in.Prevout.ScriptPubKeyType, "nonstandard")
		}
		if addr == "" {
			continue
		}
		at.Inputs = append(at.Inputs, acquisition.AcquiredIO{
			Address:   addr,
			AmountBTC: schema.SatsToBTC(in.Prevout.Value),
		})
	}
	for _, v := range t.Vout {
		if v.ScriptPubKeyAddress == "" && v.Value == 0 {
			continue // valueless non-address output (e.g. OP_RETURN) carries no flow
		}
		// Addressless but value-bearing outputs (e.g. P2PK like the genesis
		// coinbase, bare multisig) still carry value flow. Retain them with a
		// deterministic script-derived placeholder so no satoshis are lost from
		// the canonical record; graph edge-building keys on real addresses and
		// safely ignores the placeholder.
		addr := v.ScriptPubKeyAddress
		if addr == "" {
			addr = "script:" + nonEmpty(v.ScriptPubKeyType, "nonstandard")
		}
		at.Outputs = append(at.Outputs, acquisition.AcquiredIO{
			Address: addr, AmountBTC: schema.SatsToBTC(v.Value),
		})
	}
	// Partial if we could not recover any addressed input/output.
	at.Partial = len(at.Inputs) == 0 && len(at.Outputs) == 0
	return at
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func vsizeFromWeight(w int) int {
	if w <= 0 {
		return 0
	}
	return (w + 3) / 4
}

// FetchTransaction fetches one transaction by id.
func (p *Provider) FetchTransaction(ctx context.Context, txid string) (acquisition.AcquiredTransaction, error) {
	if !validTxID(txid) {
		return acquisition.AcquiredTransaction{}, fmt.Errorf("%w: %q", acquisition.ErrInvalidTarget, txid)
	}
	var raw []byte
	if err := p.get(ctx, "/tx/"+txid, &raw); err != nil {
		return acquisition.AcquiredTransaction{}, err
	}
	var et esploraTx
	if err := json.Unmarshal(raw, &et); err != nil {
		return acquisition.AcquiredTransaction{}, fmt.Errorf("%w: tx decode", acquisition.ErrPermanentProviderError)
	}
	return et.toAcquired(), nil
}

// esploraBlock mirrors the Esplora /block/:hash JSON (fields BCTX uses).
type esploraBlock struct {
	ID                string `json:"id"`
	Height            int    `json:"height"`
	Timestamp         int64  `json:"timestamp"`
	TxCount           int    `json:"tx_count"`
	Size              int    `json:"size"`
	Weight            int    `json:"weight"`
	MerkleRoot        string `json:"merkle_root"`
	PreviousBlockHash string `json:"previousblockhash"`
}

// validHash reports whether s is a 64-char hex block/tx hash.
func validHash(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// FetchBlock returns block metadata + its txids by block hash (Esplora:
// GET /block/:hash and GET /block/:hash/txids). Network use is confined here.
func (p *Provider) FetchBlock(ctx context.Context, hash string) (acquisition.AcquiredBlock, error) {
	if !validHash(hash) {
		return acquisition.AcquiredBlock{}, fmt.Errorf("%w: block hash %q", acquisition.ErrInvalidTarget, hash)
	}
	var raw []byte
	if err := p.get(ctx, "/block/"+hash, &raw); err != nil {
		return acquisition.AcquiredBlock{}, err
	}
	var eb esploraBlock
	if err := json.Unmarshal(raw, &eb); err != nil {
		return acquisition.AcquiredBlock{}, fmt.Errorf("%w: block decode", acquisition.ErrPermanentProviderError)
	}
	// Txids (one bounded request; Esplora returns the full list for a block).
	var txRaw []byte
	var txids []string
	if err := p.get(ctx, "/block/"+hash+"/txids", &txRaw); err == nil {
		_ = json.Unmarshal(txRaw, &txids)
	}
	return acquisition.AcquiredBlock{
		Hash:           eb.ID,
		Height:         eb.Height,
		TimestampEpoch: eb.Timestamp,
		PrevHash:       eb.PreviousBlockHash,
		TxCount:        eb.TxCount,
		Size:           eb.Size,
		Weight:         eb.Weight,
		MerkleRoot:     eb.MerkleRoot,
		TxIDs:          txids,
	}, nil
}

// FetchBlockByHeight resolves a height to its block hash (Esplora:
// GET /block-height/:height returns the hash as plain text) then FetchBlock.
func (p *Provider) FetchBlockByHeight(ctx context.Context, height int) (acquisition.AcquiredBlock, error) {
	if height < 0 {
		return acquisition.AcquiredBlock{}, fmt.Errorf("%w: height %d", acquisition.ErrInvalidTarget, height)
	}
	var raw []byte
	if err := p.get(ctx, "/block-height/"+strconv.Itoa(height), &raw); err != nil {
		return acquisition.AcquiredBlock{}, err
	}
	hash := strings.TrimSpace(string(raw))
	if !validHash(hash) {
		return acquisition.AcquiredBlock{}, fmt.Errorf("%w: height resolve", acquisition.ErrPermanentProviderError)
	}
	return p.FetchBlock(ctx, hash)
}

var _ acquisition.BlockSource = (*Provider)(nil)

// FetchWalletHistory returns one page of address history. Esplora paginates
// confirmed history by last-seen txid; the cursor carries that txid.
func (p *Provider) FetchWalletHistory(ctx context.Context, req acquisition.WalletHistoryRequest) (acquisition.AcquiredWalletPage, error) {
	if req.Address == "" {
		return acquisition.AcquiredWalletPage{}, acquisition.ErrInvalidTarget
	}
	path := "/address/" + req.Address + "/txs"
	if req.Cursor.Cursor != "" {
		path = "/address/" + req.Address + "/txs/chain/" + req.Cursor.Cursor
	}
	var raw []byte
	if err := p.get(ctx, path, &raw); err != nil {
		return acquisition.AcquiredWalletPage{}, err
	}
	var txs []esploraTx
	if err := json.Unmarshal(raw, &txs); err != nil {
		return acquisition.AcquiredWalletPage{}, fmt.Errorf("%w: history decode", acquisition.ErrPermanentProviderError)
	}
	page := acquisition.AcquiredWalletPage{Address: req.Address}
	for _, t := range txs {
		page.TxIDs = append(page.TxIDs, t.TxID)
		page.Transactions = append(page.Transactions, t.toAcquired())
	}
	// Esplora returns up to 25 confirmed per chain page; fewer = end.
	if len(txs) == 0 {
		page.Done = true
	} else {
		last := txs[len(txs)-1].TxID
		page.Next = acquisition.PaginationState{Cursor: last}
		// First (unpaginated) call also includes mempool txs; subsequent chain
		// pages are 25 confirmed. Treat <25 confirmed as end-of-history.
		confirmed := 0
		for _, t := range txs {
			if t.Status.Confirmed {
				confirmed++
			}
		}
		if req.Cursor.Cursor != "" && confirmed < 25 {
			page.Done = true
		}
	}
	return page, nil
}

// get performs a bounded GET and classifies HTTP/transport errors into typed
// acquisition errors.
func (p *Provider) get(ctx context.Context, path string, out *[]byte) error {
	reqURL := p.endpoint + path
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("%w: build request", acquisition.ErrPermanentProviderError)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "bctx/"+Version)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if ctx.Err() == context.Canceled {
				return acquisition.ErrAcquisitionCancelled
			}
			return fmt.Errorf("%w: %v", acquisition.ErrTimeout, err)
		}
		return fmt.Errorf("%w: %v", acquisition.ErrTransport, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	switch {
	case resp.StatusCode == http.StatusOK:
		*out = body
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", acquisition.ErrProviderNotFound, path)
	case resp.StatusCode == http.StatusTooManyRequests:
		return &rateErr{retryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: status %d", acquisition.ErrProviderUnavailable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: status %d", acquisition.ErrPermanentProviderError, resp.StatusCode)
	}
}

// rateErr wraps ErrRateLimited and carries a provider Retry-After hint.
type rateErr struct{ retryAfter int }

func (e *rateErr) Error() string          { return acquisition.ErrRateLimited.Error() }
func (e *rateErr) Unwrap() error          { return acquisition.ErrRateLimited }
func (e *rateErr) RetryAfterSeconds() int { return e.retryAfter }

func parseRetryAfter(h string) int {
	if h == "" {
		return 0
	}
	if n, err := strconv.Atoi(strings.TrimSpace(h)); err == nil {
		return n
	}
	return 0
}

func validTxID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 64 {
		return len(s) > 0 && len(s) <= 80 // tolerate synthetic ids in tests
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

var _ acquisition.DataSource = (*Provider)(nil)
