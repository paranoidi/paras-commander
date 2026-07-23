package panelcarousel

import (
	"fmt"
	"strings"
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

	cols := SplitColumns(frame, true, DefaultLayout(), [3]int{})
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

		cols := SplitColumns(frame, showChild, DefaultLayout(), [3]int{})
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
	cols := SplitColumns(frame, false, DefaultLayout(), [3]int{})
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

	cols := SplitColumns(frame, true, DefaultLayout(), [3]int{})
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

// TestDrawBodyPaintsBoundariesMatchingSplitColumnsWithMeasuredFitWidth verifies DrawBody
// doesn't compute its own measurement — it must thread BodyParams.MeasuredFitWidth straight
// into its internal SplitColumns call, so the painted column boundaries match a direct
// SplitColumns call given the same MeasuredFitWidth. Uses distinct, hand-picked header colors
// (rather than a named theme) so the parent/center header boundary is unambiguous.
func TestDrawBodyPaintsBoundariesMatchingSplitColumnsWithMeasuredFitWidth(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	layout, err := ParseLayout([]string{"<16", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	measured := [3]int{9, 0, 0}
	frame := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	screen.SetSize(frame.Width, frame.Height)
	styles := theme.Default()
	parentHeaderStyle := tcell.StyleDefault.Background(tcell.ColorRed)
	centerHeaderStyle := tcell.StyleDefault.Background(tcell.ColorBlue)
	entries := []localfs.Entry{{Name: "cat", Path: "/vol/cat", Type: localfs.EntryFile}}
	center := panel.State{Path: pathloc.MustParse("/vol"), Entries: entries}
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}}

	DrawBody(screen, BodyParams{
		Frame:               frame,
		Center:              center,
		Parent:              parent,
		Styles:              styles,
		FileListActive:      true,
		ShowIcons:           false,
		HeaderStyle:         centerHeaderStyle,
		HeaderCarouselStyle: parentHeaderStyle,
		SurfaceStyle:        styles.PanelActiveSurface,
		ShowChildColumn:     false,
		Layout:              layout,
		MeasuredFitWidth:    measured,
	})

	want := SplitColumns(frame, false, layout, measured)
	if want[0].Width != 9 {
		t.Fatalf("test setup: want parent width 9 (measured under 16-cell cap), got %d", want[0].Width)
	}
	headerY := frame.Y + 1
	_, wantParentBG, _ := parentHeaderStyle.Decompose()
	_, wantCenterBG, _ := centerHeaderStyle.Decompose()

	_, lastParentStyle, _ := screen.Get(want[0].X+want[0].Width-1, headerY)
	_, lastParentBG, _ := lastParentStyle.Decompose()
	if lastParentBG != wantParentBG {
		t.Fatalf("parent column last header cell (X=%d) bg = %v, want %v (carousel header style)",
			want[0].X+want[0].Width-1, lastParentBG, wantParentBG)
	}
	_, firstCenterStyle, _ := screen.Get(want[1].X, headerY)
	_, firstCenterBG, _ := firstCenterStyle.Decompose()
	if firstCenterBG != wantCenterBG {
		t.Fatalf("center column first header cell (X=%d) bg = %v, want %v (center header style) — "+
			"DrawBody's SplitColumns call disagrees with the direct SplitColumns call on the same MeasuredFitWidth",
			want[1].X, firstCenterBG, wantCenterBG)
	}
}

// Regression: a fit-mode column ("<20%") whose listing has only very short names (e.g. "pc")
// must still measure wide enough to show the full "Name  Size" header — it must never crop to
// something like "Name  S~".
func TestFitModeColumnHeaderNeverTruncatesWithShortNames(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)

	layout, err := ParseLayout([]string{"<20%", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	frame := geom.Rect{X: 0, Y: 0, Width: 92, Height: 18}
	screen.SetSize(frame.Width, frame.Height)
	styles := theme.Default()
	entries := []localfs.Entry{{Name: "pc", Path: "/vol/pc", Type: localfs.EntryFile}}
	center := panel.State{Path: pathloc.MustParse("/vol"), Entries: entries}
	parent := Column{Kind: ColumnParent, Populated: true, Snapshot: panel.ListingSnapshot{Entries: entries}}
	visibleRows := frame.Height - 2

	measured := MeasureFitColumnWidths(layout, parent, center, false, true, uiscrollbar.StyleThumb, visibleRows)

	DrawBody(screen, BodyParams{
		Frame:                 frame,
		Center:                center,
		Parent:                parent,
		Styles:                styles,
		FileListActive:        true,
		HeaderStyle:           styles.PanelActiveHeader,
		HeaderCarouselStyle:   styles.PanelActiveHeaderCarousel,
		SurfaceStyle:          styles.PanelActiveSurface,
		ScrollbarStyle:        uiscrollbar.StyleThumb,
		ScrollbarShowInactive: true,
		InactiveFrameStyle:    styles.PanelInactiveFrame,
		ShowChildColumn:       false,
		Layout:                layout,
		MeasuredFitWidth:      measured,
	})

	cols := SplitColumns(frame, false, layout, measured)
	headerY := frame.Y + 1
	var sb []rune
	for x := cols[0].X; x < cols[0].X+cols[0].Width; x++ {
		ch, _, _ := screen.Get(x, headerY)
		r, _ := utf8.DecodeRuneInString(ch)
		sb = append(sb, r)
	}
	hdr := string(sb)
	if strings.Contains(hdr, "~") {
		t.Fatalf("parent header %q was truncated with ellipsis, want full %q", hdr, "Name  Size")
	}
	if !strings.Contains(hdr, "Size") {
		t.Fatalf("parent header %q does not contain full %q", hdr, "Size")
	}
}
