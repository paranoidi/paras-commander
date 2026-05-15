package jobs

import "time"

// AdvanceJobThroughputStrip records one wall-clock-aligned throughput sample per completed bin.
// New bins are appended on the right; the slice is trimmed from the left to maxBins (scroll).
// When several bins elapse between progress events, the byte delta is spread evenly across those
// bins (constant B/s per closed bin) so idle gaps stay at zero without stretching one sample across the chart.
func AdvanceJobThroughputStrip(job *Job, now time.Time, doneBytes int64, binDur, window time.Duration) {
	if job == nil || binDur <= 0 || window <= 0 {
		return
	}
	binNs := binDur.Nanoseconds()
	maxBins := int(window/binDur) + 10
	const maxStripCap = 640
	if maxBins > maxStripCap {
		maxBins = maxStripCap
	}
	curBin := (now.UnixNano() / binNs) * binNs
	if !job.throughputStripOpenSet {
		job.throughputStripOpenSet = true
		job.ThroughputStripOpenBin = curBin
		job.ThroughputStripDoneAtOpen = doneBytes
		return
	}
	if curBin < job.ThroughputStripOpenBin {
		job.ThroughputStripOpenBin = curBin
		job.ThroughputStripDoneAtOpen = doneBytes
		return
	}
	if curBin == job.ThroughputStripOpenBin {
		return
	}
	db := doneBytes - job.ThroughputStripDoneAtOpen
	nClose := int((curBin - job.ThroughputStripOpenBin) / binNs)
	if nClose <= 0 {
		return
	}
	perSec := float64(db) / (float64(nClose) * binDur.Seconds())
	for i := 0; i < nClose; i++ {
		throughputStripAppend(job, perSec, maxBins)
	}
	job.ThroughputStripOpenBin = curBin
	job.ThroughputStripDoneAtOpen = doneBytes
}

func throughputStripAppend(job *Job, bps float64, maxBins int) {
	job.ThroughputStrip = append(job.ThroughputStrip, bps)
	if len(job.ThroughputStrip) > maxBins {
		job.ThroughputStrip = job.ThroughputStrip[len(job.ThroughputStrip)-maxBins:]
	}
}

// ThroughputChartColumnBuckets maps strip samples to exactly cols columns: right-padded with zeros
// when short, or the cols newest samples when long. No rescaling of values (only braille Y scaling).
func ThroughputChartColumnBuckets(strip []float64, cols int) []float64 {
	if cols <= 0 {
		return nil
	}
	out := make([]float64, cols)
	if len(strip) == 0 {
		return out
	}
	if len(strip) >= cols {
		copy(out, strip[len(strip)-cols:])
		return out
	}
	copy(out[cols-len(strip):], strip)
	return out
}
