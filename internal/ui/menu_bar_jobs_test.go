package ui

import "testing"

func TestLayoutMenuBarJobsStrip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                  string
		total                 int
		queueLen              int
		wantProgress          bool
		wantQueueW, wantProgW int
	}{
		{"zero width", 0, 3, true, 0, 0},
		{"both full", 20, 5, true, 5, 14},
		{"progress min width", 9, 5, true, 5, 3},
		{"drop progress narrow", 8, 5, true, 5, 0},
		{"queue too long progress only", 8, 10, true, 0, 8},
		{"queue too long no progress want", 8, 10, false, 0, 0},
		{"queue fits no progress", 8, 5, false, 5, 0},
		{"progress only", 10, 0, true, 0, 10},
		{"progress too narrow alone", 2, 0, true, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, p := LayoutMenuBarJobsStrip(tc.total, tc.queueLen, tc.wantProgress)
			if q != tc.wantQueueW || p != tc.wantProgW {
				t.Fatalf("LayoutMenuBarJobsStrip(%d,%d,%v) = (%d,%d), want (%d,%d)",
					tc.total, tc.queueLen, tc.wantProgress, q, p, tc.wantQueueW, tc.wantProgW)
			}
		})
	}
}

func TestMenuBarProgressFilledCells(t *testing.T) {
	t.Parallel()
	if !menuBarProgressFilledCells(0.5, 10, 4) {
		t.Fatal("50% of 10 => 5 filled, index 4 should be filled")
	}
	if menuBarProgressFilledCells(0.5, 10, 5) {
		t.Fatal("50% of 10 => 5 filled, index 5 should be empty")
	}
	if !menuBarProgressFilledCells(1, 3, 2) {
		t.Fatal("100% => all filled")
	}
	if menuBarProgressFilledCells(0, 8, 0) {
		t.Fatal("0% => none filled")
	}
}
