package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func TestDrawCompareFilterDialogNoBlankAboveButtons(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	layout := Layout{Width: 80, Height: 24}
	state := CompareFilterDialogState{
		Open:   true,
		Focus:  0,
		Filter: comparepkg.FilterAll,
	}
	DrawCompareFilterDialog(screen, layout, state, theme.Default())

	rect, ok := draw.ClampCenteredDialogRect(layout, 32, 10, 24, 10)
	if !ok {
		t.Fatal("ClampCenteredDialogRect failed")
	}
	// Content: 6 radios at Y+1..Y+6, separator at Y+7, buttons at Y+8 (no blank).
	sepY := rect.Y + 7
	buttonY := rect.Y + 8
	if buttonY != rect.Y+rect.Height-2 {
		t.Fatalf("buttonY = %d, want %d (row above bottom border)", buttonY, rect.Y+rect.Height-2)
	}

	sepCh, _, _ := screen.Get(rect.X, sepY)
	if sepCh != "├" {
		t.Fatalf("separator y=%d left ch=%q, want ├", sepY, sepCh)
	}
	foundBracket := false
	for x := rect.X + 1; x < rect.X+rect.Width-1; x++ {
		ch, _, _ := screen.Get(x, buttonY)
		if ch == "[" {
			foundBracket = true
			break
		}
	}
	if !foundBracket {
		t.Fatalf("button row y=%d missing '[' (blank row above buttons?)", buttonY)
	}
}
