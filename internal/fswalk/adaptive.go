package fswalk

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Adaptive hill-climbs walk concurrency until throughput stops improving, then freezes.
type Adaptive struct {
	sem       *DynSem
	max       int
	mu        sync.Mutex
	bestRate  int64
	bestLimit int
	first     bool
	locked    bool

	count atomic.Int64

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewAdaptive starts at p.InitialWorkers and adapts every p.AdaptIntervalMS until ctx ends or Stop is called.
func NewAdaptive(ctx context.Context, p Params) *Adaptive {
	initial := p.InitialWorkers
	if initial < 1 {
		initial = 1
	}
	max := p.MaxWorkers
	if max < initial {
		max = initial
	}
	interval := time.Duration(p.AdaptIntervalMS) * time.Millisecond
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	a := &Adaptive{
		sem:       NewDynSem(initial),
		max:       max,
		bestLimit: initial,
		first:     true,
		stopCh:    make(chan struct{}),
	}
	go a.loop(ctx, interval)
	return a
}

// Sem returns the walk semaphore whose limit Adaptive adjusts.
func (a *Adaptive) Sem() *DynSem { return a.sem }

// Bump records one throughput unit (indexed entry or completed ReadDir, per caller).
func (a *Adaptive) Bump() { a.count.Add(1) }

// Workers returns the current concurrency limit.
func (a *Adaptive) Workers() int { return a.sem.Limit() }

// Stop ends the adaptation loop.
func (a *Adaptive) Stop() {
	a.stopOnce.Do(func() { close(a.stopCh) })
}

func (a *Adaptive) loop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.tick()
		}
	}
}

func (a *Adaptive) tick() {
	rate := a.count.Swap(0)

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.locked {
		return
	}

	if a.first {
		a.first = false
		a.bestRate = rate
		a.bestLimit = a.sem.Limit()
		if a.sem.Limit() < a.max {
			next := a.sem.Limit() + 1
			a.sem.SetLimit(next)
		} else {
			a.locked = true
		}
		return
	}

	if rate > a.bestRate {
		a.bestRate = rate
		a.bestLimit = a.sem.Limit()
		if a.sem.Limit() < a.max {
			a.sem.SetLimit(a.sem.Limit() + 1)
		} else {
			a.locked = true
		}
		return
	}

	a.sem.SetLimit(a.bestLimit)
	a.locked = true
}
