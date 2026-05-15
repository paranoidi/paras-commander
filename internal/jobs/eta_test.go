package jobs

import (
	"testing"
	"time"
)

func TestFormatETAUsesSmoothedRateWhenPresent(t *testing.T) {
	started := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := started.Add(10 * time.Second)
	// With blending toward cumulative average over elapsed wall time, smoothed EMA dominates early weight.
	// cum = 100 B/s vs smoothed 1000 B/s → blended rate ~850 B/s → remain 9000 / ~850 rounds to 11s.
	got := FormatETA(StatusRunning, started, now, 10000, 1000, 0, 0, 1000, 0)
	if got != "10s" {
		t.Fatalf("FormatETA = %q, want 10s", got)
	}
}

func TestFormatETAPicksMaxOfByteAndFileEstimates(t *testing.T) {
	started := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	elapsed := 30 * time.Second
	now := started.Add(elapsed)
	// Bytes-only path completes on ~13s scale; thousands of files left dominate wall clock via dual estimate.
	bytesOnly := FormatETA(StatusRunning, started, now, 1300000, 900000, 0, 0, 30000, 0)
	withFiles := FormatETA(StatusRunning, started, now, 1300000, 900000, 500000, 500, 30000, 500.0/30.0)
	dShort, err := time.ParseDuration(bytesOnly)
	if err != nil {
		t.Fatalf("ParseDuration bytesOnly %q: %v", bytesOnly, err)
	}
	dLong, err := time.ParseDuration(withFiles)
	if err != nil {
		t.Fatalf("ParseDuration withFiles %q: %v", withFiles, err)
	}
	if dLong <= dShort {
		t.Fatalf("expected files bottleneck to lengthen ETA: bytesOnly=%q (%v) withFiles=%q (%v)", bytesOnly, dShort, withFiles, dLong)
	}
}

func TestApplyProgressETAUpdatesSmoothedRate(t *testing.T) {
	j := &Job{}
	t0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ApplyProgressETA(j, 0, 0, t0)
	if !j.LastProgressSnapshotAt.Equal(t0) || j.LastProgressDoneBytes != 0 || j.LastProgressDoneFiles != 0 {
		t.Fatalf("first sample not initialized correctly")
	}
	t1 := t0.Add(100 * time.Millisecond)
	ApplyProgressETA(j, 1000, 0, t1)
	if j.ETABytesPerSec <= 0 {
		t.Fatal("expected positive ETABytesPerSec")
	}
	wantApprox := 10000.0 // 1000 bytes / 0.1s
	if j.ETABytesPerSec < wantApprox*0.9 || j.ETABytesPerSec > wantApprox*1.1 {
		t.Fatalf("ETABytesPerSec = %v, want ~%v", j.ETABytesPerSec, wantApprox)
	}
	if j.DisplaySpeedBPS <= 0 {
		t.Fatal("expected positive DisplaySpeedBPS")
	}
}

func TestApplyProgressETAFileOnlyStepsAdvanceSnapshot(t *testing.T) {
	j := &Job{}
	t0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ApplyProgressETA(j, 0, 0, t0)
	t1 := t0.Add(100 * time.Millisecond)
	ApplyProgressETA(j, 0, 3, t1)
	if j.LastProgressDoneFiles != 3 {
		t.Fatalf("LastProgressDoneFiles = %d want 3", j.LastProgressDoneFiles)
	}
	if !j.LastProgressSnapshotAt.Equal(t1) {
		t.Fatalf("snapshot time did not advance after metadata-only progress")
	}
	if j.ETAFilesPerSec <= 0 {
		t.Fatal("expected positive ETAFilesPerSec")
	}
	t2 := t1.Add(100 * time.Millisecond)
	ApplyProgressETA(j, 8000, 4, t2)
	if j.LastProgressDoneBytes != 8000 || j.LastProgressDoneFiles != 4 {
		t.Fatalf("expected merged snapshot bytes/files updated")
	}
	// Delta covers ~100ms, not time sunk into earlier dirs-only callbacks.
	wantApprox := 80000.0 // 8000 B / 0.1s
	if j.ETABytesPerSec < wantApprox*0.85 || j.ETABytesPerSec > wantApprox*1.25 {
		t.Fatalf("ETABytesPerSec = %v want ~%v (snapshot must not include pre-byte stall)", j.ETABytesPerSec, wantApprox)
	}
}

func TestApplyProgressETAIdleHeartbeatAdvancesSnapshot(t *testing.T) {
	j := &Job{}
	t0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ApplyProgressETA(j, 0, 0, t0)
	t1 := t0.Add(100 * time.Millisecond)
	ApplyProgressETA(j, 0, 0, t1)
	if !j.LastProgressSnapshotAt.Equal(t1) {
		t.Fatal("expected snapshot advance with zero deltas past min interval")
	}
	if j.LastProgressDoneFiles != 0 || j.LastProgressDoneBytes != 0 {
		t.Fatal("expected counters unchanged")
	}
}

func TestResetProgressETA(t *testing.T) {
	j := &Job{
		ETABytesPerSec:         123,
		ETAFilesPerSec:         77,
		DisplaySpeedBPS:        55,
		ThroughputStrip:        []float64{1, 2},
		LastProgressSnapshotAt: time.Now(),
		LastProgressDoneBytes:  99,
		LastProgressDoneFiles:  8,
	}
	ResetProgressETA(j)
	if j.ETABytesPerSec != 0 || j.ETAFilesPerSec != 0 || !j.LastProgressSnapshotAt.IsZero() || j.LastProgressDoneBytes != 0 || j.LastProgressDoneFiles != 0 {
		t.Fatal("ResetProgressETA did not clear ETA fields")
	}
	if j.DisplaySpeedBPS != 0 || len(j.ThroughputStrip) != 0 {
		t.Fatal("ResetProgressETA did not clear display throughput fields")
	}
}
