package acquisition

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryConfig controls exponential backoff with jitter.
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns conservative defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{MaxAttempts: 4, InitialBackoff: 200 * time.Millisecond, MaxBackoff: 5 * time.Second}
}

// Retrier runs an operation with bounded exponential backoff + jitter, retrying
// only retryable errors. It is cancellation-aware and never loops forever.
type Retrier struct {
	cfg  RetryConfig
	rng  *rand.Rand
	wait func(ctx context.Context, d time.Duration) error // injectable for tests
}

// NewRetrier builds a retrier. seed makes jitter deterministic in tests.
func NewRetrier(cfg RetryConfig, seed int64) *Retrier {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	return &Retrier{cfg: cfg, rng: rand.New(rand.NewSource(seed)), wait: sleepCtx}
}

// Attempts returns the configured max attempts.
func (r *Retrier) Attempts() int { return r.cfg.MaxAttempts }

// Do runs op up to MaxAttempts times. It returns the last error and the number
// of retries performed (attempts-1 on failure).
func (r *Retrier) Do(ctx context.Context, op func() error) (retries int, err error) {
	for attempt := 1; attempt <= r.cfg.MaxAttempts; attempt++ {
		if cerr := ctx.Err(); cerr != nil {
			return retries, ErrAcquisitionCancelled
		}
		err = op()
		if err == nil {
			return retries, nil
		}
		if !Retryable(err) {
			return retries, err // permanent: stop immediately
		}
		if attempt == r.cfg.MaxAttempts {
			break // exhausted
		}
		retries++
		delay := r.backoff(attempt, err)
		if werr := r.wait(ctx, delay); werr != nil {
			return retries, ErrAcquisitionCancelled
		}
	}
	return retries, err
}

// backoff computes delay = min(initial * 2^(attempt-1), max) with full jitter,
// honoring a provider-supplied RetryAfter when present.
func (r *Retrier) backoff(attempt int, err error) time.Duration {
	var ra RetryAfter
	if errors.As(err, &ra) && ra.RetryAfterSeconds() > 0 {
		d := time.Duration(ra.RetryAfterSeconds()) * time.Second
		if d > r.cfg.MaxBackoff {
			d = r.cfg.MaxBackoff
		}
		return d
	}
	base := float64(r.cfg.InitialBackoff) * math.Pow(2, float64(attempt-1))
	if base > float64(r.cfg.MaxBackoff) {
		base = float64(r.cfg.MaxBackoff)
	}
	// Full jitter in [0, base].
	return time.Duration(r.rng.Float64() * base)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
