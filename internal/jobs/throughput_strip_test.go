package jobs

import (
	"math"
	"testing"
	"time"
)

func TestSampleThroughputColumns_initializesOpenColumn(t *testing.T) {
	t.Parallel()
	var j Job
	now := time.Unix(1000, 0)
	col := time.Second
	win := 30 * time.Second
	if SampleThroughputColumns(&j, now, 1000, col, win, true) {
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

func TestSampleThroughputColumns_oneColumnPerElapsedInterval(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	if SampleThroughputColumns(&j, time.Unix(0, 0), 0, col, win, true) {
		t.Fatal("first call should not append")
	}
	// Sub-column calls close nothing: the grid only advances on full columns.
	if SampleThroughputColumns(&j, time.Unix(0, int64(500*time.Millisecond)), 5, col, win, true) {
		t.Fatal("half a column should not append")
	}
	if !SampleThroughputColumns(&j, time.Unix(1, 0), 10, col, win, true) {
		t.Fatal("second call should append one column")
	}
	if len(j.ThroughputStrip) != 1 {
		t.Fatalf("len=%d want 1: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	// First sample seeds the EMA with the instantaneous rate.
	if math.Abs(j.ThroughputStrip[0]-10) > 1e-9 {
		t.Fatalf("want 10 B/s got %v", j.ThroughputStrip[0])
	}
	if !SampleThroughputColumns(&j, time.Unix(2, 0), 20, col, win, true) {
		t.Fatal("third call should append one column")
	}
	if len(j.ThroughputStrip) != 2 {
		t.Fatalf("len=%d want 2: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
}

// A late tick must catch the grid up so the chart's time axis stays honest.
func TestSampleThroughputColumns_catchesUpAfterStall(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	SampleThroughputColumns(&j, time.Unix(0, 0), 0, col, win, true)
	if !SampleThroughputColumns(&j, time.Unix(4, 0), 40, col, win, true) {
		t.Fatal("expected appends after gap")
	}
	if len(j.ThroughputStrip) != 4 {
		t.Fatalf("len=%d want 4 columns across the 4s gap: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	// 40 bytes over 4 columns is a steady 10 B/s, so every replayed column reads 10.
	for i, v := range j.ThroughputStrip {
		if math.Abs(v-10) > 1e-9 {
			t.Fatalf("column %d = %v want 10: %v", i, v, j.ThroughputStrip)
		}
	}
}

// Idle columns must still scroll the chart and decay the speed rather than being skipped.
func TestSampleThroughputColumns_idleColumnDecays(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	SampleThroughputColumns(&j, time.Unix(0, 0), 0, col, win, true)
	// Nothing moved yet: no leading zero columns, nothing to plot.
	if SampleThroughputColumns(&j, time.Unix(1, 0), 0, col, win, true) {
		t.Fatal("idle column before any bytes should not append")
	}
	if len(j.ThroughputStrip) != 0 {
		t.Fatalf("strip = %v want empty before first bytes", j.ThroughputStrip)
	}
	if !SampleThroughputColumns(&j, time.Unix(2, 0), 100, col, win, true) {
		t.Fatal("expected append after bytes moved")
	}
	peak := j.DisplaySpeedBPS
	if peak <= 0 {
		t.Fatalf("DisplaySpeedBPS = %v want positive", peak)
	}
	// A stall now appends columns (chart keeps scrolling) with a decaying value.
	if !SampleThroughputColumns(&j, time.Unix(5, 0), 100, col, win, true) {
		t.Fatal("stalled columns should still append")
	}
	if len(j.ThroughputStrip) != 4 {
		t.Fatalf("len=%d want 4: %v", len(j.ThroughputStrip), j.ThroughputStrip)
	}
	if j.DisplaySpeedBPS >= peak {
		t.Fatalf("DisplaySpeedBPS = %v want decay below %v during stall", j.DisplaySpeedBPS, peak)
	}
}

// A throttled worker emitting one large progress batch must not spike a single column.
func TestSampleThroughputColumns_burstIsSpreadNotSpiked(t *testing.T) {
	t.Parallel()
	col := 400 * time.Millisecond
	win := 10 * time.Second

	// Steady 1000 B/s delivered smoothly, one progress update per column.
	var smooth Job
	SampleThroughputColumns(&smooth, time.Unix(0, 0), 0, col, win, true)
	for i := 1; i <= 10; i++ {
		at := time.Unix(0, 0).Add(time.Duration(i) * col)
		SampleThroughputColumns(&smooth, at, int64(float64(i)*col.Seconds()*1000), col, win, true)
	}

	// Same 1000 B/s, but DoneBytes only lands every 5th column (throttled emit).
	var bursty Job
	SampleThroughputColumns(&bursty, time.Unix(0, 0), 0, col, win, true)
	for i := 1; i <= 10; i++ {
		at := time.Unix(0, 0).Add(time.Duration(i) * col)
		bytes := int64(float64(max(i/5*5, 1)) * col.Seconds() * 1000)
		SampleThroughputColumns(&bursty, at, bytes, col, win, true)
	}

	if len(smooth.ThroughputStrip) != len(bursty.ThroughputStrip) {
		t.Fatalf("column counts differ: %d vs %d", len(smooth.ThroughputStrip), len(bursty.ThroughputStrip))
	}
	max := 0.0
	for _, v := range bursty.ThroughputStrip {
		if v > max {
			max = v
		}
	}
	// Raw per-column deltas would show a 5000 B/s spike; smoothing must keep it near the real rate.
	if max > 1500 {
		t.Fatalf("bursty peak %v spikes above the real 1000 B/s: %v", max, bursty.ThroughputStrip)
	}
}

func TestSampleThroughputColumns_stripDisabledStillTracksSpeed(t *testing.T) {
	t.Parallel()
	var j Job
	col := time.Second
	win := 10 * time.Second
	SampleThroughputColumns(&j, time.Unix(0, 0), 0, col, win, false)
	if SampleThroughputColumns(&j, time.Unix(1, 0), 10, col, win, false) {
		t.Fatal("strip must not grow when recording is off")
	}
	if len(j.ThroughputStrip) != 0 {
		t.Fatalf("strip = %v want empty", j.ThroughputStrip)
	}
	if math.Abs(j.DisplaySpeedBPS-10) > 1e-9 {
		t.Fatalf("DisplaySpeedBPS = %v want 10", j.DisplaySpeedBPS)
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
