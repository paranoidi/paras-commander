package ui

import (
	"math"
	"strings"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
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
// strip holds one fixed-clock B/s sample per column (oldest left, newest right); see jobs.CloseOneThroughputColumn.
func ThroughputDetailLines(strip []float64, width int, running bool) []string {
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

	if len(strip) == 0 {
		return []string{" " + truncateRunes("(collecting samples"+string(primitive.Ellipsis)+")", width-1)}
	}

	buckets := jobs.ThroughputChartColumnBuckets(strip, chartCols)
	body := throughputGraphBodyBraille(buckets, throughputGraphBodyRows)
	if len(body) == 0 {
		return []string{" (graph error)"}
	}
	// Right-align samples in the chart area (no fake zero-throughput columns on the left).
	padCols := chartCols - len(buckets)
	if padCols > 0 {
		pad := strings.Repeat(" ", padCols)
		for i := range body {
			body[i] = " " + pad + body[i][1:]
		}
	}
	return body
}
