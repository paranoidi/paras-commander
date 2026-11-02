package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func testLayout(w, h int) Layout {
	return Layout{Width: w, Height: h}
}

// fileDialogFrame is the frame DrawFileDialog paints for state on a layout-sized screen (ok
// false when it paints nothing).
func fileDialogFrame(layout Layout, state FileDialogState, deleteIconLead int) (Rect, bool) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		return Rect{}, false
	}
	defer screen.Fini()
	screen.SetSize(layout.Width, layout.Height)
	area := DrawFileDialog(screen, layout, state, DialogRenderContext{IconLead: deleteIconLead}, nil)
	return area.Frame, area.Frame != Rect{}
}

// The painted frame is part of the area the app compares to decide whether a partial file
// dialog repaint is valid. These tests pin the two properties that guard depends on: (1)
// typing in a simple field must not move the frame (so plain typing takes the cheap partial
// path), and (2) a typing-driven resize (mass-rename preview/hint rows) must change it (so
// the keystroke falls back to a full render that clears cells outside the now-smaller frame).

func TestFileDialogFrameStableAcrossFieldValueLength(t *testing.T) {
	layout := testLayout(120, 40)
	base := FileDialogState{
		Open:       true,
		DialogType: FileDialogRename,
		Fields:     []FileDialogField{{Label: "Name", Value: strings.Repeat("a", 3)}},
	}
	short, ok := fileDialogFrame(layout, base, 0)
	if !ok {
		t.Fatal("expected drawable rect for open rename dialog")
	}
	base.Fields[0].Value = strings.Repeat("b", 40)
	long, ok := fileDialogFrame(layout, base, 0)
	if !ok {
		t.Fatal("expected drawable rect for open rename dialog (long value)")
	}
	if short != long {
		t.Fatalf("rect changed with field value length: short=%+v long=%+v", short, long)
	}
	// A name too long for the fixed width switches rename to the 80%-wide mode;
	// the rect change makes the keystroke fall back to a full render.
	base.Fields[0].Value = strings.Repeat("c", 500)
	wide, ok := fileDialogFrame(layout, base, 0)
	if !ok {
		t.Fatal("expected drawable rect for open rename dialog (overlong value)")
	}
	if wide.Width != WideDialogWidth(layout.Width) {
		t.Fatalf("overlong value width = %d, want %d (80%% of terminal)", wide.Width, WideDialogWidth(layout.Width))
	}
}

func TestFileDialogFrameChangesWithMassRenamePreviewRows(t *testing.T) {
	layout := testLayout(120, 40)
	base := FileDialogState{
		Open:           true,
		DialogType:     FileDialogMassRename,
		MassRenameMode: MassRenameModeUISimple,
		Fields: []FileDialogField{
			{Label: "Find", Value: "a"},
			{Label: "Replace", Value: "b"},
		},
		MassRenamePreviewBefore: []string{"one"},
		MassRenamePreviewAfter:  []string{"one2"},
	}
	few, ok := fileDialogFrame(layout, base, 0)
	if !ok {
		t.Fatal("expected drawable rect for mass rename dialog")
	}
	base.MassRenamePreviewBefore = []string{"one", "two", "three", "four", "five"}
	base.MassRenamePreviewAfter = []string{"one2", "two2", "three2", "four2", "five2"}
	many, ok := fileDialogFrame(layout, base, 0)
	if !ok {
		t.Fatal("expected drawable rect for mass rename dialog (more rows)")
	}
	if few.Height == many.Height {
		t.Fatalf("mass-rename rect height did not grow with preview rows: few=%+v many=%+v", few, many)
	}
}

func TestFileDialogFrameNotDrawableWhenClosed(t *testing.T) {
	layout := testLayout(120, 40)
	if _, ok := fileDialogFrame(layout, FileDialogState{Open: false, DialogType: FileDialogRename}, 0); ok {
		t.Fatal("expected ok=false for closed dialog")
	}
}
