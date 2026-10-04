package acquisition

import (
	"context"
	"fmt"
)

// FakeProvider is a deterministic, in-memory DataSource for tests and local
// demos. It is NOT a production provider: it opens no network connections. It
// is selected only when explicitly configured (provider = "fake").
//
// Scenarios are driven by the seeded data and the behavior knobs below.
type FakeProvider struct {
	Version string
	// Wallets maps address -> ordered txids (across pages).
	Wallets map[string][]string
	// Txs maps txid -> transaction detail.
	Txs map[string]AcquiredTransaction
	// Observations maps txid -> observations returned with a wallet page.
	Observations map[string][]AcquiredNetworkObservation
	// PageSize overrides the request page size for deterministic multi-page.
	PageSize int

	// Behavior knobs (per-target), evaluated before returning:
	NotFound    map[string]bool // txid/address -> ErrProviderNotFound
	FailNTimes  map[string]int  // txid -> transient failures before success
	Throttle    map[string]bool // txid -> ErrRateLimited (retryable)
	Permanent   map[string]bool // txid -> ErrPermanentProviderError
	NoWalletCap bool
	NoTxCap     bool

	attempts map[string]int
}

// NewFakeProvider builds an empty deterministic provider.
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{
		Version:      "fake-v1",
		Wallets:      map[string][]string{},
		Txs:          map[string]AcquiredTransaction{},
		Observations: map[string][]AcquiredNetworkObservation{},
		PageSize:     2,
		NotFound:     map[string]bool{},
		FailNTimes:   map[string]int{},
		Throttle:     map[string]bool{},
		Permanent:    map[string]bool{},
		attempts:     map[string]int{},
	}
}

func (f *FakeProvider) ProviderName() string    { return "fake" }
func (f *FakeProvider) ProviderVersion() string { return f.Version }

func (f *FakeProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		WalletHistory:    !f.NoWalletCap,
		TransactionFetch: !f.NoTxCap,
		Pagination:       true,
		RateLimited:      true,
		AuthRequired:     false,
		LiveWalletEvents: true,
		MempoolEvents:    true,
		ConfirmedEvents:  true,
	}
}

// FetchWalletHistory returns one page of txids using Page-based pagination.
func (f *FakeProvider) FetchWalletHistory(ctx context.Context, req WalletHistoryRequest) (AcquiredWalletPage, error) {
	if err := ctx.Err(); err != nil {
		return AcquiredWalletPage{}, ErrAcquisitionCancelled
	}
	if f.NotFound[req.Address] {
		return AcquiredWalletPage{}, fmt.Errorf("wallet %s: %w", req.Address, ErrProviderNotFound)
	}
	all, ok := f.Wallets[req.Address]
	if !ok {
		return AcquiredWalletPage{Address: req.Address, Done: true}, nil
	}
	size := f.PageSize
	if req.PageSize > 0 {
		size = req.PageSize
	}
	page := req.Cursor.Page // 0-based
	startIdx := page * size
	if startIdx >= len(all) {
		return AcquiredWalletPage{Address: req.Address, Done: true}, nil
	}
	end := startIdx + size
	if end > len(all) {
		end = len(all)
	}
	ids := append([]string{}, all[startIdx:end]...)

	var obs []AcquiredNetworkObservation
	for _, id := range ids {
		obs = append(obs, f.Observations[id]...)
	}

	next := PaginationState{Page: page + 1}
	done := end >= len(all)
	if done {
		next = PaginationState{}
	}
	return AcquiredWalletPage{
		Address: req.Address, TxIDs: ids, Observations: obs,
		Next: next, Done: done,
	}, nil
}

// FetchTransaction returns a transaction, applying behavior knobs.
func (f *FakeProvider) FetchTransaction(ctx context.Context, txid string) (AcquiredTransaction, error) {
	if err := ctx.Err(); err != nil {
		return AcquiredTransaction{}, ErrAcquisitionCancelled
	}
	if f.Permanent[txid] {
		return AcquiredTransaction{}, fmt.Errorf("tx %s: %w", txid, ErrPermanentProviderError)
	}
	if f.NotFound[txid] {
		return AcquiredTransaction{}, fmt.Errorf("tx %s: %w", txid, ErrProviderNotFound)
	}
	if n := f.FailNTimes[txid]; n > 0 {
		f.attempts[txid]++
		if f.attempts[txid] <= n {
			return AcquiredTransaction{}, fmt.Errorf("tx %s transient: %w", txid, ErrTransport)
		}
	}
	if f.Throttle[txid] {
		// Throttle once, then succeed (deterministic).
		f.attempts["throttle:"+txid]++
		if f.attempts["throttle:"+txid] == 1 {
			return AcquiredTransaction{}, fmt.Errorf("tx %s: %w", txid, ErrRateLimited)
		}
	}
	t, ok := f.Txs[txid]
	if !ok {
		return AcquiredTransaction{}, fmt.Errorf("tx %s: %w", txid, ErrProviderNotFound)
	}
	return t, nil
}

var _ DataSource = (*FakeProvider)(nil)
