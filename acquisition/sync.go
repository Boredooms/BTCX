package acquisition

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bctx/bctx/blockchain/parser"
	"github.com/bctx/bctx/ingestion"
	"github.com/bctx/bctx/pkg/schema"
	"github.com/bctx/bctx/sdk"
)

// Config bundles the tunables the engine reads from AcquisitionConfig.
type Config struct {
	MaxWorkers        int
	PageSize          int
	BatchSize         int
	RequestTimeout    time.Duration
	Retry             RetryConfig
	RequestsPerSecond float64
	Seed              int64 // deterministic jitter in tests
}

// DefaultConfig returns engine defaults.
func DefaultConfig() Config {
	return Config{
		MaxWorkers: 4, PageSize: 100, BatchSize: 500,
		RequestTimeout: 15 * time.Second, Retry: DefaultRetryConfig(),
		RequestsPerSecond: 5,
	}
}

// Engine orchestrates provider acquisition into the canonical local store. It
// is the only component that calls a DataSource. It never computes risk/ML or
// mutates graph tables directly — persistence flows through the shared
// ingestion Persister (same path as file import).
type Engine struct {
	src       DataSource
	repo      sdk.Repository
	persister *ingestion.Persister
	limiter   *RateLimiter
	retrier   *Retrier
	cfg       Config
	caseID    string
}

// NewEngine builds a sync engine.
func NewEngine(src DataSource, repo sdk.Repository, caseID string, cfg Config) *Engine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = 100
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 500
	}
	return &Engine{
		src:       src,
		repo:      repo,
		persister: ingestion.NewPersister(repo),
		limiter:   NewRateLimiter(cfg.RequestsPerSecond),
		retrier:   NewRetrier(cfg.Retry, cfg.Seed),
		cfg:       cfg,
		caseID:    caseID,
	}
}

// Close releases engine resources.
func (e *Engine) Close() { e.limiter.Close() }

// Stats is the acquisition summary.
type Stats struct {
	SyncID     string
	Provider   string
	Target     string
	TargetType string
	Status     string
	Pages      int
	Discovered int
	Fetched    int
	New        int
	Duplicates int
	Partial    int
	Rejected   int
	Retries    int
	Elapsed    time.Duration
}

// SyncTransaction acquires a single transaction, idempotently. Returns
// ("already local") without a fetch when the tx is already present and
// complete.
func (e *Engine) SyncTransaction(ctx context.Context, txid string) (Stats, error) {
	start := time.Now()
	st := Stats{Provider: e.src.ProviderName(), Target: txid, TargetType: "tx",
		SyncID: "sync-tx-" + shortID(txid), Status: "running"}

	if !parser.ValidTxID(txid) {
		return st, ErrInvalidTarget
	}
	if !e.src.Capabilities().TransactionFetch {
		return st, ErrCapabilityUnsupported
	}

	// Local cache: skip refetch when already present and complete.
	if existing, err := e.repo.GetTransaction(ctx, txid); err == nil && existing != nil {
		if existing.Completeness != schema.CompletePartial {
			st.Status = "already_local"
			st.Elapsed = time.Since(start)
			e.saveCheckpoint(ctx, st, PaginationState{})
			return st, nil
		}
	}

	at, retries, err := e.fetchTx(ctx, txid)
	st.Retries += retries
	if err != nil {
		st.Status = "failed"
		e.saveCheckpointErr(ctx, st, err)
		return st, err
	}
	st.Fetched++
	at.Metadata = e.meta(st.SyncID, txid)

	res, perr := e.persist(ctx, []AcquiredTransaction{at}, nil)
	if perr != nil {
		return st, perr
	}
	applyPersist(&st, res)
	st.Status = "completed"
	st.Elapsed = time.Since(start)
	e.saveCheckpoint(ctx, st, PaginationState{})
	return st, nil
}

// SyncWallet acquires a wallet's history page-by-page with bounded-memory
// streaming: each page's txids are fetched concurrently (bounded), persisted in
// a batch, checkpointed, then the next page is processed. Idempotent + resumable.
func (e *Engine) SyncWallet(ctx context.Context, address string, resume PaginationState) (Stats, error) {
	start := time.Now()
	st := Stats{Provider: e.src.ProviderName(), Target: address, TargetType: "wallet",
		SyncID: "sync-wallet-" + shortID(address), Status: "running"}

	if address == "" {
		return st, ErrInvalidTarget
	}
	if !e.src.Capabilities().WalletHistory {
		return st, ErrCapabilityUnsupported
	}

	cursor := resume
	for {
		if err := ctx.Err(); err != nil {
			st.Status = "cancelled"
			e.saveCheckpoint(ctx, st, cursor)
			return st, ErrAcquisitionCancelled
		}

		page, retries, err := e.fetchPage(ctx, WalletHistoryRequest{
			Address: address, Cursor: cursor, PageSize: e.cfg.PageSize})
		st.Retries += retries
		if err != nil {
			st.Status = "failed"
			e.saveCheckpointErr(ctx, st, err)
			return st, err
		}
		st.Pages++

		// Collect txids: inline transactions + ids needing a detail fetch.
		inline := map[string]AcquiredTransaction{}
		for _, t := range page.Transactions {
			inline[t.TxID] = t
		}
		ids := append([]string{}, page.TxIDs...)
		for id := range inline {
			if !contains(page.TxIDs, id) {
				ids = append(ids, id)
			}
		}
		st.Discovered += len(ids)

		// Fetch details for ids not provided inline, with bounded concurrency.
		fetched := e.fetchDetails(ctx, ids, inline, &st)

		// Map observations.
		var obs []AcquiredNetworkObservation
		obs = append(obs, page.Observations...)

		for i := range fetched {
			fetched[i].Metadata = e.meta(st.SyncID, fetched[i].TxID)
		}
		res, perr := e.persist(ctx, fetched, obs)
		if perr != nil {
			return st, perr
		}
		applyPersist(&st, res)

		cursor = page.Next
		e.saveCheckpoint(ctx, st, cursor)

		if page.Done || cursor.IsZero() {
			break
		}
	}
	st.Status = "completed"
	st.Elapsed = time.Since(start)
	e.saveCheckpoint(ctx, st, PaginationState{})
	return st, nil
}

