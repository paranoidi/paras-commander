package ui

import "testing"

func TestJobsDetailLabelRunes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		line string
		want int
	}{
		{"ETA:         12s", 13},
		{"Source: /a/b:c", 8},
		{"no label here", 0},
		{"Trailing:", 0},
		{"", 0},
	} {
		if got := jobsDetailLabelRunes(tc.line); got != tc.want {
			t.Errorf("jobsDetailLabelRunes(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestIsJobsDetailPlotLine(t *testing.T) {
	t.Parallel()
	if !isJobsDetailPlotLine(" ⣀⣤⣶") || isJobsDetailPlotLine("Speed: 1 MB/s") || isJobsDetailPlotLine("⠀ ") {
		t.Fatal("plot line detection wrong")
	}
}
