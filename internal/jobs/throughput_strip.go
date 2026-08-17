package jobs

import "time"

// throughputStripMaxCap bounds strip memory regardless of window/column configuration.
const throughputStripMaxCap = 640

func throughputStripMaxBins(columnDur, window time.Duration) int {
	return min(int(window/columnDur)+10, throughputStripMaxCap)
}

// SampleThroughputColumns advances the job's fixed wall-clock column grid to now.
//
// The column grid is the single sampling clock for transfer speed: every fully elapsed column
// feeds one instantaneous B/s value (bytes moved since the grid last advanced, spread evenly over
// the elapsed columns) into job.DisplaySpeedBPS, the EMA shown in the Speed column and plotted by
// the details chart. Sampling on this grid rather than on worker progress events keeps the speed
// and the chart independent of the throttled, irregular EventProgress cadence: idle columns feed
// zero and decay the EMA instead of being skipped, and a burst of bytes delivered in one event is
// spread across the columns it actually covers instead of spiking a single one.
//
// When recordStrip is true each closed column also appends the current DisplaySpeedBPS to the
// strip, so the chart scrolls exactly one column per columnDur while the job runs. Leading zero
// columns before the first bytes move are not recorded (the UI shows "collecting samples" until
// then). Returns true when the strip grew.
func SampleThroughputColumns(job *Job, now time.Time, doneBytes int64, columnDur, window time.Duration, recordStrip bool) bool {
	if job == nil || columnDur <= 0 || window <= 0 {
		return false
	}
	if !job.throughputStripOpenSet {
		job.throughputStripOpenSet = true
		job.ThroughputStripOpenBin = now.UnixNano()
		job.ThroughputStripDoneAtOpen = doneBytes
		return false
	}
	openStart := time.Unix(0, job.ThroughputStripOpenBin)
	if now.Before(openStart) {
		// Wall clock moved backwards; re-anchor rather than replay the grid.
		job.ThroughputStripOpenBin = now.UnixNano()
		job.ThroughputStripDoneAtOpen = doneBytes
		return false
	}
	cols := int(now.Sub(openStart) / columnDur)
	if cols <= 0 {
		return false
	}

	db := doneBytes - job.ThroughputStripDoneAtOpen
	instant := 0.0
	if db > 0 {
		instant = float64(db) / (float64(cols) * columnDur.Seconds())
	}
	job.ThroughputStripOpenBin = openStart.Add(time.Duration(cols) * columnDur).UnixNano()
	job.ThroughputStripDoneAtOpen = doneBytes

	maxBins := throughputStripMaxBins(columnDur, window)
	// A long main-loop stall replays at most a full strip worth of columns; the EMA has
	// converged well before that, so further iterations would change nothing.
	if cols > maxBins {
		cols = maxBins
	}
	grew := false
	for i := 0; i < cols; i++ {
		if job.DisplaySpeedBPS <= 0 {
			job.DisplaySpeedBPS = instant
		} else {
			job.DisplaySpeedBPS = displaySpeedEMAAlpha*instant + (1-displaySpeedEMAAlpha)*job.DisplaySpeedBPS
		}
		if !recordStrip || (job.DisplaySpeedBPS <= 0 && len(job.ThroughputStrip) == 0) {
			continue
		}
		throughputStripAppend(job, job.DisplaySpeedBPS, maxBins)
		grew = true
	}
	return grew
}

func throughputStripAppend(job *Job, bps float64, maxBins int) {
	job.ThroughputStrip = append(job.ThroughputStrip, bps)
	if len(job.ThroughputStrip) > maxBins {
		job.ThroughputStrip = job.ThroughputStrip[len(job.ThroughputStrip)-maxBins:]
	}
}

// ThroughputChartColumnBuckets returns the newest strip samples up to cols entries.
// It does not left-pad with zeros (padding would look like empty time periods on the chart).
func ThroughputChartColumnBuckets(strip []float64, cols int) []float64 {
	if cols <= 0 || len(strip) == 0 {
		return nil
	}
	if len(strip) <= cols {
		out := make([]float64, len(strip))
		copy(out, strip)
		return out
	}
	out := make([]float64, cols)
	copy(out, strip[len(strip)-cols:])
	return out
}
