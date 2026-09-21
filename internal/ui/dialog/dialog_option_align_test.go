package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func asciiDialogStyles() theme.Theme {
	styles := theme.Default()
	styles.UseNerdfontIcons = false
	return styles
}

func assertOptionMarkerAtDialogTextX(t *testing.T, screen tcell.SimulationScreen, rect draw.Rect, rowY int) {
	t.Helper()
	textX := draw.DialogTextX(rect)
	ch, _, _ := screen.Get(textX, rowY)
	if ch != "(" && ch != "[" {
		t.Fatalf("marker at DialogTextX(%d) y=%d = %q, want '(' or '['", textX, rowY, ch)
	}
}

func TestSortDialogRadioAlignsWithDialogTextX(t *testing.T) {
	const w, h = 80, 24
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	layout := Layout{Width: w, Height: h}
	DrawSortDialog(screen, layout, SortDialogState{Open: true, SortMode: panel.SortName}, asciiDialogStyles())
	rect, ok := draw.ClampCenteredDialogRect(layout, 56, 11, 30, 11)
	if !ok {
		t.Fatal("sort dialog rect not drawable")
	}
	assertOptionMarkerAtDialogTextX(t, screen, rect, rect.Y+1)
}

func TestListingFormatDialogRadioAlignsWithDialogTextX(t *testing.T) {
	const w, h = 80, 24
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(w, h)
	layout := Layout{Width: w, Height: h}
	DrawListingFormatDialog(screen, layout, ListingFormatDialogState{Open: true}, asciiDialogStyles())
	rect, ok := draw.ClampCenteredDialogRect(layout, 44, 7, 28, 7)
	if !ok {
		t.Fatal("listing format dialog rect not drawable")
	}
	assertOptionMarkerAtDialogTextX(t, screen, rect, rect.Y+1)
}
