package acquisition

import (
	"context"
	"time"
)

// RateLimiter enforces a maximum request rate with a simple token-interval
// scheme. It is cancellation-aware. requests_per_second <= 0 disables limiting.
type RateLimiter struct {
	interval time.Duration
	tokens   chan struct{}
	stop     chan struct{}
}

// NewRateLimiter builds a limiter at rps requests/second (0 = unlimited).
func NewRateLimiter(rps float64) *RateLimiter {
	if rps <= 0 {
		return &RateLimiter{} // disabled
	}
	rl := &RateLimiter{
		interval: time.Duration(float64(time.Second) / rps),
		tokens:   make(chan struct{}, 1),
		stop:     make(chan struct{}),
	}
	// Prime one token so the first request proceeds immediately.
	rl.tokens <- struct{}{}
	go rl.refill()
	return rl
}

func (rl *RateLimiter) refill() {
	t := time.NewTicker(rl.interval)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-t.C:
			select {
			case rl.tokens <- struct{}{}:
			default: // bucket full; drop
			}
		}
	}
}

// Wait blocks until a token is available or ctx is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	if rl.tokens == nil {
		return ctx.Err() // disabled: only respect cancellation
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.tokens:
		return nil
	}
}

// Close stops the limiter's background goroutine.
func (rl *RateLimiter) Close() {
	if rl.stop != nil {
		close(rl.stop)
	}
}
