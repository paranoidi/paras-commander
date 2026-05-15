package jobs

import (
	"time"
)

const etaEMAAlpha = 0.35

const displaySpeedEMAAlpha = 0.15

// etaProgressMinInterval is the minimum wall time between ETA throughput samples.
const etaProgressMinInterval = 50 * time.Millisecond

// etaBlendTransitionSecs controls how fast ETA rate blending shifts from EMA toward cumulative average.
const etaBlendTransitionSecs = 45.0

// etaBlendEMAFloor is the minimum weight kept on EMA after blending fully ramps (rest is cumulative).
const etaBlendEMAFloor = 0.25

// ApplyProgressETA updates smoothed bytes/sec and files/sec from monotonic DoneBytes and DoneFiles samples.
func ApplyProgressETA(job *Job, doneBytes int64, doneFiles int, now time.Time) {
	if job == nil {
		return
	}
	if job.LastProgressSnapshotAt.IsZero() {
		job.LastProgressSnapshotAt = now
		job.LastProgressDoneBytes = doneBytes
		job.LastProgressDoneFiles = doneFiles
		return
	}
	deltaT := now.Sub(job.LastProgressSnapshotAt)
	if deltaT < etaProgressMinInterval {
		return
	}
	deltaB := doneBytes - job.LastProgressDoneBytes
	deltaF := doneFiles - job.LastProgressDoneFiles
	if deltaB < 0 || deltaF < 0 {
		return
	}

	processed := false
	if deltaB > 0 {
		instantB := float64(deltaB) / deltaT.Seconds()
		if job.ETABytesPerSec <= 0 {
			job.ETABytesPerSec = instantB
		} else {
			job.ETABytesPerSec = etaEMAAlpha*instantB + (1-etaEMAAlpha)*job.ETABytesPerSec
		}
		if job.DisplaySpeedBPS <= 0 {
			job.DisplaySpeedBPS = instantB
		} else {
			job.DisplaySpeedBPS = displaySpeedEMAAlpha*instantB + (1-displaySpeedEMAAlpha)*job.DisplaySpeedBPS
		}
		processed = true
	}
	if deltaF > 0 {
		instantF := float64(deltaF) / deltaT.Seconds()
		if job.ETAFilesPerSec <= 0 {
			job.ETAFilesPerSec = instantF
		} else {
			job.ETAFilesPerSec = etaEMAAlpha*instantF + (1-etaEMAAlpha)*job.ETAFilesPerSec
		}
		processed = true
	}
	if !processed {
		job.LastProgressSnapshotAt = now
		return
	}

	job.LastProgressSnapshotAt = now
	job.LastProgressDoneBytes = doneBytes
	job.LastProgressDoneFiles = doneFiles
}

// ResetProgressETA clears ETA smoothing state (call when a job starts running).
func ResetProgressETA(job *Job) {
	if job == nil {
		return
	}
	job.LastProgressSnapshotAt = time.Time{}
	job.LastProgressDoneBytes = 0
	job.LastProgressDoneFiles = 0
	job.ETABytesPerSec = 0
	job.ETAFilesPerSec = 0
	job.DisplaySpeedBPS = 0
	job.ThroughputStrip = nil
	job.ThroughputStripOpenBin = 0
	job.throughputStripOpenSet = false
	job.ThroughputStripDoneAtOpen = 0
}

// blendedETARate mixes EMA with cumulative average; shifts toward cumulative over elapsed wall time.
func blendedETARate(ema, cumulative float64, elapsed time.Duration) float64 {
	if ema <= 0 && cumulative <= 0 {
		return 0
	}
	if ema <= 0 {
		return cumulative
	}
	if cumulative <= 0 {
		return ema
	}
	sec := elapsed.Seconds()
	wEMA := etaBlendEMAFloor
	if sec < etaBlendTransitionSecs {
		t := sec / etaBlendTransitionSecs
		wEMA = 1.0 - (1.0-etaBlendEMAFloor)*t
	}
	return wEMA*ema + (1.0-wEMA)*cumulative
}

func roundRemainSecs(remain float64, rate float64) float64 {
	if remain <= 0 || rate <= 0 {
		return -1
	}
	return remain / rate
}

// FormatETA returns a human-readable ETA label for the jobs UI.
func FormatETA(status Status, startedAt, now time.Time, totalBytes, doneBytes int64, totalFiles, doneFiles int, etaBytesPerSec, etaFilesPerSec float64) string {
	if status != StatusRunning && status != StatusQueued {
		return "—"
	}
	if startedAt.IsZero() {
		return "—"
	}
	elapsed := now.Sub(startedAt)
	if elapsed < time.Second {
		return "…"
	}

	maxSecs := -1.0

	if totalBytes > 0 && doneBytes > 0 {
		cumB := float64(doneBytes) / elapsed.Seconds()
		rateB := blendedETARate(etaBytesPerSec, cumB, elapsed)
		if rateB > 0 {
			remain := float64(totalBytes - doneBytes)
			if secs := roundRemainSecs(remain, rateB); secs >= 0 {
				maxSecs = max64(maxSecs, secs)
			}
		}
	}
	if totalFiles > 0 && doneFiles > 0 {
		cumF := float64(doneFiles) / elapsed.Seconds()
		rateF := blendedETARate(etaFilesPerSec, cumF, elapsed)
		if rateF > 0 {
			remain := float64(totalFiles - doneFiles)
			if secs := roundRemainSecs(remain, rateF); secs >= 0 {
				maxSecs = max64(maxSecs, secs)
			}
		}
	}

	if maxSecs < 0 {
		return "—"
	}
	return (time.Duration(maxSecs) * time.Second).Round(time.Second).String()
}

func max64(a, b float64) float64 {
	if b > a {
		return b
	}
	return a
}
