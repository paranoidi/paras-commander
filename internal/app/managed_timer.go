package app

import (
	"sync"
	"time"
)

// managedTimer is a thread-safe one-shot timer with stop-drain-reset semantics.
// The zero value is ready to use.
type managedTimer struct {
	mu sync.Mutex
	t  *time.Timer
}

// stopDrain stops the timer and drains the channel if it already fired.
// Must be called with mu held.
func (mt *managedTimer) stopDrain() {
	if mt.t == nil {
		return
	}
	if !mt.t.Stop() {
		select {
		case <-mt.t.C:
		default:
		}
	}
	mt.t = nil
}

// Clear cancels any pending timer.
func (mt *managedTimer) Clear() {
	mt.mu.Lock()
	mt.stopDrain()
	mt.mu.Unlock()
}

// Reset cancels any pending timer and schedules fn to run after delay.
// fn is called without any lock held.
func (mt *managedTimer) Reset(delay time.Duration, fn func()) {
	mt.mu.Lock()
	mt.stopDrain()
	mt.t = time.AfterFunc(delay, func() {
		mt.mu.Lock()
		mt.t = nil
		mt.mu.Unlock()
		fn()
	})
	mt.mu.Unlock()
}
