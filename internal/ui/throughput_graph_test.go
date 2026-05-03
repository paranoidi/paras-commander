package ui

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestThroughputBucketMaxMonotonic(t *testing.T) {
	samples := []float64{10, 10, 10, 1000, 10, 10}
	b := throughputBucketMax(samples, 6)
	lowCount, highCount := 0, 0
	for _, v := range b {
		if v <= 50 {
			lowCount++
		}
		if v >= 500 {
			highCount++
		}
	}
	if highCount < 1 {
		t.Fatalf("expected at least one bucket with peak ~1000: %#v", b)
	}
	if lowCount == 0 {
		t.Fatalf("expected some low buckets: %#v", b)
	}
}

func TestThroughputGraphBodyBrailleFlatSeries(t *testing.T) {
	buckets := []float64{5, 5, 5, 5}
	lines := throughputGraphBodyBraille(buckets, 4)
	if len(lines) != 4 {
		t.Fatalf("len(lines) = %d want 4", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, " ") {
			t.Fatalf("expected leading margin: %q", line)
		}
		for _, r := range line[1:] {
			if r != ' ' && (r < '\u2800' || r > '\u28ff') {
				t.Fatalf("expected space or braille rune, got %q in %q", r, line)
			}
		}
	}
}

func TestThroughputBucketsWindowFixedWallTime(t *testing.T) {
	now := time.Date(2022, 3, 4, 5, 6, 7, 0, time.UTC)
	start := now.Add(-jobs.ThroughputDetailChartWindow)
	// 30 columns × 30s window → 1s per column; sample must fall strictly inside [slotStart, slotEnd).
	samples := []jobs.ThroughputSample{
		{At: start.Add(500 * time.Millisecond), BPS: 100},
		{At: start.Add(19*time.Second + 500*time.Millisecond), BPS: 200},
	}
	b := throughputBucketsWindow(samples, now, 30, jobs.ThroughputDetailChartWindow)
	if len(b) != 30 {
		t.Fatalf("len %d", len(b))
	}
	if b[0] != 100 {
		t.Fatalf("col0 = %v want 100", b[0])
	}
	if b[19] != 200 {
		t.Fatalf("col19 = %v want 200", b[19])
	}
}

func TestThroughputDetailLinesRunningPlaceholder(t *testing.T) {
	lines := ThroughputDetailLines([]jobs.ThroughputSample{{BPS: 10}}, time.Now(), 40, true)
	if len(lines) != 1 {
		t.Fatalf("len = %d want 1", len(lines))
	}
	if !strings.Contains(lines[0], "collecting") {
		t.Fatalf("want collecting placeholder: %#v", lines)
	}
}

func TestThroughputDetailLinesGraphShape(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 45, 0, time.UTC)
	samples := make([]jobs.ThroughputSample, 30)
	for i := range samples {
		samples[i] = jobs.ThroughputSample{
			At:  now.Add(time.Duration(-29+i) * time.Second),
			BPS: float64(i+1) * 1e6,
		}
	}
	lines := ThroughputDetailLines(samples, now, 50, true)
	if len(lines) != throughputGraphBodyRows {
		t.Fatalf("len(lines) = %d want %d", len(lines), throughputGraphBodyRows)
	}
	foundBraille := false
	for _, row := range lines {
		for _, r := range row {
			if r >= '\u2800' && r <= '\u28ff' && r != '\u2800' {
				foundBraille = true
				break
			}
		}
		if foundBraille {
			break
		}
	}
	if !foundBraille {
		t.Fatalf("expected non-empty braille in graph: %#v", lines)
	}
	for _, row := range lines {
		if strings.ContainsRune(row, '█') {
			t.Fatalf("did not expect block glyph: %q", row)
		}
	}
	if !strings.HasPrefix(lines[0], " ") {
		t.Fatalf("expected leading margin: %q", lines[0])
	}
	if utf8.RuneCountInString(lines[0]) != 50 {
		t.Fatalf("first graph line width = %d want 50", utf8.RuneCountInString(lines[0]))
	}
}
