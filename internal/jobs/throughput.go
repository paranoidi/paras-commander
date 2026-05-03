package jobs

import (
	"fmt"
	"math"
	"time"
)

// FormatThroughput renders bytes/sec compactly for the jobs UI (e.g. "82MB/s", "640KB/s").
func FormatThroughput(bps float64) string {
	if bps <= 0 || math.IsNaN(bps) || math.IsInf(bps, 0) {
		return "—"
	}
	const kb = 1024.0
	const mb = kb * 1024
	const gb = mb * 1024
	switch {
	case bps >= gb:
		v := bps / gb
		if v >= 100 || isNearInt(v) {
			return fmt.Sprintf("%.0fGB/s", v)
		}
		return fmt.Sprintf("%.1fGB/s", v)
	case bps >= mb:
		v := bps / mb
		if v >= 100 || isNearInt(v) {
			return fmt.Sprintf("%.0fMB/s", v)
		}
		return fmt.Sprintf("%.1fMB/s", v)
	case bps >= kb:
		v := bps / kb
		if v >= 100 || isNearInt(v) {
			return fmt.Sprintf("%.0fKB/s", v)
		}
		return fmt.Sprintf("%.1fKB/s", v)
	default:
		return fmt.Sprintf("%.0fB/s", bps)
	}
}

func isNearInt(v float64) bool {
	return math.Abs(v-math.Round(v)) < 1e-3
}

// EffectiveDisplayThroughputBPS chooses displayed throughput: DisplaySpeedBPS when warmed,
// otherwise lifetime average while running after one second with bytes moved.
func EffectiveDisplayThroughputBPS(status Status, startedAt, now time.Time, doneBytes int64, displaySpeedBPS float64) float64 {
	if status != StatusRunning && status != StatusQueued {
		return 0
	}
	if displaySpeedBPS > 0 {
		return displaySpeedBPS
	}
	if startedAt.IsZero() {
		return 0
	}
	elapsed := now.Sub(startedAt)
	if elapsed < time.Second || doneBytes <= 0 {
		return 0
	}
	return float64(doneBytes) / elapsed.Seconds()
}
