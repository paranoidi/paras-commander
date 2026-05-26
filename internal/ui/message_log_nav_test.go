package ui

import "testing"

func TestAdjacentMessageLogIndexSkipsWrappedLines(t *testing.T) {
	t.Parallel()
	entries := []MessageLogEntry{
		{Time: "12:00:00", Text: "Job failed: delete"},
		{Time: "", Text: "failed to delete v1"},
		{Time: "11:59:00", Text: "Job started"},
	}
	if got := AdjacentMessageLogIndex(entries, 0, 1); got != 2 {
		t.Fatalf("down from 0 = %d, want 2", got)
	}
	if got := AdjacentMessageLogIndex(entries, 1, 1); got != 2 {
		t.Fatalf("down from continuation = %d, want 2", got)
	}
	if got := AdjacentMessageLogIndex(entries, 2, -1); got != 0 {
		t.Fatalf("up from 2 = %d, want 0", got)
	}
	if got := AdjacentMessageLogIndex(entries, 1, -1); got != 0 {
		t.Fatalf("up from continuation = %d, want 0", got)
	}
}
