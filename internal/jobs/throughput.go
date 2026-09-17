package jobs

import (
	"fmt"
	"math"
	"time"
)

// FormatThroughput renders bytes/sec compactly for the jobs UI (e.g. "82MB/s", "1.5KB/s").
func FormatThroughput(bps float64) string { return formatThroughput(bps, true) }

// FormatThroughputWhole is FormatThroughput without decimals ("97MB/s"), for the menu-bar speed
// pill. Output is at most 7 cells ("999MB/s"): a value rounding to 1024 moves to the next unit.
func FormatThroughputWhole(bps float64) string { return formatThroughput(bps, false) }

func formatThroughput(bps float64, decimals bool) string {
	if bps <= 0 || math.IsNaN(bps) || math.IsInf(bps, 0) {
		return "—"
	}
	units := [...]string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	i := 0
	for i < len(units)-1 && math.Round(bps) >= 1024 {
		bps /= 1024
		i++
	}
	if decimals && i > 0 && bps < 100 && !isNearInt(bps) {
		return fmt.Sprintf("%.1f%s", bps, units[i])
	}
	return fmt.Sprintf("%.0f%s", bps, units[i])
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
