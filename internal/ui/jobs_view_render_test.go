package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
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

func TestJobDetailLinesOmitDestinationForDelete(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	del := JobEntry{
		ID:      "del",
		Type:    string(jobs.TypeDelete),
		Status:  "running",
		Sources: []string{"/tmp/remove-me"},
	}
	copyJob := JobEntry{
		ID:          "cp",
		Type:        string(jobs.TypeCopy),
		Status:      "running",
		Sources:     []string{"/tmp/a"},
		Destination: "/tmp/b",
	}
	delLines := detailStaticLines(del, now, 80, "", false)
	for _, line := range delLines {
		if strings.Contains(line, "Destination:") {
			t.Fatalf("delete job details must not include destination row; got %q", line)
		}
	}
	copyLines := detailStaticLines(copyJob, now, 80, "", false)
	found := false
	for _, line := range copyLines {
		if strings.Contains(line, "Destination:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("copy job details should include destination row")
	}
}

func TestJobDetailProgressLineDeleteOmitsByteRatio(t *testing.T) {
	t.Parallel()
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	delNoBytes := JobEntry{
		ID:         "del",
		Type:       string(jobs.TypeDelete),
		Status:     "running",
		DoneFiles:  2,
		TotalFiles: 5,
	}
	line := jobDetailProgressLine(delNoBytes)
	if strings.Contains(line, "bytes") || strings.Contains(line, "deleted") {
		t.Fatalf("delete with no DoneBytes should not show byte suffix; got %q", line)
	}
	if !strings.Contains(line, "2 / 5 items") {
		t.Fatalf("expected item progress; got %q", line)
	}

	delWithBytes := JobEntry{
		ID:         "del2",
		Type:       string(jobs.TypeDelete),
		Status:     "completed",
		DoneFiles:  5,
		TotalFiles: 5,
		DoneBytes:  19_000_000_000,
		TotalBytes: 0,
	}
	line = jobDetailProgressLine(delWithBytes)
	if strings.Contains(line, "/ 0") {
		t.Fatalf("delete must not show zero total bytes; got %q", line)
	}
	if !strings.Contains(line, "deleted") {
		t.Fatalf("expected deleted byte label; got %q", line)
	}

	copyLine := jobDetailProgressLine(JobEntry{
		Type:       string(jobs.TypeCopy),
		Status:     "running",
		DoneFiles:  1,
		TotalFiles: 10,
		DoneBytes:  500,
		TotalBytes: 1000,
	})
	if !strings.Contains(copyLine, "500 / 1000 bytes") {
		t.Fatalf("copy should keep byte ratio; got %q", copyLine)
	}

	lines := detailStaticLines(delWithBytes, now, 80, "", false)
	for _, l := range lines {
		if strings.Contains(l, "/ 0") && strings.Contains(l, "bytes") {
			t.Fatalf("detailStaticLines must not show / 0 bytes for delete; got %q", l)
		}
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
