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
// Stop and the generation bump share one lock so a firing callback cannot pass its
// generation check in between.
func (d *Debouncer) Invalidate() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
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
		d.fire(gen, fn)
	})
}

func (d *Debouncer) fire(gen uint64, fn func()) {
	d.mu.Lock()
	if d.gen.Load() != gen {
		d.mu.Unlock()
		return
	}
	d.timer = nil
	d.mu.Unlock()
	if d.gen.Load() != gen {
		return
	}
	fn()
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

// Hold gates follow-up work while a key is held: Arm sets Held and (re)starts the release
// timer; on fire Held clears, then fn runs (typically posting a flush wake). Clear releases now.
type Hold struct {
	d    Debouncer
	held atomic.Bool
}

// Arm marks the hold active and (re)starts the release timer.
func (h *Hold) Arm(delay time.Duration, fn func()) {
	h.held.Store(true)
	h.d.Arm(delay, func() {
		h.held.Store(false)
		fn()
	})
}

// Clear cancels a pending release and drops the hold immediately.
func (h *Hold) Clear() {
	h.d.Invalidate()
	h.held.Store(false)
}

// Held reports whether the hold is active.
func (h *Hold) Held() bool { return h.held.Load() }