// SyncBlock acquires a block by hash: fetch block metadata + txids, fetch each
// transaction (bounded concurrency, reusing fetchDetails), persist through the
// canonical Persister (idempotent), persist the block header, and checkpoint.
// Large blocks stay bounded-memory: txids are fetched in chunks of BatchSize so
// the whole block is never held in memory at once. Re-acquisition is idempotent
// (dedup authoritative). The provider must implement BlockSource.
func (e *Engine) SyncBlock(ctx context.Context, hash string, height int, byHeight bool) (Stats, error) {
	start := time.Now()
	target := hash
	if byHeight {
		target = fmt.Sprintf("height:%d", height)
	}
	st := Stats{Provider: e.src.ProviderName(), Target: target, TargetType: "block",
		SyncID: "sync-block-" + shortID(target), Status: "running"}

	bs, ok := e.src.(BlockSource)
	if !ok || !e.src.Capabilities().BlockLookup {
		return st, ErrCapabilityUnsupported
	}

	// Fetch block metadata + txids (through retrier/limiter).
	var blk AcquiredBlock
	retries, err := e.retrier.Do(ctx, func() error {
		if werr := e.limiter.Wait(ctx); werr != nil {
			return ErrAcquisitionCancelled
		}
		rctx, cancel := e.reqCtx(ctx)
		defer cancel()
		var b AcquiredBlock
		var ferr error
		if byHeight {
			b, ferr = bs.FetchBlockByHeight(rctx, height)
		} else {
			b, ferr = bs.FetchBlock(rctx, hash)
		}
		if ferr != nil {
			return ferr
		}
		blk = b
		return nil
	})
	st.Retries += retries
	if err != nil {
		st.Status = "failed"
		e.saveCheckpointErr(ctx, st, err)
		return st, err
	}
	st.Target = blk.Hash
	st.Discovered = len(blk.TxIDs)

	// Fetch + persist the block's transactions in bounded chunks so a large
	// block (thousands of txs) never loads entirely into memory.
	chunk := e.cfg.BatchSize
	if chunk <= 0 {
		chunk = 500
	}
	for start := 0; start < len(blk.TxIDs); start += chunk {
		if cerr := ctx.Err(); cerr != nil {
			st.Status = "cancelled"
			e.saveCheckpoint(ctx, st, PaginationState{})
			return st, ErrAcquisitionCancelled
		}
		end := start + chunk
		if end > len(blk.TxIDs) {
			end = len(blk.TxIDs)
		}
		ids := blk.TxIDs[start:end]
		fetched := e.fetchDetails(ctx, ids, map[string]AcquiredTransaction{}, &st)
		for i := range fetched {
			fetched[i].Metadata = e.meta(st.SyncID, fetched[i].TxID)
		}
		res, perr := e.persist(ctx, fetched, nil)
		if perr != nil {
			return st, perr
		}
		applyPersist(&st, res)
		e.saveCheckpoint(ctx, st, PaginationState{Page: end})
	}

	// Persist the canonical block header (links txids -> this block).
	if berr := e.repo.SaveBlock(ctx, toCanonicalBlock(blk, e.meta(st.SyncID, blk.Hash))); berr != nil {
		return st, berr
	}

	st.Status = "completed"
	if st.Fetched < st.Discovered {
		st.Status = "partial" // some tx fetches failed; checkpoint retained
	}
	st.Elapsed = time.Since(start)
	e.saveCheckpoint(ctx, st, PaginationState{})
	return st, nil
}

