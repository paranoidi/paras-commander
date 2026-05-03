package ui

import (
	"math"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

const throughputGraphBodyRows = 5

// brailleGraphUp matches btop Symbols::graph_symbols "braille_up" (index = a*5+b, a,b in 0..4).
// See btop/src/btop_draw.cpp Draw::Graph::_create.
var brailleGraphUp = [25]rune{
	' ', '⢀', '⢠', '⢰', '⢸',
	'⡀', '⣀', '⣠', '⣰', '⣸',
	'⡄', '⣄', '⣤', '⣴', '⣼',
	'⡆', '⣆', '⣦', '⣶', '⣾',
	'⡇', '⣇', '⣧', '⣷', '⣿',
}

// throughputBucketMax aggregates samples into fixed buckets using max within each bucket.
func throughputBucketMax(samples []float64, buckets int) []float64 {
	if buckets <= 0 {
		return nil
	}
	out := make([]float64, buckets)
	if len(samples) == 0 {
		return out
	}
	n := len(samples)
	for i := range out {
		start := i * n / buckets
		end := (i + 1) * n / buckets
		if end <= start {
			end = start + 1
		}
		if end > n {
			end = n
		}
		maxv := samples[start]
		for k := start; k < end; k++ {
			if samples[k] > maxv {
				maxv = samples[k]
			}
		}
		out[i] = maxv
	}
	return out
}

// throughputBucketsWindow maps each chart column to an equal wall-time slice of [now-window, now).
// The max BPS within each slice becomes that column's value (0 if no sample fell in the slice).
func throughputBucketsWindow(samples []jobs.ThroughputSample, now time.Time, cols int, window time.Duration) []float64 {
	if cols <= 0 || window <= 0 {
		return nil
	}
	out := make([]float64, cols)
	if len(samples) == 0 {
		return out
	}
	start := now.Add(-window)
	winNs := window.Nanoseconds()
	if winNs <= 0 {
		return out
	}
	for col := 0; col < cols; col++ {
		slotStart := start.Add(time.Duration(winNs * int64(col) / int64(cols)))
		slotEnd := start.Add(time.Duration(winNs * int64(col+1) / int64(cols)))
		var maxv float64
		found := false
		for _, s := range samples {
			if s.At.Before(slotStart) {
				continue
			}
			if col == cols-1 {
				if s.At.After(now) {
					continue
				}
			} else if !s.At.Before(slotEnd) {
				continue
			}
			if !found || s.BPS > maxv {
				maxv = s.BPS
				found = true
			}
		}
		if found {
			out[col] = maxv
		}
	}
	return out
}

func throughputYCap(bucketMax []float64) float64 {
	globalMax := 0.0
	for _, v := range bucketMax {
		if v > globalMax {
			globalMax = v
		}
	}
	yCap := globalMax * 1.05
	if yCap <= 1e-9 {
		yCap = 1
	}
	return yCap
}

func scaleThroughputTo100(v float64, yCap float64) int {
	if yCap <= 1e-9 {
		return 0
	}
	x := v / yCap * 100
	if x < 0 {
		return 0
	}
	if x > 100 {
		return 100
	}
	return int(math.Round(x))
}

// subBand maps a 0..100 value to 0..4 for one horizontal braille pair component (btop Graph::_create).
func subBand(value, curLow, curHigh int, mod float64, clampMin int) int {
	if value >= curHigh {
		return 4
	}
	if value <= curLow {
		return clampMin
	}
	den := curHigh - curLow
	if den <= 0 {
		return clampMin
	}
	inner := float64(value-curLow)*4/float64(den) + mod
	return clampInt(int(math.Round(inner)), clampMin, 4)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// throughputGraphBodyBraille renders throughput buckets using btop-style braille_up glyphs
// (two consecutive 0..100 samples per column: baseline 0 then each bucket, same width as bucket count).
func throughputGraphBodyBraille(bucketMax []float64, graphHeight int) []string {
	width := len(bucketMax)
	if width <= 0 || graphHeight <= 0 {
		return nil
	}
	yCap := throughputYCap(bucketMax)
	scaled := make([]int, width)
	for i := range bucketMax {
		scaled[i] = scaleThroughputTo100(bucketMax[i], yCap)
	}

	mod := 0.1
	if graphHeight == 1 {
		mod = 0.3
	}

	lines := make([]string, graphHeight)
	last := 0
	for col := 0; col < width; col++ {
		dataVal := scaled[col]
		for horizon := 0; horizon < graphHeight; horizon++ {
			var curHigh, curLow int
			if graphHeight > 1 {
				curHigh = int(math.Round(100.0 * float64(graphHeight-horizon) / float64(graphHeight)))
				curLow = int(math.Round(100.0 * float64(graphHeight-(horizon+1)) / float64(graphHeight)))
			} else {
				curHigh = 100
				curLow = 0
			}
			clampMin := 0
			a := subBand(last, curLow, curHigh, mod, clampMin)
			b := subBand(dataVal, curLow, curHigh, mod, clampMin)
			r := brailleGraphUp[a*5+b]
			if col == 0 {
				lines[horizon] = " " + string(r)
			} else {
				lines[horizon] += string(r)
			}
		}
		last = dataVal
	}
	return lines
}

// ThroughputDetailLines returns detail-panel lines for the throughput chart section.
// width is the interior content width in runes (one leading margin space is added per line).
func ThroughputDetailLines(samples []jobs.ThroughputSample, now time.Time, width int, running bool) []string {
	if !running {
		return nil
	}
	if width < 8 {
		width = 8
	}
	chartCols := width - 1
	if chartCols < 1 {
		chartCols = 1
	}

	if len(samples) < 2 {
		return []string{" " + truncateRunes("(collecting samples…)", width-1)}
	}

	buckets := throughputBucketsWindow(samples, now, chartCols, jobs.ThroughputDetailChartWindow)
	body := throughputGraphBodyBraille(buckets, throughputGraphBodyRows)
	if len(body) == 0 {
		return []string{" (graph error)"}
	}
	return body
}
