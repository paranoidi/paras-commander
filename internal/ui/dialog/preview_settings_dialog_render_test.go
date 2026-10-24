package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawPreviewSettingsDialogShowsCapabilityNoRadio(t *testing.T) {
	const w, h = 80, 40
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)

	DrawPreviewSettingsDialog(screen, Layout{Width: w, Height: h}, PreviewSettingsDialogState{
		Open:             true,
		Sixel:            config.PreviewTerminalCapabilityNo,
		Kitty:            config.PreviewTerminalCapabilityAuto,
		KittyPlaceholder: config.PreviewTerminalCapabilityYes,
	}, theme.Default())

	// Each group: one row with the label followed by its auto/yes/no radios, selection marked.
	sel := theme.Default().IconDialogRadio(true) + " "
	want := map[string]string{"Sixel": "no", "Kitty": "auto", "Placeholder": "yes"}
	for y := 0; y < h; y++ {
		row := tcelltest.TextAt(screen, 0, y, w)
		label, _, _ := strings.Cut(strings.Trim(row, "│ "), " ")
		v, ok := want[label]
		if !ok {
			continue
		}
		for _, l := range []string{"auto", "yes", "no"} {
			if !strings.Contains(row, l) {
				t.Fatalf("%s row = %q, missing %q", label, strings.TrimSpace(row), l)
			}
		}
		if !strings.Contains(row, sel+v) {
			t.Fatalf("%s row = %q, want %q selected", label, strings.TrimSpace(row), v)
		}
		delete(want, label)
	}
	if len(want) != 0 {
		t.Fatalf("capability groups missing: %v", want)
	}
}
