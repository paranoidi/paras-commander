package jobs

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterUnlimitedByDefault(t *testing.T) {
	var r RateLimiter
	if got := r.Limit(); got != 0 {
		t.Fatalf("Limit() = %d, want 0", got)
	}
	start := time.Now()
	if err := r.Wait(context.Background(), 10<<20); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("unlimited Wait should return immediately")
	}
}

func TestRateLimiterThrottles(t *testing.T) {
	var r RateLimiter
	r.SetLimit(10_000) // 10KB/s
	start := time.Now()
	// First call consumes the empty bucket instantly (no elapsed time to refill from).
	if err := r.Wait(context.Background(), 10_000); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	// Second call must wait ~200ms to earn back tokens for another 2000 bytes.
	if err := r.Wait(context.Background(), 2_000); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least ~200ms", elapsed)
	}
}

func TestRateLimiterWaitRespectsContextCancel(t *testing.T) {
	var r RateLimiter
	r.SetLimit(1) // ~1 byte/s, so any nontrivial n forces a long sleep
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := r.Wait(ctx, 1<<20)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestRateLimiterSetLimitResetsBucket(t *testing.T) {
	var r RateLimiter
	r.SetLimit(1024)
	// Simulate a huge stale deficit directly (without actually sleeping for it) by poking
	// the unexported bucket state; SetLimit must clear it so the next Wait doesn't stall.
	r.mu.Lock()
	r.tokens = -1_000_000
	r.mu.Unlock()
	r.SetLimit(1024)
	start := time.Now()
	if err := r.Wait(context.Background(), 1); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatal("SetLimit should reset accumulated deficit")
	}
}
