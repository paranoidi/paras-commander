package jobs

import "time"

// CloseOneThroughputColumn closes at most one fixed-duration column on the throughput strip.
// The first call anchors the open column; later calls append one B/s sample when columnDur has elapsed.
// Returns true when a sample was appended.
func CloseOneThroughputColumn(job *Job, now time.Time, doneBytes int64, columnDur, window time.Duration) bool {
	if job == nil || columnDur <= 0 || window <= 0 {
		return false
	}
	maxBins := int(window/columnDur) + 10
	const maxStripCap = 640
	if maxBins > maxStripCap {
		maxBins = maxStripCap
	}
	if !job.throughputStripOpenSet {
		job.throughputStripOpenSet = true
		job.ThroughputStripOpenBin = now.UnixNano()
		job.ThroughputStripDoneAtOpen = doneBytes
		return false
	}
	openStart := time.Unix(0, job.ThroughputStripOpenBin)
	if now.Before(openStart) {
		job.ThroughputStripOpenBin = now.UnixNano()
		job.ThroughputStripDoneAtOpen = doneBytes
		return false
	}
	if now.Sub(openStart) < columnDur {
		return false
	}
	db := doneBytes - job.ThroughputStripDoneAtOpen
	nextOpen := openStart.Add(columnDur)
	job.ThroughputStripOpenBin = nextOpen.UnixNano()
	job.ThroughputStripDoneAtOpen = doneBytes
	if db <= 0 {
		// Advance the column clock without recording a sample (metadata / idle gaps).
		return false
	}
	bps := float64(db) / columnDur.Seconds()
	throughputStripAppend(job, bps, maxBins)
	return true
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
