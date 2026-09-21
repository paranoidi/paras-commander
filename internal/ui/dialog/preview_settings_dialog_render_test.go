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

	var foundSixelNo, foundKittyAuto, foundPlaceholderYes bool
	for y := 0; y < h; y++ {
		row := tcelltest.TextAt(screen, 0, y, w)
		if strings.Contains(row, "Sixel no") {
			foundSixelNo = true
			if !strings.Contains(row, theme.Default().IconDialogRadio(true)) && !strings.Contains(row, "*") {
				t.Fatalf("Sixel no row = %q, want a selected radio marker", strings.TrimSpace(row))
			}
		}
		if strings.Contains(row, "Kitty auto") {
			foundKittyAuto = true
		}
		if strings.Contains(row, "Placeholder yes") {
			foundPlaceholderYes = true
		}
	}
	if !foundSixelNo || !foundKittyAuto || !foundPlaceholderYes {
		t.Fatalf("capability radio labels missing: sixel-no=%v kitty-auto=%v placeholder-yes=%v",
			foundSixelNo, foundKittyAuto, foundPlaceholderYes)
	}
}
