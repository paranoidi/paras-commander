package ui

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func thumbPaintedRowRange(t *testing.T, screen tcell.Screen, rect Rect, state panel.State, style uiscrollbar.Style) (minRow, maxRow int, ok bool) {
	t.Helper()
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles, ScrollbarStyle: style},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})
	borderX := rect.X + rect.Width - 1
	visible := PanelListRows(rect)
	minRow = visible
	maxRow = -1
	thumbRune := styles.SymbolScrollbarThumb()
	for row := rect.Y + 2; row < rect.Y+2+visible; row++ {
		cell, _, _ := screen.Get(borderX, row)
		r, _ := utf8.DecodeRuneInString(cell)
		if r == '█' || r == thumbRune {
			rrow := row - (rect.Y + 2)
			if rrow < minRow {
				minRow = rrow
			}
			if rrow > maxRow {
				maxRow = rrow
			}
		}
	}
	return minRow, maxRow, maxRow >= 0
}

func TestPanelScrollbarUsesFullListHeight(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(120, 40)

	col := Rect{X: 0, Y: 1, Width: 60, Height: 38}
	rect := FileListFrameWithStripCount(col, SelectionsStripSplitParams{
		StripItemCount:     0,
		MaxRows:            5,
		ActivePercent:      50,
		MinFileContentRows: MinFileListContentRows,
	})
	visible := PanelListRows(rect)
	if visible < 10 {
		t.Fatalf("visible = %d, want reasonable viewport", visible)
	}

	for _, total := range []int{80, 500} {
		t.Run(fmt.Sprintf("total=%d", total), func(t *testing.T) {
			entries := make([]localfs.Entry, total)
			for i := range entries {
				entries[i] = localfs.Entry{
					Name: fmt.Sprintf("entry-%04d", i),
					Path: fmt.Sprintf("/tmp/entry-%04d", i),
					Type: localfs.EntryFile,
				}
			}
			maxOff := total - visible
			for _, style := range []uiscrollbar.Style{uiscrollbar.StyleBar, uiscrollbar.StyleThumb} {
				minSeen, maxSeen := visible, -1
				for off := 0; off <= maxOff; off++ {
					state := panel.State{
						Path:         pathloc.MustParse("/tmp"),
						Entries:      entries,
						ScrollOffset: off,
						Cursor:       off,
					}
					minRow, maxRow, painted := thumbPaintedRowRange(t, screen, rect, state, style)
					if !painted {
						t.Fatalf("style=%s offset=%d: no thumb painted", style, off)
					}
					if minRow < minSeen {
						minSeen = minRow
					}
					if maxRow > maxSeen {
						maxSeen = maxRow
					}
				}
				if minSeen != 0 {
					t.Fatalf("style=%s thumb never reached top row: minSeen=%d want 0", style, minSeen)
				}
				if maxSeen != visible-1 {
					t.Fatalf("style=%s thumb never reached bottom row: maxSeen=%d want %d", style, maxSeen, visible-1)
				}
			}
		})
	}
}
