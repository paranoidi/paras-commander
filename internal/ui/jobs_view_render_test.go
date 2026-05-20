package ui

import (
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelZoomSplitsColumnsOnlyInBrowser(t *testing.T) {
	if !PanelZoomSplitsColumns(ViewBrowser, true) {
		t.Fatal("browser + zoom: want split columns")
	}
	if PanelZoomSplitsColumns(ViewBrowser, false) {
		t.Fatal("browser + zoom off: no split")
	}
	for _, vm := range []ViewMode{ViewJobs, ViewCommands, ViewMessages, ViewFilePreview} {
		if PanelZoomSplitsColumns(vm, true) {
			t.Fatalf("%v + zoom: auxiliary views must not use zoomed split", vm)
		}
	}
}

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

func TestJobDetailLineCountOmitsThroughputGraphWhenDisabled(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	j := JobEntry{
		ID:              "x",
		Type:            "copy",
		Status:          "running",
		StartedAt:       now.Add(-time.Minute),
		Sources:         []string{"/tmp/a"},
		Destination:     "/tmp/b",
		ThroughputStrip: []float64{1, 2, 3, 4, 5},
	}
	on := JobDetailLineCount(j, now, true)
	off := JobDetailLineCount(j, now, false)
	if d := on - off; d != throughputGraphBodyRows {
		t.Fatalf("line delta = %d want %d (graph body rows)", d, throughputGraphBodyRows)
	}
}

func TestJobRowLeadingIconCompletedUsesDoneGlyph(t *testing.T) {
	if got := jobRowLeadingIcon("completed", theme.Theme{}); got != "\uf05d" {
		t.Fatalf("completed icon = %q, want %q", got, "\uf05d")
	}
}

func TestJobRowLeadingIconFailedUsesErrorGlyph(t *testing.T) {
	want := "\uf06a"
	if got := jobRowLeadingIcon("failed", theme.Theme{}); got != want {
		t.Fatalf("failed icon = %q, want %q", got, want)
	}
}

func TestJobRowLeadingIconCanceledUsesStoppedGlyph(t *testing.T) {
	want := "\uf28d"
	if got := jobRowLeadingIcon("canceled", theme.Theme{}); got != want {
		t.Fatalf("canceled icon = %q, want %q", got, want)
	}
}

func TestJobRowLeadingIconDecisionUsesInputRequiredGlyph(t *testing.T) {
	want := "\U000f02d7"
	if got := jobRowLeadingIcon("decision", theme.Theme{}); got != want {
		t.Fatalf("decision icon = %q, want %q", got, want)
	}
}

func TestJobRowLeadingIconQueuedUsesClockGlyph(t *testing.T) {
	want := "\u231B" // ⌛ queued (hourglass)
	if got := jobRowLeadingIcon("queued", theme.Theme{}); got != want {
		t.Fatalf("queued icon = %q, want %q", got, want)
	}
}

func TestJobRowLeadingIconRunningUsesOngoingGlyph(t *testing.T) {
	want := "\uf144"
	if got := jobRowLeadingIcon("running", theme.Theme{}); got != want {
		t.Fatalf("running icon = %q, want %q", got, want)
	}
}

func TestJobRowLeadingIconPausedUsesPausedGlyph(t *testing.T) {
	want := "\uf28b"
	if got := jobRowLeadingIcon("paused", theme.Theme{}); got != want {
		t.Fatalf("paused icon = %q, want %q", got, want)
	}
}
