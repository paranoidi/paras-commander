package ui

import (
	"strings"
	"testing"
	"unicode/utf8"
)

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

func TestThroughputDetailLinesRunningPlaceholder(t *testing.T) {
	lines := ThroughputDetailLines(nil, 40, true)
	if len(lines) != 1 {
		t.Fatalf("len = %d want 1", len(lines))
	}
	if !strings.Contains(lines[0], "collecting") {
		t.Fatalf("want collecting placeholder: %#v", lines)
	}
}

func TestThroughputDetailLinesGraphShape(t *testing.T) {
	strip := make([]float64, 30)
	for i := range strip {
		strip[i] = float64(i+1) * 1e6
	}
	lines := ThroughputDetailLines(strip, 50, true)
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
