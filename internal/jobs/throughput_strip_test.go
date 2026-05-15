package jobs

import (
	"math"
	"testing"
	"time"
)

func TestAdvanceJobThroughputStrip_initializesOpenBin(t *testing.T) {
	t.Parallel()
	var j Job
	now := time.Unix(1000, 0)
	bin := time.Second
	win := 30 * time.Second
	AdvanceJobThroughputStrip(&j, now, 1000, bin, win)
	if len(j.ThroughputStrip) != 0 {
		t.Fatalf("strip %v want empty", j.ThroughputStrip)
	}
	wantOpen := time.Unix(1000, 0).UnixNano()
	if j.ThroughputStripOpenBin != wantOpen {
		t.Fatalf("open bin = %d want %d", j.ThroughputStripOpenBin, wantOpen)
	}
	if !j.throughputStripOpenSet {
		t.Fatal("expected open bin anchored")
	}
	if j.ThroughputStripDoneAtOpen != 1000 {
		t.Fatalf("done at open = %d want 1000", j.ThroughputStripDoneAtOpen)
	}
}

func TestAdvanceJobThroughputStrip_closesBinsEvenSpread(t *testing.T) {
	t.Parallel()
	var j Job
	bin := time.Second
	win := 10 * time.Second
	AdvanceJobThroughputStrip(&j, time.Unix(0, 0), 0, bin, win)
	AdvanceJobThroughputStrip(&j, time.Unix(3, 0), 30, bin, win)
	if len(j.ThroughputStrip) != 3 {
		t.Fatalf("len=%d want 3: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	for _, v := range j.ThroughputStrip {
		if math.Abs(v-10) > 1e-9 {
			t.Fatalf("want 10 B/s each got %v", j.ThroughputStrip)
		}
	}
}

func TestThroughputChartColumnBuckets_padAndTrim(t *testing.T) {
	t.Parallel()
	got := ThroughputChartColumnBuckets([]float64{1, 2, 3}, 5)
	want := []float64{0, 0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len %d", len(got))
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
