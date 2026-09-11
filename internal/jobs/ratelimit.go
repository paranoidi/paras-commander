package jobs

import (
	"context"
	"sync"
	"time"
)

// RateLimiter is a thread-safe token bucket throttling bytes/sec. A zero value is unlimited.
type RateLimiter struct {
	mu       sync.Mutex
	limitBPS int64 // 0 = unlimited
	tokens   float64
	last     time.Time
}

// SetLimit sets the limit in bytes/sec (0 = unlimited) and resets the bucket so switching
// limits does not cause an immediate stall (stale deficit) or unbounded burst (stale
// surplus). The bucket starts full (one second's worth of the new limit) rather than empty,
// so the very next write is not needlessly delayed either.
func (r *RateLimiter) SetLimit(bps int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limitBPS = bps
	r.tokens = float64(bps)
	r.last = time.Time{}
}

// Limit returns the current limit in bytes/sec (0 = unlimited).
func (r *RateLimiter) Limit() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.limitBPS
}

// Wait blocks until n bytes may be spent under the current limit, or ctx is canceled.
// Unlimited (limit <= 0) returns immediately without locking overhead.
func (r *RateLimiter) Wait(ctx context.Context, n int) error {
	if r.Limit() <= 0 {
		return nil
	}

	r.mu.Lock()
	limitBPS := r.limitBPS
	if limitBPS <= 0 {
		r.mu.Unlock()
		return nil
	}
	now := time.Now()
	if r.last.IsZero() {
		r.last = now
	}
	elapsed := now.Sub(r.last).Seconds()
	r.last = now
	r.tokens += elapsed * float64(limitBPS)
	// Cap burst credit at one second's worth so a freshly-lowered limit or an idle spell
	// doesn't let a huge stale surplus through at once.
	if burst := float64(limitBPS); r.tokens > burst {
		r.tokens = burst
	}
	r.tokens -= float64(n)
	deficit := -r.tokens
	r.mu.Unlock()

	if deficit <= 0 {
		return nil
	}
	dur := time.Duration(deficit / float64(limitBPS) * float64(time.Second))
	select {
	case <-time.After(dur):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
