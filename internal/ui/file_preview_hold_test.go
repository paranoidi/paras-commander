package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFilePreviewHoldable(t *testing.T) {
	t.Parallel()
	if FilePreviewHoldable(FilePreviewState{Open: true, Phase: FilePreviewPhasePending}) {
		t.Fatal("pending preview should not be holdable")
	}
	if !FilePreviewHoldable(FilePreviewState{
		Open: true, Phase: FilePreviewPhaseDone, CombinedText: "line one\n",
	}) {
		t.Fatal("done preview with text should be holdable")
	}
	if !FilePreviewHoldable(FilePreviewState{
		Open: true, Phase: FilePreviewPhaseDone, ErrorMsg: "not a text file",
	}) {
		t.Fatal("done preview with error message should be holdable")
	}
}

func TestMergeFilePreviewDrawWithHoldKeepsBodyWhileLoading(t *testing.T) {
	t.Parallel()
	hold := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		Path:         "/tmp/old.txt",
		TitleBase:    "old.txt",
		CombinedText: "stale body",
	}
	hold.EnsureWrappedLines(10, tcell.StyleDefault)

	live := FilePreviewState{
		Open:      true,
		Phase:     FilePreviewPhasePending,
		Path:      "/tmp/new.txt",
		TitleBase: "new.txt",
	}
	draw := MergeFilePreviewDrawWithHold(live, hold)
	if !draw.BodyHeld {
		t.Fatal("BodyHeld = false, want true")
	}
	if draw.CombinedText != "stale body" {
		t.Fatalf("CombinedText = %q, want stale body from hold", draw.CombinedText)
	}
	if draw.TitleBase != "new.txt" {
		t.Fatalf("TitleBase = %q, want new file title", draw.TitleBase)
	}
	if draw.Path != "/tmp/new.txt" {
		t.Fatalf("Path = %q, want new path", draw.Path)
	}
}

func TestMergeFilePreviewDrawWithHoldNoHoldShowsLoading(t *testing.T) {
	t.Parallel()
	live := FilePreviewState{
		Open:      true,
		Phase:     FilePreviewPhasePending,
		Path:      "/tmp/new.txt",
		TitleBase: "new.txt",
	}
	draw := MergeFilePreviewDrawWithHold(live, FilePreviewState{})
	if draw.BodyHeld {
		t.Fatal("BodyHeld = true without hold, want false")
	}
	if draw.CombinedText != "" {
		t.Fatalf("CombinedText = %q, want empty", draw.CombinedText)
	}
}

func TestMergeFilePreviewDrawWithHoldDoneUsesLive(t *testing.T) {
	t.Parallel()
	hold := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		CombinedText: "stale body",
	}
	live := FilePreviewState{
		Open:         true,
		Phase:        FilePreviewPhaseDone,
		Path:         "/tmp/new.txt",
		CombinedText: "fresh body",
	}
	draw := MergeFilePreviewDrawWithHold(live, hold)
	if draw.BodyHeld {
		t.Fatal("BodyHeld on done live state")
	}
	if draw.CombinedText != "fresh body" {
		t.Fatalf("CombinedText = %q, want live content", draw.CombinedText)
	}
}
