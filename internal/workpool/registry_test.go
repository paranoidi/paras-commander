package workpool

import (
	"context"
	"testing"

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
