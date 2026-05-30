package sched

import (
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
