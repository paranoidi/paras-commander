package jobs

import (
	"time"
)

// EstimateJobRemainingSecs returns estimated seconds until the job completes, or -1 if unknown.
func EstimateJobRemainingSecs(j *Job, now time.Time, refBytesPerSec, refFilesPerSec float64) float64 {
	if j == nil || j.Status.IsFinished() || j.Status == StatusScanning {
		return -1
	}

	switch j.Status {
	case StatusRunning:
		return estimateRunningRemainSecs(j, now)
	case StatusQueued, StatusPaused, StatusWaitingDecision:
		return estimateQueuedRemainSecs(j, refBytesPerSec, refFilesPerSec)
	default:
		return -1
	}
}

func estimateRunningRemainSecs(j *Job, now time.Time) float64 {
	if j.StartedAt.IsZero() {
		return -1
	}
	elapsed := now.Sub(j.StartedAt)
	if elapsed < time.Second {
		return -1
	}
	return estimateRemainFromTotals(j.TotalBytes, j.DoneBytes, j.TotalFiles, j.DoneFiles,
		j.ETABytesPerSec, j.ETAFilesPerSec, elapsed)
}

func estimateQueuedRemainSecs(j *Job, refBytesPerSec, refFilesPerSec float64) float64 {
	if refBytesPerSec <= 0 && refFilesPerSec <= 0 {
		return -1
	}
	if j.TotalFiles <= 0 && j.TotalBytes <= 0 {
		return -1
	}
	return estimateRemainFromTotals(j.TotalBytes, 0, j.TotalFiles, 0, refBytesPerSec, refFilesPerSec, time.Second)
}

func estimateRemainFromTotals(totalBytes, doneBytes int64, totalFiles, doneFiles int,
	etaBytesPerSec, etaFilesPerSec float64, elapsed time.Duration,
) float64 {
	maxSecs := -1.0

	if totalBytes > 0 && doneBytes >= 0 {
		var rateB float64
		if doneBytes > 0 && elapsed >= time.Second {
			cumB := float64(doneBytes) / elapsed.Seconds()
			rateB = blendedETARate(etaBytesPerSec, cumB, elapsed)
		} else if etaBytesPerSec > 0 {
			rateB = etaBytesPerSec
		}
		if rateB > 0 {
			remain := float64(totalBytes - doneBytes)
			if secs := roundRemainSecs(remain, rateB); secs >= 0 {
				maxSecs = max64(maxSecs, secs)
			}
		}
	}
	if totalFiles > 0 && doneFiles >= 0 {
		var rateF float64
		if doneFiles > 0 && elapsed >= time.Second {
			cumF := float64(doneFiles) / elapsed.Seconds()
			rateF = blendedETARate(etaFilesPerSec, cumF, elapsed)
		} else if etaFilesPerSec > 0 {
			rateF = etaFilesPerSec
		}
		if rateF > 0 {
			remain := float64(totalFiles - doneFiles)
			if secs := roundRemainSecs(remain, rateF); secs >= 0 {
				maxSecs = max64(maxSecs, secs)
			}
		}
	}
	return maxSecs
}

func referenceTransferRates(jobs []*Job, now time.Time) (bytesPerSec, filesPerSec float64) {
	for _, j := range jobs {
		if j == nil || j.Status != StatusRunning || j.StartedAt.IsZero() {
			continue
		}
		elapsed := now.Sub(j.StartedAt)
		if elapsed < time.Second {
			continue
		}
		if j.TotalBytes > 0 && j.DoneBytes > 0 {
			cumB := float64(j.DoneBytes) / elapsed.Seconds()
			bytesPerSec = blendedETARate(j.ETABytesPerSec, cumB, elapsed)
		} else if j.ETABytesPerSec > 0 {
			bytesPerSec = j.ETABytesPerSec
		}
		if j.TotalFiles > 0 && j.DoneFiles > 0 {
			cumF := float64(j.DoneFiles) / elapsed.Seconds()
			filesPerSec = blendedETARate(j.ETAFilesPerSec, cumF, elapsed)
		} else if j.ETAFilesPerSec > 0 {
			filesPerSec = j.ETAFilesPerSec
		}
		return bytesPerSec, filesPerSec
	}
	return 0, 0
}

// ComputeQueueETAs returns cumulative queue ETAs keyed by job ID (unfinished jobs only).
func ComputeQueueETAs(jobs []*Job, now time.Time) map[string]string {
	refB, refF := referenceTransferRates(jobs, now)
	out := make(map[string]string)
	var offsetSecs float64
	for _, j := range jobs {
		if j == nil || j.Status.IsFinished() {
			continue
		}
		remain := EstimateJobRemainingSecs(j, now, refB, refF)
		if remain < 0 {
			out[j.ID] = "—"
			continue
		}
		if j.Status == StatusRunning && !j.StartedAt.IsZero() && now.Sub(j.StartedAt) < time.Second {
			out[j.ID] = "…"
			continue
		}
		total := offsetSecs + remain
		offsetSecs = total
		out[j.ID] = formatETADuration(total)
	}
	return out
}

func formatETADuration(secs float64) string {
	if secs < 0 {
		return "—"
	}
	return (time.Duration(secs) * time.Second).Round(time.Second).String()
}

// FormatQueueETA returns the display ETA for one job using precomputed queue offsets.
func FormatQueueETA(jobID string, status Status, queueETAs map[string]string) string {
	if status == StatusScanning {
		return "—"
	}
	if s, ok := queueETAs[jobID]; ok && s != "" {
		return s
	}
	return "—"
}
