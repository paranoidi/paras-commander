package jobs

import (
	"math"
	"testing"
	"time"
)

func TestCloseOneThroughputColumn_initializesOpenColumn(t *testing.T) {
	t.Parallel()
	var j Job
	now := time.Unix(1000, 0)
	col := time.Second
	win := 30 * time.Second
	if CloseOneThroughputColumn(&j, now, 1000, col, win) {
		t.Fatal("first call should not append")
	}
	if len(j.ThroughputStrip) != 0 {
		t.Fatalf("strip %v want empty", j.ThroughputStrip)
	}
	if j.ThroughputStripOpenBin != now.UnixNano() {
		t.Fatalf("open = %d want %d", j.ThroughputStripOpenBin, now.UnixNano())
	}
	if !j.throughputStripOpenSet {
		t.Fatal("expected open column anchored")
	}
	if j.ThroughputStripDoneAtOpen != 1000 {
		t.Fatalf("done at open = %d want 1000", j.ThroughputStripDoneAtOpen)
	}
}

func TestCloseOneThroughputColumn_onePerCall(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	if CloseOneThroughputColumn(&j, time.Unix(0, 0), 0, col, win) {
		t.Fatal("first call should not append")
	}
	if !CloseOneThroughputColumn(&j, time.Unix(1, 0), 10, col, win) {
		t.Fatal("second call should append one column")
	}
	if len(j.ThroughputStrip) != 1 {
		t.Fatalf("len=%d want 1: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	if math.Abs(j.ThroughputStrip[0]-10) > 1e-9 {
		t.Fatalf("want 10 B/s got %v", j.ThroughputStrip[0])
	}
	if !CloseOneThroughputColumn(&j, time.Unix(2, 0), 20, col, win) {
		t.Fatal("third call should append one column")
	}
	if len(j.ThroughputStrip) != 2 {
		t.Fatalf("len=%d want 2: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	// Large gap: still only one column per call.
	if !CloseOneThroughputColumn(&j, time.Unix(5, 0), 50, col, win) {
		t.Fatal("expected one append after gap")
	}
	if len(j.ThroughputStrip) != 3 {
		t.Fatalf("len=%d want 3 after single close across gap: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
}

func TestCloseOneThroughputColumn_skipsIdleColumn(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	CloseOneThroughputColumn(&j, time.Unix(0, 0), 0, col, win)
	if CloseOneThroughputColumn(&j, time.Unix(1, 0), 0, col, win) {
		t.Fatal("idle column should not append")
	}
	if len(j.ThroughputStrip) != 0 {
		t.Fatalf("strip = %v want empty after idle", j.ThroughputStrip)
	}
	if !CloseOneThroughputColumn(&j, time.Unix(2, 0), 100, col, win) {
		t.Fatal("expected append after bytes moved")
	}
	if len(j.ThroughputStrip) != 1 {
		t.Fatalf("len=%d want 1", len(j.ThroughputStrip))
	}
}

func TestThroughputChartColumnBuckets_noFakeLeadingZeros(t *testing.T) {
	t.Parallel()
	got := ThroughputChartColumnBuckets([]float64{1, 2, 3}, 5)
	want := []float64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("i=%d got %v want %v", i, got, want)
		}
	}
	long := make([]float64, 10)
	for i := range long {
		long[i] = float64(i)
	}
	got2 := ThroughputChartColumnBuckets(long, 3)
	if got2[0] != 7 || got2[1] != 8 || got2[2] != 9 {
		t.Fatalf("trim: %v", got2)
	}
}
