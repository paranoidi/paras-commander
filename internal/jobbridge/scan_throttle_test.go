package jobbridge

import (
	"testing"
	"time"
)

// TestAdaptiveThrottleGrowsWhenPausingHelps drives recordProbe directly with an injected
// before/after throughput pair (no real time.Sleep) simulating a probe where pausing the
// counting walk measurably raised throughput (30% jump, above the 20% significance threshold).
// dutyPauseMS must grow from zero.
func TestAdaptiveThrottleGrowsWhenPausingHelps(t *testing.T) {
	var th adaptiveThrottle
	now := time.Unix(0, 0)

	th.recordProbe(now, 10_000_000, 13_000_000) // +30%
	if th.dutyPauseMS <= 0 {
		t.Fatalf("expected dutyPauseMS to grow from a measurable benefit, got %v", th.dutyPauseMS)
	}
	first := th.dutyPauseMS

	// A second consecutive benefit should grow it further (until the cap).
	now = now.Add(5 * time.Second)
	th.recordProbe(now, 10_000_000, 13_000_000)
	if th.dutyPauseMS <= first {
		t.Fatalf("expected dutyPauseMS to keep growing on repeated benefit: first=%v after=%v", first, th.dutyPauseMS)
	}
}

// TestAdaptiveThrottleDecaysWhenNoBenefit simulates a probe where pausing made no measurable
// difference, asserting dutyPauseMS decays toward zero from a prior nonzero value.
func TestAdaptiveThrottleDecaysWhenNoBenefit(t *testing.T) {
	th := adaptiveThrottle{dutyPauseMS: 100}
	now := time.Unix(0, 0)

	th.recordProbe(now, 10_000_000, 10_050_000) // ~0.5% change, well under threshold
	if th.dutyPauseMS >= 100 {
		t.Fatalf("expected dutyPauseMS to decay from 100 on no benefit, got %v", th.dutyPauseMS)
	}
	if th.dutyPauseMS < 0 {
		t.Fatalf("dutyPauseMS must not go negative, got %v", th.dutyPauseMS)
	}

	// Repeated no-benefit probes should keep decaying it toward (but not below) zero.
	prev := th.dutyPauseMS
	for i := 0; i < 20; i++ {
		now = now.Add(5 * time.Second)
		th.recordProbe(now, 10_000_000, 10_050_000)
		if th.dutyPauseMS > prev {
			t.Fatalf("dutyPauseMS increased during decay: prev=%v now=%v", prev, th.dutyPauseMS)
		}
		prev = th.dutyPauseMS
	}
	if prev >= 1 {
		t.Fatalf("expected dutyPauseMS to have decayed near zero after repeated no-benefit probes, got %v", prev)
	}
}

// TestAdaptiveThrottleShouldProbeCadence verifies the probe cadence gate: first call always
// probes, a later call inside the interval does not, and one at/after the interval does.
func TestAdaptiveThrottleShouldProbeCadence(t *testing.T) {
	var th adaptiveThrottle
	start := time.Unix(0, 0)

	if !th.shouldProbe(start) {
		t.Fatal("expected first call to always probe")
	}
	th.recordProbe(start, 1, 1)

	if th.shouldProbe(start.Add(time.Second)) {
		t.Fatal("expected no probe well inside the interval")
	}
	if !th.shouldProbe(start.Add(4 * time.Second)) {
		t.Fatal("expected a probe once the interval has elapsed")
	}
}

// TestAdaptiveThrottleCapsAtMax ensures repeated benefit probes clamp at the configured maximum
// duty-cycle pause rather than growing unbounded.
func TestAdaptiveThrottleCapsAtMax(t *testing.T) {
	th := adaptiveThrottle{dutyPauseMS: 490}
	now := time.Unix(0, 0)
	for i := 0; i < 5; i++ {
		now = now.Add(5 * time.Second)
		th.recordProbe(now, 10_000_000, 13_000_000)
	}
	if th.dutyPauseMS > 500 {
		t.Fatalf("expected dutyPauseMS to clamp at 500, got %v", th.dutyPauseMS)
	}
}
