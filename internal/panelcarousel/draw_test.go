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

func TestSubtreeSelectionMarkUsesSelectedForeground(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 80, 10
	screen.SetSize(width, height)

	styles := theme.Default()
	wantFG, ok := styles.PanelFileIconFG["panel.active.row.cursor"]
	if !ok {
		t.Fatal("default theme missing panel.active.row.cursor icon")
	}

	root := "/vol"
	parentPath := root + "/parent"
	childPath := parentPath + "/nested.txt"
	center := panel.State{
		Path: pathloc.MustParse(root),
		Entries: []localfs.Entry{
			{Name: "parent", Path: parentPath, Type: localfs.EntryDirectory},
		},
		Cursor:        0,
		SelectedPaths: map[string]bool{childPath: true},
	}

	frame := geom.Rect{X: 0, Y: 0, Width: width, Height: height}
	DrawBody(screen, BodyParams{
		Frame:               frame,
		Center:              center,
		Styles:              styles,
		FileListActive:      true,
		ShowIcons:           false,
		HeaderStyle:         styles.PanelActiveHeader,
		HeaderCarouselStyle: styles.PanelActiveHeaderCarousel,
		SurfaceStyle:        styles.PanelActiveSurface,
		Layout:              DefaultLayout(),
	})

	cols := SplitColumns(frame, true, DefaultLayout())
	centerCol := cols[1]
	rowY := centerCol.Y
	wantMark := styles.SymbolFilelistSelectionSubtree()
	markCol := -1
	for col := centerCol.X; col < centerCol.X+centerCol.Width; col++ {
		ch, _, _ := screen.Get(col, rowY)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == wantMark {
			markCol = col
			break
		}
	}
	if markCol < 0 {
		t.Fatal("subtree selection mark not found on directory row")
	}
	_, markStyle, _ := screen.Get(markCol, rowY)
	gotFG, _, _ := markStyle.Decompose()
	if gotFG != wantFG {
		t.Fatalf("subtree mark foreground = %v, want cursor-row icon %v", gotFG, wantFG)
	}
}

func TestCenterScrollbarUsesInactiveFrameBetweenColumns(t *testing.T) {
	t.Parallel()
	styles := theme.Default()
	wantInactiveFG, _, _ := styles.PanelInactiveFrame.Decompose()

	scrollableEntries := func(withSubdir bool) []localfs.Entry {
		entries := make([]localfs.Entry, 40)
		for i := range entries {
			name := fmt.Sprintf("file-%03d", i)
			entries[i] = localfs.Entry{Name: name, Path: "/vol/" + name, Type: localfs.EntryFile}
		}
		if withSubdir {
			return append([]localfs.Entry{
				{Name: "birch", Path: "/vol/birch", Type: localfs.EntryDirectory},
			}, entries...)
		}
		return entries
	}

	frame := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	centerTrackFG := func(t *testing.T, center panel.State, showChild bool) tcell.Color {
		t.Helper()
		screen := tcell.NewSimulationScreen("UTF-8")
		if err := screen.Init(); err != nil {
			t.Fatalf("Init: %v", err)
		}
		t.Cleanup(screen.Fini)
		screen.SetSize(frame.Width, frame.Height)

		DrawBody(screen, BodyParams{
			Frame:                 frame,
			Center:                center,
			Styles:                styles,
			FileListActive:        true,
			ShowIcons:             false,
			HeaderStyle:           styles.PanelActiveHeader,
			HeaderCarouselStyle:   styles.PanelActiveHeaderCarousel,
			SurfaceStyle:          styles.PanelActiveSurface,
			ShowChildColumn:       showChild,
			ScrollbarStyle:        uiscrollbar.StyleThumb,
			ScrollbarShowInactive: true,
			InactiveFrameStyle:    styles.PanelInactiveFrame,
			Layout:                DefaultLayout(),
		})

		cols := SplitColumns(frame, showChild, DefaultLayout())
		sbX := cols[1].X + cols[1].Width - 1
		listY := cols[1].Y
		for row := 0; row < geom.PanelListRows(frame); row++ {
			ch, style, _ := screen.Get(sbX, listY+row)
			r, _ := utf8.DecodeRuneInString(ch)
			if r == '│' {
				fg, _, _ := style.Decompose()
				return fg
			}
		}
		t.Fatal("center scrollbar track │ not found")
		return tcell.ColorDefault
	}

	filesOnly := panel.State{
		Path:         pathloc.MustParse("/vol"),
		Entries:      scrollableEntries(false),
		Cursor:       20,
		ScrollOffset: 15,
	}
	if ShowChildPreviewColumn(filesOnly, false, false) {
		t.Fatal("files-only fixture should hide child column")
	}
	cols := SplitColumns(frame, false, DefaultLayout())
	sbX := cols[1].X + cols[1].Width - 1
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(frame.Width, frame.Height)
	DrawBody(screen, BodyParams{
		Frame:                 frame,
		Center:                filesOnly,
		Styles:                styles,
		FileListActive:        true,
		ShowIcons:             false,
		HeaderStyle:           styles.PanelActiveHeader,
		HeaderCarouselStyle:   styles.PanelActiveHeaderCarousel,
		SurfaceStyle:          styles.PanelActiveSurface,
		ShowChildColumn:       false,
		ScrollbarStyle:        uiscrollbar.StyleThumb,
		ScrollbarShowInactive: true,
		InactiveFrameStyle:    styles.PanelInactiveFrame,
		Layout:                DefaultLayout(),
	})
	for row := 0; row < geom.PanelListRows(frame); row++ {
		ch, _, _ := screen.Get(sbX, cols[1].Y+row)
		r, _ := utf8.DecodeRuneInString(ch)
		if r == '│' || r == styles.SymbolScrollbarThumb() {
			t.Fatalf("two-column DrawBody should not paint scrollbar at center column edge, got %q", ch)
		}
	}

	withSubdir := panel.State{
		Path:         pathloc.MustParse("/vol"),
		Entries:      scrollableEntries(true),
		Cursor:       20,
		ScrollOffset: 15,
	}
	if !ShowChildPreviewColumn(withSubdir, false, false) {
		t.Fatal("subdir fixture should show child column")
	}
	if got := centerTrackFG(t, withSubdir, true); got != wantInactiveFG {
		t.Fatalf("three-column center track fg = %v, want inactive frame %v", got, wantInactiveFG)
	}
}

