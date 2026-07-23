package panelcarousel

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestCarouselSizeNotOverlappedByScrollbar(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	const fileSize = int64(62464) // formats as "61K"
	entries := make([]localfs.Entry, 40)
	for i := range entries {
		name := fmt.Sprintf("file-%03d", i)
		entries[i] = localfs.Entry{Name: name, Path: "/vol/" + name, Type: localfs.EntryFile, Size: fileSize}
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
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries, Cursor: 0, Scroll: 0}}
	child := Column{Kind: ColumnChild, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries, Cursor: 0, Scroll: 0}}

	DrawBody(screen, BodyParams{
		Frame:                 frame,
		Center:                center,
		Parent:                parent,
		Child:                 child,
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
		Layout:                DefaultLayout(),
	})

	cols := SplitColumns(frame, true, DefaultLayout(), [3]int{})
	for i, col := range cols {
		sbX := col.X + col.Width - 1
		rowY := col.Y + 5
		sbCell, _, _ := screen.Get(sbX, rowY)
		sbRune, _ := utf8.DecodeRuneInString(sbCell)
		if sbRune != '│' && sbRune != '█' && sbRune != '░' && sbRune != styles.SymbolScrollbarThumb() {
			continue
		}
		sizeCell, _, _ := screen.Get(sbX-1, rowY)
		sizeRune, _ := utf8.DecodeRuneInString(sizeCell)
		if sizeRune != 'K' {
			t.Fatalf("col %d: size suffix at x=%d is %q, want K (scrollbar at x=%d is %q)", i, sbX-1, sizeCell, sbX, sbCell)
		}
	}
}