// fetchDetails fetches transaction details with bounded concurrency, preserving
// a bounded work queue and no goroutine leaks.
func (e *Engine) fetchDetails(ctx context.Context, ids []string, inline map[string]AcquiredTransaction, st *Stats) []AcquiredTransaction {
	type result struct {
		tx      AcquiredTransaction
		ok      bool
		retries int
	}
	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup

	workers := e.cfg.MaxWorkers
	if workers > len(ids) && len(ids) > 0 {
		workers = len(ids)
	}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if t, ok := inline[id]; ok {
					results <- result{tx: t, ok: true}
					continue
				}
				// Local cache: skip refetch of complete local txs.
				if ex, err := e.repo.GetTransaction(ctx, id); err == nil && ex != nil &&
					ex.Completeness != schema.CompletePartial {
					results <- result{ok: false} // already local, no new fetch
					continue
				}
				t, r, err := e.fetchTx(ctx, id)
				if err != nil {
					results <- result{ok: false, retries: r}
					continue
				}
				results <- result{tx: t, ok: true, retries: r}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, id := range ids {
			select {
			case <-ctx.Done():
				return
			case jobs <- id:
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	var out []AcquiredTransaction
	for r := range results {
		st.Retries += r.retries
		if r.ok {
			st.Fetched++
			out = append(out, r.tx)
		}
	}
	return out
}

// fetchTx fetches one transaction through the rate limiter + retrier.
func (e *Engine) fetchTx(ctx context.Context, txid string) (AcquiredTransaction, int, error) {
	var at AcquiredTransaction
	retries, err := e.retrier.Do(ctx, func() error {
		if werr := e.limiter.Wait(ctx); werr != nil {
			return ErrAcquisitionCancelled
		}
		rctx, cancel := e.reqCtx(ctx)
		defer cancel()
		t, err := e.src.FetchTransaction(rctx, txid)
		if err != nil {
			return err
		}
		at = t
		return nil
	})
	return at, retries, err
}

// fetchPage fetches one wallet-history page through the rate limiter + retrier.
func (e *Engine) fetchPage(ctx context.Context, req WalletHistoryRequest) (AcquiredWalletPage, int, error) {
	var page AcquiredWalletPage
	retries, err := e.retrier.Do(ctx, func() error {
		if werr := e.limiter.Wait(ctx); werr != nil {
			return ErrAcquisitionCancelled
		}
		rctx, cancel := e.reqCtx(ctx)
		defer cancel()
		p, err := e.src.FetchWalletHistory(rctx, req)
		if err != nil {
			return err
		}
		page = p
		return nil
	})
	return page, retries, err
}

func (e *Engine) reqCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if e.cfg.RequestTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, e.cfg.RequestTimeout)
}

// persist maps acquired txs/obs to canonical and runs the shared Persister with
// enrichment enabled (acquisition may complete a locally-partial tx).
func (e *Engine) persist(ctx context.Context, txs []AcquiredTransaction, obs []AcquiredNetworkObservation) (ingestion.PersistResult, error) {
	ctxs := make([]schema.Transaction, 0, len(txs))
	for _, a := range txs {
		ctxs = append(ctxs, toCanonicalTx(a))
	}
	var cobs []schema.NetworkObservation
	for _, o := range obs {
		if co, ok := toCanonicalObs(o, e.meta("", o.TxID)); ok {
			cobs = append(cobs, co)
		}
	}
	return e.persister.PersistBatch(ctx, ctxs, cobs, true)
}

func (e *Engine) meta(syncID, srcID string) AcquisitionMetadata {
	return AcquisitionMetadata{
		Provider: e.src.ProviderName(), ProviderVersion: e.src.ProviderVersion(),
		SyncID: syncID, SourceObjectID: srcID, AcquiredAt: time.Now().UTC(),
	}
}

func (e *Engine) saveCheckpoint(ctx context.Context, st Stats, cursor PaginationState) {
	_ = e.repo.SaveSyncCheckpoint(ctx, e.toCheckpoint(st, cursor, ""))
}

func (e *Engine) saveCheckpointErr(ctx context.Context, st Stats, err error) {
	_ = e.repo.SaveSyncCheckpoint(ctx, e.toCheckpoint(st, PaginationState{}, err.Error()))
}

func (e *Engine) toCheckpoint(st Stats, cursor PaginationState, errStr string) sdk.SyncCheckpoint {
	now := time.Now().UTC().Format(time.RFC3339)
	return sdk.SyncCheckpoint{
		SyncID: st.SyncID, CaseID: e.caseID, Provider: st.Provider,
		ProviderVersion: e.src.ProviderVersion(), TargetType: st.TargetType,
		Target: st.Target, Cursor: cursor.Cursor, CursorPage: cursor.Page,
		PagesDone: st.Pages, Discovered: st.Discovered, Acquired: st.Fetched,
		Persisted: st.New, Duplicates: st.Duplicates, PartialCount: st.Partial,
		Rejected: st.Rejected, Retries: st.Retries, Status: st.Status,
		Error: errStr, StartedAt: now, UpdatedAt: now,
	}
}

func applyPersist(st *Stats, res ingestion.PersistResult) {
	st.New += res.Transactions + res.NetworkObs
	st.Duplicates += res.Duplicates
	st.Partial += res.Partial
	st.Rejected += res.Rejected
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func shortID(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16]
}

var _ = fmt.Sprintf