func TestCarouselNoScrollbarLaneWhenListFits(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	entries := []localfs.Entry{
		{Name: "birch", Path: "/vol/birch", Type: localfs.EntryDirectory},
		{Name: "cedar.txt", Path: "/vol/cedar.txt", Type: localfs.EntryFile, Size: 1024},
	}
	center := panel.State{
		Path:    pathloc.MustParse("/vol"),
		Entries: entries,
		Cursor:  0,
	}
	frame := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	screen.SetSize(frame.Width, frame.Height)
	styles := theme.Default()
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}}
	child := Column{Kind: ColumnChild, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}}

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

	cols := SplitColumns(frame, true, DefaultLayout())
	col := cols[1]
	reserve := columnScrollbarReserve(
		columnHasScrollbarLane(Column{Kind: ColumnCenter, Populated: true, Active: true}, false, true),
		true,
		uiscrollbar.StyleThumb,
		len(center.Entries),
		geom.PanelListRows(frame),
		center.ScrollOffset,
	)
	if reserve != 0 {
		t.Fatalf("center reserve = %d, want 0 when list fits", reserve)
	}
	listW := columnListTextWidth(col.Width, false, reserve)
	if listW != col.Width {
		t.Fatalf("center list width = %d, want full column width %d", listW, col.Width)
	}
	rowY := col.Y
	rightX := col.X + col.Width - 1
	cell, style, _ := screen.Get(rightX, rowY)
	r, _ := utf8.DecodeRuneInString(cell)
	if r == '│' || r == styles.SymbolScrollbarThumb() {
		t.Fatalf("center column right edge at (%d,%d) is scrollbar %q", rightX, rowY, cell)
	}
	_, surfaceBG, _ := styles.PanelActiveSurface.Decompose()
	_, rightBG, _ := style.Decompose()
	_, innerStyle, _ := screen.Get(rightX-1, rowY)
	_, innerBG, _ := innerStyle.Decompose()
	if rightBG == surfaceBG && innerBG != surfaceBG {
		t.Fatalf("center column right edge looks like an empty scrollbar lane")
	}
}
