package workpool

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPoolAcquireRelease(t *testing.T) {
	p := New(1)
	ctx := context.Background()
	if err := p.Acquire(ctx); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	acquired := make(chan struct{})
	go func() {
		if err := p.Acquire(ctx); err != nil {
			t.Errorf("second Acquire: %v", err)
			return
		}
		close(acquired)
	}()
	select {
	case <-acquired:
		t.Fatal("second Acquire should block while first slot held")
	case <-time.After(50 * time.Millisecond):
	}
	p.Release()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second Acquire did not proceed after Release")
	}
	p.Release()
}

func TestPoolAcquireContextCancel(t *testing.T) {
	p := New(1)
	if err := p.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Acquire(ctx); err == nil {
		t.Fatal("expected context error while waiting for slot")
	}
}

func TestPoolParallelLimit(t *testing.T) {
	const n = 3
	p := New(n)
	ctx := context.Background()
	var running int
	var mu sync.Mutex
	var peak int
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.Acquire(ctx); err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			defer p.Release()
			mu.Lock()
			running++
			if running > peak {
				peak = running
			}
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			running--
			mu.Unlock()
		}()
	}
	wg.Wait()
	if peak > n {
		t.Fatalf("peak concurrent = %d, want <= %d", peak, n)
	}
}
