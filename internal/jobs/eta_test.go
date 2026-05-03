package jobs

import (
	"testing"
	"time"
)

func TestFormatETAUsesSmoothedRateWhenPresent(t *testing.T) {
	started := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	now := started.Add(10 * time.Second)
	// Lifetime avg would be 100 B/s; smoothed says 1000 B/s → ETA should match smoothed.
	got := FormatETA(StatusRunning, started, now, 10000, 1000, 0, 0, 1000)
	if got != "9s" {
		t.Fatalf("FormatETA = %q, want 9s", got)
	}
}

func TestApplyProgressETAUpdatesSmoothedRate(t *testing.T) {
	j := &Job{}
	t0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	ApplyProgressETA(j, 0, t0)
	if !j.LastProgressSnapshotAt.Equal(t0) || j.LastProgressDoneBytes != 0 {
		t.Fatalf("first sample not initialized correctly")
	}
	t1 := t0.Add(100 * time.Millisecond)
	ApplyProgressETA(j, 1000, t1)
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
	if len(j.ThroughputSamples) != 1 || j.ThroughputSamples[0].BPS != wantApprox {
		t.Fatalf("ThroughputSamples = %v, want one sample BPS %v", j.ThroughputSamples, wantApprox)
	}
}

func TestResetProgressETA(t *testing.T) {
	j := &Job{
		ETABytesPerSec:         123,
		DisplaySpeedBPS:        55,
		ThroughputSamples:      []ThroughputSample{{BPS: 1}, {BPS: 2}},
		LastProgressSnapshotAt: time.Now(),
		LastProgressDoneBytes:  99,
	}
	ResetProgressETA(j)
	if j.ETABytesPerSec != 0 || !j.LastProgressSnapshotAt.IsZero() || j.LastProgressDoneBytes != 0 {
		t.Fatal("ResetProgressETA did not clear fields")
	}
	if j.DisplaySpeedBPS != 0 || len(j.ThroughputSamples) != 0 {
		t.Fatal("ResetProgressETA did not clear display throughput fields")
	}
}
