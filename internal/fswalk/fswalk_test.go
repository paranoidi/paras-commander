package fswalk_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/fswalk"
)

func TestDynSemAcquireRelease(t *testing.T) {
	t.Parallel()
	sem := fswalk.NewDynSem(2)
	ctx := context.Background()
	if err := sem.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sem.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("third acquire should block")
	case <-time.After(20 * time.Millisecond):
	}
	sem.Release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("waiter not released")
	}
	sem.Release()
	sem.Release()
}

func TestDynSemSetLimitShrink(t *testing.T) {
	t.Parallel()
	sem := fswalk.NewDynSem(2)
	ctx := context.Background()
	if err := sem.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sem.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	sem.SetLimit(1)
	blocked := make(chan struct{})
	go func() {
		_ = sem.Acquire(ctx)
		close(blocked)
	}()
	select {
	case <-blocked:
		t.Fatal("acquire should wait until limit rises or releases happen")
	case <-time.After(20 * time.Millisecond):
	}
	sem.Release()
	sem.Release()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("waiter not released after in-flight drop")
	}
	sem.Release()
}

func TestDynSemAcquireCancel(t *testing.T) {
	t.Parallel()
	sem := fswalk.NewDynSem(1)
	ctx, cancel := context.WithCancel(context.Background())
	if err := sem.Acquire(ctx); err != nil {
		t.Fatal(err)
	}
	waitCtx, waitCancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- sem.Acquire(waitCtx)
	}()
	time.Sleep(20 * time.Millisecond)
	waitCancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected cancel error")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked acquire did not cancel")
	}
	cancel()
}

func TestAdaptiveHillClimbFreeze(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := fswalk.NewAdaptive(ctx, fswalk.Params{
		InitialWorkers:  1,
		MaxWorkers:      4,
		AdaptIntervalMS: 50,
	})
	defer a.Stop()

	// Window 1 at limit 1: rate 10 -> bump to 2
	for range 10 {
		a.Bump()
	}
	time.Sleep(60 * time.Millisecond)
	if got := a.Workers(); got != 2 {
		t.Fatalf("after first window Workers = %d, want 2", got)
	}

	// Window 2 at limit 2: rate 20 -> bump to 3
	for range 20 {
		a.Bump()
	}
	time.Sleep(60 * time.Millisecond)
	if got := a.Workers(); got != 3 {
		t.Fatalf("after improving window Workers = %d, want 3", got)
	}

	// Window 3 at limit 3: rate 15 -> freeze at bestLimit 2
	for range 15 {
		a.Bump()
	}
	time.Sleep(60 * time.Millisecond)
	if got := a.Workers(); got != 2 {
		t.Fatalf("after worse window Workers = %d, want 2", got)
	}

	// Window 4: still locked at 2 despite higher count
	for range 30 {
		a.Bump()
	}
	time.Sleep(60 * time.Millisecond)
	if got := a.Workers(); got != 2 {
		t.Fatalf("locked Workers = %d, want 2", got)
	}
}

func TestAdaptiveEarlyFinishStaysInitial(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := fswalk.NewAdaptive(ctx, fswalk.Params{
		InitialWorkers:  1,
		MaxWorkers:      8,
		AdaptIntervalMS: 500,
	})
	defer a.Stop()
	a.Bump()
	if got := a.Workers(); got != 1 {
		t.Fatalf("Workers = %d, want 1 before first window", got)
	}
}

func TestAdaptiveMaxWorkersFreeze(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := fswalk.NewAdaptive(ctx, fswalk.Params{
		InitialWorkers:  2,
		MaxWorkers:      2,
		AdaptIntervalMS: 30,
	})
	defer a.Stop()

	for range 5 {
		a.Bump()
	}
	time.Sleep(40 * time.Millisecond)
	if got := a.Workers(); got != 2 {
		t.Fatalf("Workers = %d, want 2 at max", got)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 10 {
			a.Bump()
		}
		time.Sleep(40 * time.Millisecond)
	}()
	wg.Wait()
	if got := a.Workers(); got != 2 {
		t.Fatalf("Workers = %d, want 2 after max lock", got)
	}
}
