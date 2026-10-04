package monitoring

import (
	"context"
	"sync"

	"github.com/bctx/bctx/acquisition"
)

// scriptedProvider is a deterministic monitoring provider whose wallet-history
// response changes per poll, enabling live-scenario tests (new tx over time,
// mempool->confirmed, outage, reconnect, duplicates). No network.
type scriptedProvider struct {
	mu      sync.Mutex
	polls   int
	pages   []acquisition.AcquiredWalletPage // page returned at poll i (clamped to last)
	failAt  map[int]error                    // poll index -> transient error
	caps    acquisition.ProviderCapabilities
	callLog int
}

func newScripted(pages []acquisition.AcquiredWalletPage) *scriptedProvider {
	return &scriptedProvider{
		pages:  pages,
		failAt: map[int]error{},
		caps: acquisition.ProviderCapabilities{
			WalletHistory: true, TransactionFetch: true, Pagination: true,
			LiveWalletEvents: true, MempoolEvents: true, ConfirmedEvents: true,
		},
	}
}

func (s *scriptedProvider) ProviderName() string                           { return "scripted" }
func (s *scriptedProvider) ProviderVersion() string                        { return "scripted-v1" }
func (s *scriptedProvider) Capabilities() acquisition.ProviderCapabilities { return s.caps }

func (s *scriptedProvider) FetchWalletHistory(ctx context.Context, req acquisition.WalletHistoryRequest) (acquisition.AcquiredWalletPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callLog++
	i := s.polls
	s.polls++
	if err, ok := s.failAt[i]; ok {
		return acquisition.AcquiredWalletPage{}, err
	}
	if len(s.pages) == 0 {
		return acquisition.AcquiredWalletPage{Address: req.Address, Done: true}, nil
	}
	if i >= len(s.pages) {
		i = len(s.pages) - 1
	}
	return s.pages[i], nil
}

func (s *scriptedProvider) FetchTransaction(ctx context.Context, txid string) (acquisition.AcquiredTransaction, error) {
	return acquisition.AcquiredTransaction{}, acquisition.ErrProviderNotFound
}

func (s *scriptedProvider) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callLog
}
