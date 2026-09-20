package sched

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncerArmRunsAfterDelay(t *testing.T) {
	var d Debouncer
	var ran atomic.Bool
	d.Arm(20*time.Millisecond, func() { ran.Store(true) })
	time.Sleep(50 * time.Millisecond)
	if !ran.Load() {
		t.Fatal("expected callback to run")
	}
}

func TestDebouncerInvalidateSuppressesCallback(t *testing.T) {
	var d Debouncer
	var ran atomic.Bool
	d.Arm(30*time.Millisecond, func() { ran.Store(true) })
	d.Invalidate()
	time.Sleep(60 * time.Millisecond)
	if ran.Load() {
		t.Fatal("invalidated callback should not run")
	}
}

func TestDebouncerArmBumpsGeneration(t *testing.T) {
	var d Debouncer
	before := d.Generation()
	d.Arm(time.Hour, func() {})
	if got := d.Generation(); got != before+1 {
		t.Fatalf("Generation after Arm = %d want %d", got, before+1)
	}
	d.Invalidate()
	if got := d.Generation(); got != before+2 {
		t.Fatalf("Generation after Invalidate = %d want %d", got, before+2)
	}
}

func TestDebouncerStopDoesNotBumpGeneration(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	d.Arm(time.Hour, func() {})
	before := d.Generation()
	d.Stop()
	if got := d.Generation(); got != before {
		t.Fatalf("Generation after Stop = %d want %d", got, before)
	}
	if d.Armed() {
		t.Fatal("Stop should leave the debouncer unarmed")
	}
}

func TestDebouncerStopDoesNotSuppressFiringCallback(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	var ran atomic.Bool
	d.Arm(time.Hour, func() { ran.Store(true) })
	gen := d.Generation()
	d.Stop()
	d.fire(gen, func() { ran.Store(true) })
	if !ran.Load() {
		t.Fatal("Stop should not suppress an already-firing callback")
	}
}

func TestDebouncerInvalidateSuppressesFiringCallback(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	var ran atomic.Bool
	d.Arm(time.Hour, func() { ran.Store(true) })
	gen := d.Generation()
	d.Invalidate()
	d.fire(gen, func() { ran.Store(true) })
	if ran.Load() {
		t.Fatal("Invalidate should suppress a callback at the firing boundary")
	}
	if d.Armed() {
		t.Fatal("Invalidate should leave the debouncer unarmed")
	}
}

func TestDebouncerOldCallbackDoesNotClearReplacementTimer(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	var oldRan atomic.Bool
	d.Arm(time.Hour, func() { oldRan.Store(true) })
	oldGen := d.Generation()

	d.Arm(time.Hour, func() {})
	if !d.Armed() {
		t.Fatal("replacement Arm should be armed")
	}

	d.fire(oldGen, func() { oldRan.Store(true) })
	if oldRan.Load() {
		t.Fatal("stale callback should not run after replacement Arm")
	}
	if !d.Armed() {
		t.Fatal("stale AfterFunc cleared the replacement timer")
	}
}

func TestDebouncerReplacementArmWhileOldCallbackHoldsMutex(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	d.Arm(time.Hour, func() {})
	oldGen := d.Generation()

	d.mu.Lock()
	startedFire := make(chan struct{})
	startedArm := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		close(startedFire)
		d.fire(oldGen, func() {})
	}()
	go func() {
		defer wg.Done()
		close(startedArm)
		d.Arm(time.Hour, func() {})
	}()
	<-startedFire
	<-startedArm
	for range 50 {
		runtime.Gosched()
	}
	d.mu.Unlock()
	wg.Wait()

	if !d.Armed() {
		t.Fatal("old AfterFunc cleared the replacement timer")
	}
}

func TestDebouncerCallbackMayReenter(t *testing.T) {
	var d Debouncer
	t.Cleanup(d.Stop)
	done := make(chan struct{})
	d.Arm(time.Millisecond, func() {
		if d.Armed() {
			t.Error("callback should not see its own timer still armed")
		}
		d.Arm(time.Hour, func() {})
		if !d.Armed() {
			t.Error("Arm from callback should leave a pending timer")
		}
		d.Stop()
		if d.Armed() {
			t.Error("Stop from callback should clear the pending timer")
		}
		d.Arm(time.Hour, func() {})
		d.Invalidate()
		if d.Armed() {
			t.Error("Invalidate from callback should clear the pending timer")
		}
		close(done)
	})
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for re-entrant callback")
	}
}

func TestDebouncerConcurrentArmStopInvalidate(t *testing.T) {
	d := new(Debouncer)
	n := new(atomic.Int64)
	var wg sync.WaitGroup
	const goroutines = 8
	const iters = 250
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for i := range iters {
				switch i % 5 {
				case 0:
					d.Arm(time.Microsecond, func() { n.Add(1) })
				case 1:
					d.Arm(time.Hour, func() { n.Add(1) })
				case 2:
					d.Stop()
				case 3:
					d.Invalidate()
				default:
					_ = d.Armed()
					_ = d.Generation()
				}
			}
		}()
	}
	wg.Wait()
	d.Invalidate()
	time.Sleep(200 * time.Millisecond)
}
