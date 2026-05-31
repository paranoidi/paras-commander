package workpool

import "context"

// Pool limits how many units of work may run concurrently.
type Pool struct {
	sem chan struct{}
}

// New creates a pool that allows at most maxParallel concurrent acquisitions.
// Values below 1 are treated as 1.
func New(maxParallel int) *Pool {
	if maxParallel < 1 {
		maxParallel = 1
	}
	return &Pool{sem: make(chan struct{}, maxParallel)}
}

// Acquire blocks until a slot is available or ctx is canceled.
func (p *Pool) Acquire(ctx context.Context) error {
	select {
	case p.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns one slot to the pool.
func (p *Pool) Release() {
	<-p.sem
}
