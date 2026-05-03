package jobs

import (
	"time"
)

const etaEMAAlpha = 0.35

const displaySpeedEMAAlpha = 0.15

// ThroughputDetailChartWindow is the wall-time span shown across the details panel throughput chart.
const ThroughputDetailChartWindow = 30 * time.Second

const maxThroughputSamples = 400 // ~30s at fine-grained progress updates; older samples are dropped by time first.

// ApplyProgressETA updates smoothed throughput from monotonic DoneBytes samples.
func ApplyProgressETA(job *Job, doneBytes int64, now time.Time) {
	if job == nil {
		return
	}
	if job.LastProgressSnapshotAt.IsZero() {
		job.LastProgressSnapshotAt = now
		job.LastProgressDoneBytes = doneBytes
		return
	}
	deltaB := doneBytes - job.LastProgressDoneBytes
	deltaT := now.Sub(job.LastProgressSnapshotAt)
	if deltaB < 0 || deltaT < 50*time.Millisecond {
		return
	}
	instant := float64(deltaB) / deltaT.Seconds()
	if instant <= 0 {
		return
	}
	if job.ETABytesPerSec <= 0 {
		job.ETABytesPerSec = instant
	} else {
		job.ETABytesPerSec = etaEMAAlpha*instant + (1-etaEMAAlpha)*job.ETABytesPerSec
	}
	if job.DisplaySpeedBPS <= 0 {
		job.DisplaySpeedBPS = instant
	} else {
		job.DisplaySpeedBPS = displaySpeedEMAAlpha*instant + (1-displaySpeedEMAAlpha)*job.DisplaySpeedBPS
	}
	appendThroughputSample(job, now, instant)
	job.LastProgressSnapshotAt = now
	job.LastProgressDoneBytes = doneBytes
}

func appendThroughputSample(job *Job, now time.Time, instant float64) {
	if instant <= 0 || job == nil {
		return
	}
	job.ThroughputSamples = append(job.ThroughputSamples, ThroughputSample{At: now, BPS: instant})
	cutoff := now.Add(-ThroughputDetailChartWindow)
	for len(job.ThroughputSamples) > 0 && job.ThroughputSamples[0].At.Before(cutoff) {
		job.ThroughputSamples = job.ThroughputSamples[1:]
	}
	if len(job.ThroughputSamples) > maxThroughputSamples {
		job.ThroughputSamples = job.ThroughputSamples[len(job.ThroughputSamples)-maxThroughputSamples:]
	}
}

// ResetProgressETA clears ETA smoothing state (call when a job starts running).
func ResetProgressETA(job *Job) {
	if job == nil {
		return
	}
	job.LastProgressSnapshotAt = time.Time{}
	job.LastProgressDoneBytes = 0
	job.ETABytesPerSec = 0
	job.DisplaySpeedBPS = 0
	job.ThroughputSamples = nil
}

// FormatETA returns a human-readable ETA label for the jobs UI.
func FormatETA(status Status, startedAt, now time.Time, totalBytes, doneBytes int64, totalFiles, doneFiles int, etaBytesPerSec float64) string {
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
	if totalBytes > 0 && doneBytes > 0 {
		rate := etaBytesPerSec
		if rate <= 0 {
			rate = float64(doneBytes) / elapsed.Seconds()
		}
		if rate > 0 {
			remain := totalBytes - doneBytes
			if remain <= 0 {
				return "0s"
			}
			secs := float64(remain) / rate
			return (time.Duration(secs) * time.Second).Round(time.Second).String()
		}
	}
	if totalFiles > 0 && doneFiles > 0 {
		rate := float64(doneFiles) / elapsed.Seconds()
		if rate > 0 {
			remain := totalFiles - doneFiles
			if remain <= 0 {
				return "0s"
			}
			secs := float64(remain) / rate
			return (time.Duration(secs) * time.Second).Round(time.Second).String()
		}
	}
	return "—"
}
