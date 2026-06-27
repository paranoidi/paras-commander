package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestSyncFilePreviewThemePickerRanksFiltersLabels(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{
		Open: true,
		Choices: []dialog.ThemeChoice{
			{Name: "monokai", Label: "monokai"},
			{Name: "github", Label: "github"},
		},
		DisplayLines: []string{"monokai", "github"},
		Query:        "git",
	}
	app.syncFilePreviewThemePickerRanks()
	if len(app.model.FilePreviewThemePicker.Ranked) != 1 {
		t.Fatalf("ranked len = %d, want 1 match for query test", len(app.model.FilePreviewThemePicker.Ranked))
	}
	if idx := app.model.FilePreviewThemePicker.Ranked[0]; idx != 1 {
		t.Fatalf("ranked[0] = %d, want index 1 (github)", idx)
	}
}

func TestFilePreviewOverlayMapsF9ToThemePicker(t *testing.T) {
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	id, ok := bundle.FilePreview.Lookup(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if !ok || id != keymap.ActionFileViewThemePicker {
		t.Fatalf("FilePreview.Lookup(F9) = %q %v, want %s", id, ok, keymap.ActionFileViewThemePicker)
	}
}

func TestOpenFilePreviewThemePickerPopulatesChromaStyles(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.openFilePreviewThemePicker()
	st := app.model.FilePreviewThemePicker
	if len(st.Choices) < 10 {
		t.Fatalf("Choices len=%d, want many Chroma styles", len(st.Choices))
	}
	if len(st.DisplayLines) != len(st.Choices) {
		t.Fatalf("DisplayLines len=%d Choices len=%d", len(st.DisplayLines), len(st.Choices))
	}
	for i, c := range st.Choices {
		if c.Name == "" || c.Label != c.Name {
			t.Fatalf("choice[%d] = %+v, want Name==Label", i, c)
		}
		if c.Name == "default" {
			t.Fatal("app UI theme name default must not appear in picker")
		}
		if st.DisplayLines[i] != c.Label {
			t.Fatalf("DisplayLines[%d]=%q want Label %q", i, st.DisplayLines[i], c.Label)
		}
	}
}

func TestFilePreviewThemePickerFooterWhileOpen(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.openFilePreviewThemePicker()
	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("footer len = %d, want Esc Close + Enter Save + F10 Quit", len(keys))
	}
	if keys[0].Hint != "Close" || keys[0].Key != tcell.KeyEsc {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyEnter || keys[1].KeyLabel != "Enter" || keys[1].Hint != "Save" {
		t.Fatalf("footer[1] = %+v, want Enter Save", keys[1])
	}
	if keys[2].Hint != "Quit" || keys[2].Key != tcell.KeyF10 {
		t.Fatalf("footer[2] = %+v, want F10 Quit", keys[2])
	}
	for _, fk := range keys {
		if fk.Key == tcell.KeyF4 || fk.Key == tcell.KeyF9 {
			t.Fatalf("footer must not show F4/F9 while picker open: %+v", keys)
		}
	}
}

func TestPreviewStylePickerDebounceDefersRefreshUntilFlush(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.config.UI.KeyRepeatDebounceMS = 500
	app.openFilePreviewThemePicker()

	genAfterOpen := app.filePreviewRunGen.Load()
	styleAfterOpen := app.config.Preview.Style
	app.handleFilePreviewThemePickerKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	if app.filePreviewRunGen.Load() != genAfterOpen {
		t.Fatal("debounced style change should not start preview immediately")
	}
	if app.config.Preview.Style == styleAfterOpen {
		t.Fatalf("preview.style still %q after Down, want new selection", styleAfterOpen)
	}
	if !app.applyPreviewStylePickerFlush(previewStylePickerFlushPayload{gen: app.previewStylePickerDebounceGen.Load()}) {
		t.Fatal("applyPreviewStylePickerFlush should run deferred preview")
	}
	if app.filePreviewRunGen.Load() == genAfterOpen {
		t.Fatal("flush should start preview refresh")
	}
}
