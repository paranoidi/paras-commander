package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestSplitFullscreenPreviewRectsClosedUsesFullUnion(t *testing.T) {
	t.Parallel()
	union := Rect{X: 0, Y: 1, Width: 80, Height: 20}
	choices := []dialog.ThemeChoice{{Name: "default", Label: "Default"}}
	preview, picker := SplitFullscreenPreviewRects(union, false, choices)
	if preview != union {
		t.Fatalf("preview = %+v, want full union %+v", preview, union)
	}
	if picker != (Rect{}) {
		t.Fatalf("picker = %+v, want zero rect when closed", picker)
	}
}

func TestSplitFullscreenPreviewRectsOpenReservesRightColumn(t *testing.T) {
	t.Parallel()
	union := Rect{X: 0, Y: 1, Width: 80, Height: 20}
	choices := []dialog.ThemeChoice{
		{Name: "default", Label: "Default"},
		{Name: "test-theme", Label: "Test Theme"},
	}
	preview, picker := SplitFullscreenPreviewRects(union, true, choices)
	if preview.Width < union.Width/2 {
		t.Fatalf("preview width = %d, want at least half of union", preview.Width)
	}
	if picker.Width < filePreviewThemePickerMinWidth {
		t.Fatalf("picker width = %d, want >= %d", picker.Width, filePreviewThemePickerMinWidth)
	}
	if preview.X != union.X || preview.Y != union.Y || preview.Height != union.Height {
		t.Fatalf("preview rect = %+v, want same origin/height as union", preview)
	}
	if picker.X+picker.Width != union.X+union.Width {
		t.Fatalf("picker does not align to union right edge: picker=%+v union=%+v", picker, union)
	}
}
