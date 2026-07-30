package fswalk

import (
	"context"
	"sync/atomic"
)

// DynSem is a counting semaphore with a dynamically adjustable limit.
type DynSem struct {
	limit    atomic.Int32
	inFlight atomic.Int32
	wake     chan struct{}
}

// NewDynSem creates a semaphore with the given limit (minimum 1).
func NewDynSem(limit int) *DynSem {
	if limit < 1 {
		limit = 1
	}
	d := &DynSem{wake: make(chan struct{}, 1)}
	d.limit.Store(int32(limit))
	return d
}

// Limit returns the current concurrency limit.
func (d *DynSem) Limit() int {
	return int(d.limit.Load())
}

// SetLimit stores a new limit (minimum 1) and wakes waiters.
func (d *DynSem) SetLimit(n int) {
	if n < 1 {
		n = 1
	}
	d.limit.Store(int32(n))
	d.poke()
}

// Acquire blocks until in-flight work is below the limit or ctx is cancelled.
func (d *DynSem) Acquire(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		lim := d.limit.Load()
		infl := d.inFlight.Load()
		if infl < lim {
			if d.inFlight.CompareAndSwap(infl, infl+1) {
				if d.inFlight.Load() <= d.limit.Load() {
					return nil
				}
				d.inFlight.Add(-1)
				d.poke()
			}
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.wake:
		}
	}
}

// Release decrements in-flight work and wakes one waiter.
func (d *DynSem) Release() {
	d.inFlight.Add(-1)
	d.poke()
}

func (d *DynSem) poke() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}
