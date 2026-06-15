package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestSnapshotPreviewDrawStatesHoldsBodyWhileLoading(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhasePending,
		Path:         "/tmp/new.txt",
		TitleBase:    "new.txt",
		CombinedText: "",
	}
	app.commandsMu.Unlock()
	app.filePreviewHold = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         "/tmp/old.txt",
		TitleBase:    "old.txt",
		CombinedText: "previous preview text",
	}

	app.snapshotPreviewDrawStates()

	draw := app.model.FilePreviewDraw
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
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{
		Open:         true,
		Phase:        ui.FilePreviewPhaseDone,
		Path:         "/tmp/old.txt",
		CombinedText: "keep me",
	}
	app.commandsMu.Unlock()

	app.captureFilePreviewHold(previewTargetInactive)

	if app.filePreviewHold.CombinedText != "keep me" {
		t.Fatalf("hold CombinedText = %q, want keep me", app.filePreviewHold.CombinedText)
	}
}
