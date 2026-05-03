package ui

import "testing"

func TestJobPercentDoneCompletedReportsFull(t *testing.T) {
	p := jobPercentDone(JobEntry{Status: "completed", DoneFiles: 99, TotalFiles: 100})
	if p != 100 {
		t.Fatalf("completed job with stale DoneFiles: got %v, want 100", p)
	}
}

func TestJobPercentDoneRunningUsesByteRatioWhenTotalBytesSet(t *testing.T) {
	p := jobPercentDone(JobEntry{
		Status: "running", DoneFiles: 1, TotalFiles: 10,
		DoneBytes: 500_000_000, TotalBytes: 1_000_000_000,
	})
	if p != 50 {
		t.Fatalf("got %v, want 50 from DoneBytes/TotalBytes", p)
	}
}

func TestJobPercentDoneRunningFallsBackToFilesWhenNoByteTotal(t *testing.T) {
	p := jobPercentDone(JobEntry{Status: "running", DoneFiles: 50, TotalFiles: 100, TotalBytes: 0})
	if p != 50 {
		t.Fatalf("got %v, want 50 from DoneFiles/TotalFiles", p)
	}
}

func TestJobPercentDoneSingleLargeFileUsesBytes(t *testing.T) {
	p := jobPercentDone(JobEntry{
		Status: "running", DoneFiles: 0, TotalFiles: 1,
		DoneBytes: 400_000_000, TotalBytes: 1_000_000_000,
	})
	if p != 40 {
		t.Fatalf("got %v, want 40 while first file still in progress", p)
	}
}

func TestJobPercentDoneCapsByteRatioAt100(t *testing.T) {
	p := jobPercentDone(JobEntry{
		Status: "running", DoneFiles: 1, TotalFiles: 2,
		DoneBytes: 2_000_000_000, TotalBytes: 1_000_000_000,
	})
	if p != 100 {
		t.Fatalf("got %v, want 100", p)
	}
}

func TestJobRowLeadingIconCompletedUsesDoneGlyph(t *testing.T) {
	if got := jobRowLeadingIcon("completed"); got != "\uf05d" {
		t.Fatalf("completed icon = %q, want %q", got, "\uf05d")
	}
}

func TestJobRowLeadingIconFailedUsesAlertGlyph(t *testing.T) {
	want := "\U000f0028"
	if got := jobRowLeadingIcon("failed"); got != want {
		t.Fatalf("failed icon = %q, want %q", got, want)
	}
}
