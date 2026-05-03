package ui

import "testing"

func TestThemeDialogListViewportRows(t *testing.T) {
	t.Parallel()
	layout := Layout{Width: 80, Height: 24}
	if got := ThemeDialogListViewportRows(layout, 100); got != 18 {
		t.Fatalf("tall dialog viewport = %d, want 18", got)
	}
	small := Layout{Width: 80, Height: 10}
	if got := ThemeDialogListViewportRows(small, 100); got != 4 {
		t.Fatalf("clamped dialog viewport = %d, want 4", got)
	}
}
