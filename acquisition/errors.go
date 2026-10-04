package acquisition

import "errors"

// Typed acquisition errors so the CLI and future TUI can branch on cause
// without fragile string matching. Providers wrap these; the sync engine and
// retry classifier inspect them with errors.Is.
var (
	// ErrProviderUnavailable: the provider is temporarily unreachable (retryable).
	ErrProviderUnavailable = errors.New("provider unavailable")
	// ErrRateLimited: the provider throttled the request (retryable, honor delay).
	ErrRateLimited = errors.New("provider rate limited")
	// ErrProviderNotFound: the target does not exist at the provider (permanent).
	ErrProviderNotFound = errors.New("not found at provider")
	// ErrInvalidTarget: the address/txid failed canonical validation (permanent).
	ErrInvalidTarget = errors.New("invalid acquisition target")
	// ErrCapabilityUnsupported: the provider cannot perform the request.
	ErrCapabilityUnsupported = errors.New("provider capability unsupported")
	// ErrSyncCheckpointConflict: resume target/provider/config mismatch.
	ErrSyncCheckpointConflict = errors.New("sync checkpoint conflict")
	// ErrOfflineAcquisition: acquisition requested while offline/airgapped.
	ErrOfflineAcquisition = errors.New("acquisition unavailable offline")
	// ErrPermanentProviderError: a non-retryable provider error.
	ErrPermanentProviderError = errors.New("permanent provider error")
	// ErrAcquisitionCancelled: context cancelled mid-acquisition.
	ErrAcquisitionCancelled = errors.New("acquisition cancelled")
	// ErrTransport: a transient transport failure (retryable).
	ErrTransport = errors.New("transport error")
	// ErrTimeout: a request timed out (retryable).
	ErrTimeout = errors.New("request timeout")
)

// RetryAfter is an optional interface a provider error can implement to signal
// a server-specified retry delay (seconds). Honored by the rate limiter.
type RetryAfter interface {
	RetryAfterSeconds() int
}

// Retryable reports whether an error is worth retrying with backoff. Only
// transient categories retry; validation/not-found/capability are permanent.
func Retryable(err error) bool {
	switch {
	case errors.Is(err, ErrProviderUnavailable),
		errors.Is(err, ErrRateLimited),
		errors.Is(err, ErrTransport),
		errors.Is(err, ErrTimeout):
		return true
	default:
		return false
	}
}
