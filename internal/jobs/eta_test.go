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
	// Displayed speed is sampled on the throughput-column clock, not on progress events.
	if j.DisplaySpeedBPS != 0 {
		t.Fatalf("DisplaySpeedBPS = %v, want untouched by progress events", j.DisplaySpeedBPS)
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

func TestStepChangeRestartsETARatesAndClock(t *testing.T) {
	s := NewState()
	s.AddJob(&Job{ID: "scan-1", Type: TypeCopy, Status: StatusQueued})
	s.ApplyEvent(Event{Type: EventStarted, JobID: "scan-1", Status: StatusRunning})
	j := s.findJobUnlocked("scan-1")

	// Step 0: the walk climbs fast (1000 files/s) for ten minutes.
	walkStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	j.StartedAt = walkStart
	for i := 0; i <= 10; i++ {
		ApplyProgressETA(j, 0, i*1000, walkStart.Add(time.Duration(i)*time.Second))
	}
	j.DoneFiles = 600000
	if j.ETAFilesPerSec < 900 {
		t.Fatalf("walk rate = %v, want ~1000", j.ETAFilesPerSec)
	}
	if !j.StepStartedAt.IsZero() || !j.ETAStart().Equal(walkStart) {
		t.Fatalf("before a step change ETAStart = %v, want StartedAt", j.ETAStart())
	}

	// Hashing starts: totals announced, then DoneFiles restarts at 0 on step 2.
	s.ApplyEvent(Event{Type: EventPlanTotals, JobID: "scan-1", TotalFiles: 1000})
	s.ApplyEvent(Event{Type: EventProgress, JobID: "scan-1", Step: 2, DoneFiles: 0})
	if j.StepStartedAt.IsZero() || !j.ETAStart().Equal(j.StepStartedAt) {
		t.Fatalf("ETAStart = %v, want StepStartedAt %v", j.ETAStart(), j.StepStartedAt)
	}
	if j.ETAFilesPerSec != 0 || j.LastProgressSnapshotAt.IsZero() || j.LastProgressDoneFiles != 0 {
		t.Fatalf("step change left rates %v / baseline %v / %d",
			j.ETAFilesPerSec, j.LastProgressSnapshotAt, j.LastProgressDoneFiles)
	}

	// Hashing runs slowly (10 files/s). Move the step's clock to a fixed instant.
	hashStart := walkStart.Add(10 * time.Minute)
	j.StepStartedAt, j.LastProgressSnapshotAt = hashStart, hashStart
	for i := 1; i <= 10; i++ {
		ApplyProgressETA(j, 0, i*10, hashStart.Add(time.Duration(i)*time.Second))
	}
	j.DoneFiles = 100
	got := EstimateJobRemainingSecs(j, hashStart.Add(10*time.Second), 0, 0)
	if got < 80 || got > 100 {
		t.Fatalf("remaining = %v s, want ~90 (slow hash rate)", got)
	}
}

func TestJobWithoutStepChangeUsesStartedAt(t *testing.T) {
	s := NewState()
	s.AddJob(&Job{ID: "copy-1", Type: TypeCopy, Status: StatusQueued})
	s.ApplyEvent(Event{Type: EventStarted, JobID: "copy-1", Status: StatusRunning})
	s.ApplyEvent(Event{Type: EventProgress, JobID: "copy-1", DoneFiles: 5})
	j := s.findJobUnlocked("copy-1")
	if !j.StepStartedAt.IsZero() {
		t.Fatalf("StepStartedAt = %v, want zero without a step change", j.StepStartedAt)
	}

	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	j.StartedAt = start
	j.TotalFiles, j.DoneFiles = 1000, 100
	j.ETAFilesPerSec = 10
	if !j.ETAStart().Equal(start) {
		t.Fatalf("ETAStart = %v, want StartedAt", j.ETAStart())
	}
	got := EstimateJobRemainingSecs(j, start.Add(10*time.Second), 0, 0)
	if got < 80 || got > 100 {
		t.Fatalf("remaining = %v s, want ~90", got)
	}
}
