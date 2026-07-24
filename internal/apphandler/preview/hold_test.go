package preview

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSnapshotPreviewDrawStatesHoldsBodyWhileLoading(t *testing.T) {
	h, _ := newTestHandler(t, 80, 24)

	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhasePending,
		Path:         "/tmp/new.txt",
		TitleBase:    "new.txt",
		CombinedText: "",
	}
	h.mu.Unlock()
	h.filePreviewHold = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         "/tmp/old.txt",
		TitleBase:    "old.txt",
		CombinedText: "previous preview text",
	}

	h.SnapshotPreviewDrawStates()

	draw := h.model.FilePreviewDraw
	if !draw.BodyHeld {
		t.Fatal("FilePreviewDraw.BodyHeld = false, want true")
	}
	if draw.CombinedText != "previous preview text" {
		t.Fatalf("FilePreviewDraw.CombinedText = %q, want held body", draw.CombinedText)
	}
	if draw.TitleBase != "new.txt" {
		t.Fatalf("FilePreviewDraw.TitleBase = %q, want new file title", draw.TitleBase)
	}
}

func TestCaptureFilePreviewHoldFromDonePreview(t *testing.T) {
	h, _ := newTestHandler(t, 80, 24)

	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         "/tmp/old.txt",
		CombinedText: "keep me",
	}
	h.mu.Unlock()

	h.captureFilePreviewHold(previewTargetInactive)

	if h.filePreviewHold.CombinedText != "keep me" {
		t.Fatalf("hold CombinedText = %q, want keep me", h.filePreviewHold.CombinedText)
	}
}
