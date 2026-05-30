package sched

import (
	"sync"
	"sync/atomic"
	"time"
)

// Debouncer coalesces delayed callbacks. Each Arm increments an internal generation;
// stale timers exit without running fn. Invalidate stops any pending timer and bumps
// generation so in-flight callbacks are ignored.
type Debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	gen   atomic.Uint64
}

// Stop cancels a pending timer without bumping generation.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

// Invalidate stops a pending timer and bumps generation so scheduled callbacks are ignored.
func (d *Debouncer) Invalidate() {
	d.Stop()
	d.gen.Add(1)
}

// Armed reports whether a timer is currently scheduled.
func (d *Debouncer) Armed() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.timer != nil
}

// Generation returns the current invalidation generation (for tests).
func (d *Debouncer) Generation() uint64 {
	return d.gen.Load()
}

// Arm schedules fn after delay. Any previous timer is stopped and generation is bumped first.
func (d *Debouncer) Arm(delay time.Duration, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
	gen := d.gen.Add(1)
	d.timer = time.AfterFunc(delay, func() {
		d.mu.Lock()
		d.timer = nil
		d.mu.Unlock()
		if d.gen.Load() != gen {
			return
		}
		fn()
	})
}

func (d *Debouncer) stopLocked() {
	if d.timer == nil {
		return
	}
	if !d.timer.Stop() {
		select {
		case <-d.timer.C:
		default:
		}
	}
	d.timer = nil
}
