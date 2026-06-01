package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPaintSelectionsStripBottomSizeFrameDashBeforeCorner(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 6)

	rect := Rect{X: 0, Y: 0, Width: 30, Height: 5}
	styles := theme.Default()
	chrome := styles.PanelChrome(true, false)
	drawAuxPanelChrome(screen, rect, panelSelectionsChromePadded, "", true, false, styles)
	endStyle := styles.PanelBottomIndicator(theme.PanelBottomIndicatorKeySelectionSize, true, false)
	paintSelectionsStripBottomSize(screen, rect, "2 items (1 KiB)", endStyle, chrome.Frame)

	y := rect.Y + rect.Height - 1
	lastIn := rect.X + rect.Width - 2
	cornerX := rect.X + rect.Width - 1
	dashR, _, _ := screen.Get(lastIn, y)
	if dashR != "─" {
		t.Fatalf("dash before corner = %q, want '─'", dashR)
	}
	cornerR, _, _ := screen.Get(cornerX, y)
	if cornerR != "┘" {
		t.Fatalf("corner = %q, want '┘'", cornerR)
	}
	// Padded label ends with a trailing space on the column before the frame dash.
	trail, _, _ := screen.Get(lastIn-1, y)
	if trail != " " {
		t.Fatalf("trailing space before dash = %q, want ' '", trail)
	}
	mid, _, _ := screen.Get(lastIn-3, y)
	if mid == "─" || mid == "┘" || mid == "" {
		t.Fatalf("expected size glyphs before trailing space, got %q", mid)
	}
}
