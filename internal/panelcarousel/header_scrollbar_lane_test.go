package panelcarousel

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestCarouselHeaderFillsScrollbarLane(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := fmt.Sprintf("file-%03d", i)
		entries[i] = localfs.Entry{Name: name, Path: "/vol/" + name, Type: localfs.EntryFile, Size: 62464}
	}
	center := panel.State{
		Path: pathloc.MustParse("/vol"),
		Entries: append([]localfs.Entry{
			{Name: "birch", Path: "/vol/birch", Type: localfs.EntryDirectory},
		}, entries...),
		Cursor:       20,
		ScrollOffset: 15,
	}
	frame := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	screen.SetSize(frame.Width, frame.Height)
	styles := theme.Default()

	DrawBody(screen, BodyParams{
		Frame:                 frame,
		Center:                center,
		Parent:                Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}},
		Child:                 Column{Kind: ColumnChild, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}},
		Styles:                styles,
		FileListActive:        true,
		ShowIcons:             false,
		HeaderStyle:           styles.PanelActiveHeader,
		HeaderCarouselStyle:   styles.PanelActiveHeaderCarousel,
		SurfaceStyle:          styles.PanelActiveSurface,
		ShowChildColumn:       true,
		ScrollbarStyle:        uiscrollbar.StyleThumb,
		ScrollbarShowInactive: true,
		InactiveFrameStyle:    styles.PanelInactiveFrame,
	})

	headerY := frame.Y + 1
	cols := SplitColumns(frame, true)
	for i, col := range cols {
		sbX := col.X + col.Width - 1
		wantStyle := styles.PanelActiveHeaderCarousel
		if i == 1 {
			wantStyle = styles.PanelActiveHeader
		}
		_, wantBG, _ := wantStyle.Decompose()
		_, gotStyle, _ := screen.Get(sbX, headerY)
		_, gotBG, _ := gotStyle.Decompose()
		if gotBG != wantBG {
			t.Fatalf("col %d header scrollbar lane at x=%d bg=%v, want header bg %v", i, sbX, gotBG, wantBG)
		}
	}
}
