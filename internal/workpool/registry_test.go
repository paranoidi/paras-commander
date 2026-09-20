package workpool

import (
	"context"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pools"
)

func TestRegistryLookupAndAcquire(t *testing.T) {
	reg := NewRegistry([]pools.Def{
		{Name: "a", MaxParallel: 2},
		{Name: "b", MaxParallel: 1},
	})
	if _, ok := reg.Pool("a"); !ok {
		t.Fatal("pool a missing")
	}
	release, err := reg.Acquire(context.Background(), "a")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	release()
}

func TestRegistryUnknownPool(t *testing.T) {
	reg := NewRegistry(nil)
	_, err := reg.Acquire(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for unknown pool")
	}
}

func TestRegistryEmptyName(t *testing.T) {
	reg := NewRegistry([]pools.Def{{Name: "x", MaxParallel: 1}})
	_, err := reg.Acquire(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty pool name")
	}
}

func TestRegistryAcquireCancelWhileWaiting(t *testing.T) {
	reg := NewRegistry([]pools.Def{{Name: "one", MaxParallel: 1}})
	release, err := reg.Acquire(context.Background(), "one")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := reg.Acquire(ctx, "one")
		errCh <- err
	}()

	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected context error while waiting for a slot")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire did not return after cancel")
	}
}
