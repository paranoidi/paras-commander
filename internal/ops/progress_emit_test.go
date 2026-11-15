package ops

import (
	"testing"
	"time"
)

func countingState(interval time.Duration) (*copyRunState, *int) {
	n := 0
	return &copyRunState{
		th:       ProgressEmitThrottle{MinInterval: interval},
		progress: func(string, string, int, int64) { n++ },
	}, &n
}

func TestEmitProgressTimeOnly(t *testing.T) {
	s, n := countingState(time.Hour)
	for i := 0; i < 50; i++ {
		s.doneBytes += 1 << 20
		s.bytesSinceEmit += 1 << 20
		s.emitProgress("a", "b")
	}
	if *n != 1 {
		t.Fatalf("emits = %d, want exactly the first sample", *n)
	}

	s, n = countingState(time.Nanosecond)
	for i := 0; i < 3; i++ {
		s.bytesSinceEmit += 10
		s.emitProgress("a", "b")
		time.Sleep(time.Millisecond)
	}
	if *n != 3 {
		t.Fatalf("emits = %d, want 3 with a tiny interval", *n)
	}
}

func TestEmitProgressReportsFileWithoutBytes(t *testing.T) {
	s, n := countingState(20 * time.Millisecond)
	s.bytesSinceEmit = 1
	s.emitProgress("a", "b") // first sample
	s.doneFiles++
	s.filesDirty = true // empty file: no new bytes
	s.emitProgress("a", "b")
	if *n != 1 {
		t.Fatalf("emits = %d before the interval elapsed, want 1", *n)
	}
	time.Sleep(30 * time.Millisecond)
	s.emitProgress("a", "b")
	if *n != 2 || s.filesDirty {
		t.Fatalf("emits = %d dirty = %v, want the pending file reported once", *n, s.filesDirty)
	}
	s.emitProgress("a", "b")
	if *n != 2 {
		t.Fatalf("emits = %d, nothing pending should not emit", *n)
	}
}

func TestTransferMeterTimeOnly(t *testing.T) {
	n := 0
	m := transferMeter{th: ProgressEmitThrottle{MinInterval: time.Hour}, fn: func(string, int64) { n++ }}
	for i := 0; i < 20; i++ {
		m.add("x", 1<<20)
	}
	if n != 1 {
		t.Fatalf("emits = %d, want 1", n)
	}
}
