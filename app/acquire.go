package app

import "context"

// acquire.go defines the TUI-facing acquisition seam. The TUI (and tui/screens)
// import ONLY this interface — never a provider adapter or net/http. The
// concrete implementation is constructed at the cli/commands composition root
// (the sole importer of the explorer provider) and injected into the Root, so
// the TUI's import closure stays transport-free and the offline-boundary test
// keeps passing. This mirrors the llm.Summarizer injection pattern.

// AcquireKind is the subject class an acquisition targets.
type AcquireKind string

const (
	AcquireWallet AcquireKind = "wallet"
	AcquireTx     AcquireKind = "tx"
	AcquireBlock  AcquireKind = "block"
)

// AcquireRequest is one explicit, user-confirmed acquisition. Height is used
// only when Kind==AcquireBlock and ByHeight is true; otherwise Target carries
// the address / txid / block hash.
type AcquireRequest struct {
	Kind     AcquireKind
	Target   string
	Height   int
	ByHeight bool
}

// AcquireResult is the provider-neutral summary the TUI renders in its progress
// / completion modal. It is a flat copy of the engine Stats so the TUI never
// imports the acquisition package.
type AcquireResult struct {
	Provider   string
	Target     string
	TargetType string
	Status     string // running|completed|partial|failed|cancelled|already_local
	Discovered int
	Fetched    int
	New        int
	Duplicates int
	Partial    int
	Retries    int
	ElapsedMS  int64
	// Subject is the id the caller should open after acquisition (the resolved
	// block hash for a by-height block, else the original target).
	Subject string
}

// AcquireService performs a single explicit acquisition against the active
// case's repository, fully synchronously (the TUI runs it inside a cancellable
// tea.Cmd off the UI thread). It is the ONLY network-touching dependency the
// TUI holds, and only via this interface. A nil AcquireService means the TUI is
// offline-only: it must NOT offer network acquisition.
type AcquireService interface {
	// Available reports whether network acquisition is currently permitted
	// (online + acquisition enabled + not --offline/--airgap). The TUI uses this
	// to show "Acquire?" vs an honest "acquisition unavailable offline".
	Available() bool
	// ProviderName is the configured provider's display name (for the modal).
	ProviderName() string
	// Acquire runs one acquisition. It respects ctx cancellation (Esc/quit) and
	// returns a result even on partial/cancelled so the TUI can report honestly.
	Acquire(ctx context.Context, req AcquireRequest) (AcquireResult, error)
}
