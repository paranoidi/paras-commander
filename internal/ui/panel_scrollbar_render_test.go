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

func TestDrawPanelPaintsThumbScrollbarOnBorder(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := "entry-" + string(rune('a'+i%26))
		entries[i] = localfs.Entry{Name: name, Path: "/tmp/" + name, Type: localfs.EntryFile}
	}
	state := panel.State{
		Path:         pathloc.MustParse("/tmp"),
		Entries:      entries,
		Cursor:       20,
		ScrollOffset: 15,
	}
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 12}
	styles := theme.Default()
	wantThumb := styles.SymbolScrollbarThumb()
	drawPanel(screen, rect, state, true, false, styles, false, "", nil, false, nil, false,
		PrimaryPanel, nil, -1, -1, nil, false, false, false, PrimaryPanel, "", false,
		uiscrollbar.StyleThumb, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "", SplitHorizontal)

	borderX := rect.X + rect.Width - 1
	foundThumb := false
	for row := rect.Y + 2; row < rect.Y+2+PanelListRows(rect); row++ {
		cell, _, _ := screen.Get(borderX, row)
		r, _ := utf8.DecodeRuneInString(cell)
		if r == wantThumb {
			foundThumb = true
			break
		}
	}
	if !foundThumb {
		t.Fatalf("expected %q thumb on right border", string(wantThumb))
	}
}

func TestDrawPanelCarouselTwoColumnScrollbarOnBorder(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := fmt.Sprintf("file-%03d", i)
		entries[i] = localfs.Entry{Name: name, Path: "/tmp/" + name, Type: localfs.EntryFile}
	}
	state := panel.State{
		Path:         pathloc.MustParse("/tmp"),
		Entries:      entries,
		Cursor:       20,
		ScrollOffset: 15,
		CarouselMode: true,
	}
	rect := Rect{X: 0, Y: 0, Width: 92, Height: 18}
	screen.SetSize(rect.Width, rect.Height)
	styles := theme.Default()
	wantThumb := styles.SymbolScrollbarThumb()
	drawPanel(screen, rect, state, true, false, styles, false, "", nil, false, nil, false,
		PrimaryPanel, nil, -1, -1, nil, false, false, false, PrimaryPanel, "", false,
		uiscrollbar.StyleThumb, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "", SplitHorizontal)

	borderX := rect.X + rect.Width - 1
	foundThumb := false
	for row := rect.Y + 2; row < rect.Y+2+PanelListRows(rect); row++ {
		cell, _, _ := screen.Get(borderX, row)
		r, _ := utf8.DecodeRuneInString(cell)
		if r == wantThumb {
			foundThumb = true
			break
		}
	}
	if !foundThumb {
		t.Fatalf("expected %q thumb on panel border in two-column carousel", string(wantThumb))
	}
}
