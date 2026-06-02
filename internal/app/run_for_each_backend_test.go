package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSummarizeRunForEachIssuesFailed(t *testing.T) {
	entries := []ui.CommandRunEntry{
		{ExitCode: 1, ErrorMsg: "boom"},
		{ExitCode: 0},
	}
	log, banner, urg, ok := summarizeRunForEachIssues("Run for each", entries)
	if !ok {
		t.Fatal("expected issue summary")
	}
	if urg != ui.MessageUrgencyError {
		t.Fatalf("urgency = %v", urg)
	}
	if log != "Run for each: 1 failed (boom)" {
		t.Fatalf("log = %q", log)
	}
	if banner == "" {
		t.Fatal("expected banner")
	}
}

func TestSummarizeRunForEachIssuesOK(t *testing.T) {
	_, _, _, ok := summarizeRunForEachIssues("Run for each", []ui.CommandRunEntry{{ExitCode: 0}})
	if ok {
		t.Fatal("expected no summary for all-success batch")
	}
}
