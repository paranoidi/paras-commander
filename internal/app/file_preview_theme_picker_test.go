package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

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

func TestFilePreviewThemePickerFooterWhileOpen(t *testing.T) {
	app, _ := newFilePreviewThemePickerTestApp(t)
	app.previewCtrl.TryDispatchFileView(keymap.ActionFileViewThemePicker)
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
