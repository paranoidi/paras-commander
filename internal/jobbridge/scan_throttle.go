package jobbridge

import (
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
)

// adaptiveThrottle implements the pre-scan counting walk's duty-cycle contention probe (see
// docs/agents/jobs.md): periodically pause the counting walk for a short window and compare the
// job's measured transfer throughput just before vs. during the pause, growing the duty-cycle
// pause only when pausing measurably helped and decaying it otherwise, re-probing on a fixed
// cadence since conditions change through a job's lifetime. This self-corrects for any storage
// type — on SSD/NVMe/network storage pausing never shows a throughput win, so the duty-cycle
// pause stays at (or decays back to) zero. shouldProbe/recordProbe are pure functions of
// (now, before, after) so the state machine is testable without real timing;
// newAdaptiveThrottleYield wires it to time.Now/time.Sleep.
type adaptiveThrottle struct {
	dutyPauseMS float64
	lastProbeAt time.Time
}

// shouldProbe reports whether a probe is due at now (the first call always probes).
func (t *adaptiveThrottle) shouldProbe(now time.Time) bool {
	interval := time.Duration(config.DefaultScanThrottleProbeIntervalSec) * time.Second
	return t.lastProbeAt.IsZero() || now.Sub(t.lastProbeAt) >= interval
}

// recordProbe updates dutyPauseMS from one completed probe's before/after throughput samples
// and marks now as the last probe time.
func (t *adaptiveThrottle) recordProbe(now time.Time, before, after float64) {
	t.lastProbeAt = now
	if after > before*(1+config.DefaultScanThrottleSignificantDropThreshold) {
		if t.dutyPauseMS <= 0 {
			t.dutyPauseMS = config.DefaultScanThrottleDutyPauseInitialMS
		} else {
			t.dutyPauseMS *= config.DefaultScanThrottleDutyPauseGrowFactor
		}
		if t.dutyPauseMS > config.DefaultScanThrottleDutyPauseMaxMS {
			t.dutyPauseMS = config.DefaultScanThrottleDutyPauseMaxMS
		}
		return
	}
	t.dutyPauseMS *= config.DefaultScanThrottleDutyPauseDecayFactor
	if t.dutyPauseMS < 0 {
		t.dutyPauseMS = 0
	}
}

// newAdaptiveThrottleYield wraps baseYield (the existing fixed-interval cooperative yield) with
// the counting-walk contention probe. ScanFunc wraps the result in newDutyCycleYield, so it runs
// once per time-slice sleep, not per entry check. baseYield always runs first (baseline behavior preserved).
// Then, on the probe cadence while throughputBPS reports a positive rate (a transfer is actively
// moving bytes), the counting walk is paused for DefaultScanThrottleProbeWindowMS and the
// resulting throughput change decides whether to grow or decay the duty-cycle pause. Between
// probes, the current duty-cycle pause (if any) is applied instead. throughputBPS == nil disables
// the probe entirely (baseYield-only behavior).
func newAdaptiveThrottleYield(baseYield func(), throughputBPS func() float64) func() {
	t := &adaptiveThrottle{}
	return func() {
		if baseYield != nil {
			baseYield()
		}
		if throughputBPS == nil {
			return
		}
		before := throughputBPS()
		if before > 0 && t.shouldProbe(time.Now()) {
			time.Sleep(time.Duration(config.DefaultScanThrottleProbeWindowMS) * time.Millisecond)
			t.recordProbe(time.Now(), before, throughputBPS())
			return
		}
		if t.dutyPauseMS > 0 {
			time.Sleep(time.Duration(t.dutyPauseMS * float64(time.Millisecond)))
		}
	}
}

// newDutyCycleYield turns sleep (one cooperative pause) into a time-sliced yield for
// ops.PlanBuildOptions.Yield: the walk calls the returned func every
// config.DefaultScanYieldCheckEveryN entries, and sleep runs only once work has elapsed since the
// previous sleep ended. A fixed per-entry-count sleep would cap a walk over cached metadata at a
// constant entries/sec rate whatever the disk speed. Each walk needs its own instance: the
// returned func is not safe for concurrent use. now is time.Now outside tests.
func newDutyCycleYield(sleep func(), work time.Duration, now func() time.Time) func() {
	if sleep == nil {
		return nil
	}
	var sliceStart time.Time
	return func() {
		t := now()
		if sliceStart.IsZero() {
			sliceStart = t
			return
		}
		if t.Sub(sliceStart) < work {
			return
		}
		sleep()
		sliceStart = now()
	}
}
