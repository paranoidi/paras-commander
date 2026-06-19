package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

func TestEntryDisplayNameTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 40)
	entry := localfs.Entry{Name: long, Path: "/tmp/" + long}
	th := theme.Default()
	if !entryDisplayNameTruncated(entry, 10, true, panellist.RowSuffix{}, th) {
		t.Fatal("expected truncated for narrow name column")
	}
	short := localfs.Entry{Name: "a.txt", Path: "/tmp/a.txt"}
	if entryDisplayNameTruncated(short, 20, true, panellist.RowSuffix{}, th) {
		t.Fatal("expected short name to fit")
	}
}

func TestPanelBottomCenterOverlaySpanSkipsIndicators(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 0, Y: 0, Width: 50, Height: 10}
	ctx := PanelBottomIndicatorContext{
		PanelID:              LeftPanel,
		SelectionsBottomHint: true,
		SyncDriverPanelID:    LeftPanel,
		Styles:               theme.Default(),
	}
	start, end, ok := panelBottomCenterOverlaySpan(rect, LeftPanel, ctx)
	if !ok {
		t.Fatal("expected span")
	}
	selUsed := panelBottomStartEdgeUsedWidth(rect, LeftPanel, ctx)
	syncW := len([]rune(panelSyncIndicatorLabel(LeftPanel)))
	if start < rect.X+1+selUsed {
		t.Fatalf("start %d too close to selections corner (want >= %d)", start, rect.X+1+selUsed)
	}
	lastIn := rect.X + rect.Width - 2
	if end > lastIn-syncW {
		t.Fatalf("end %d overlaps sync (lastIn-syncW=%d)", end, lastIn-syncW)
	}
}

func TestDrawPanelBottomCursorNameHintOnActiveRightPanel(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("r", 30) + ".txt"
	rect := Rect{X: 40, Y: 0, Width: 40, Height: 8}
	state := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
		Cursor:  0,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state, true, false, styles, true, "", nil, false, nil, false, RightPanel, nil, -1, -1, nil, false, false, false, RightPanel, "", false, uiscrollbar.StyleNone, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "")

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("right panel bottom = %q, want full cursor name", bottom)
	}
}

func TestPanelBottomCenterOverlaySpanRightPanelNoIndicators(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 40, Y: 0, Width: 40, Height: 10}
	ctx := PanelBottomIndicatorContext{PanelID: RightPanel, Styles: theme.Default()}
	start, end, ok := panelBottomCenterOverlaySpan(rect, RightPanel, ctx)
	if !ok {
		t.Fatal("expected span")
	}
	firstIn := rect.X + 1
	lastIn := rect.X + rect.Width - 2
	if start != firstIn || end != lastIn {
		t.Fatalf("span [%d,%d], want [%d,%d] with no indicators", start, end, firstIn, lastIn)
	}
}

func TestDrawPanelBottomCursorNameHintOnActivePanel(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("w", 30) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 8}
	state := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
		Cursor:  0,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state, true, false, styles, true, "", nil, false, nil, false, LeftPanel, nil, -1, -1, nil, false, false, false, LeftPanel, "", false, uiscrollbar.StyleNone, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "")

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("bottom = %q, want full cursor name", bottom)
	}
}

func TestDrawPanelBottomCursorNameHintOmittedWhenWiderThanOverlaySpan(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("q", 80) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 8}
	state := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
		Cursor:  0,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state, true, false, styles, true, "", nil, false, nil, false, LeftPanel, nil, LeftPanel, -1, nil, false, false, false, LeftPanel, "", false, uiscrollbar.StyleNone, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "")

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if strings.Contains(bottom, longName) {
		t.Fatalf("bottom = %q, want no hint when full name cannot fit between indicators", bottom)
	}
}

func TestDrawPanelCarouselCursorNameHint(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("v", 30) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 80, Height: 12}
	state := panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "otherdir", Path: "/tmp/otherdir", Type: localfs.EntryDirectory},
			{Name: longName, Path: "/tmp/" + longName, Type: localfs.EntryFile},
		},
		Cursor:       1,
		CarouselMode: true,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state, true, false, styles, true, "", nil, false, nil, false, LeftPanel, nil, -1, -1, nil, false, false, false, LeftPanel, "", false, uiscrollbar.StyleNone, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "")

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("carousel bottom = %q, want full cursor name between indicators", bottom)
	}
}

func TestDrawPanelBottomCursorNameHintHiddenOnInactivePanel(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("z", 30) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 8}
	state := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
		Cursor:  0,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state, false, false, styles, true, "", nil, false, nil, false, LeftPanel, nil, -1, -1, nil, false, false, false, RightPanel, "", false, uiscrollbar.StyleNone, true, panelcarousel.DefaultLayout(), FilePreviewState{}, "")

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if strings.Contains(bottom, longName) {
		t.Fatalf("inactive bottom = %q, want no cursor name hint", bottom)
	}
}
