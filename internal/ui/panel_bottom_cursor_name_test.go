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
		PanelID:              PrimaryPanel,
		SelectionsBottomHint: true,
		SyncDriverPanelID:    PrimaryPanel,
		Styles:               theme.Default(),
	}
	start, end, ok := panelBottomCenterOverlaySpan(rect, PrimaryPanel, ctx)
	if !ok {
		t.Fatal("expected span")
	}
	selUsed := panelBottomStartEdgeUsedWidth(rect, PrimaryPanel, ctx)
	syncW := len([]rune(panelSyncIndicatorLabel(PrimaryPanel, SplitHorizontal)))
	if start < rect.X+1+selUsed {
		t.Fatalf("start %d too close to selections corner (want >= %d)", start, rect.X+1+selUsed)
	}
	lastIn := rect.X + rect.Width - 2
	if end > lastIn-syncW {
		t.Fatalf("end %d overlaps sync (lastIn-syncW=%d)", end, lastIn-syncW)
	}
}

func TestDrawPanelBottomCursorNameHintOnActiveSecondaryPanel(t *testing.T) {
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
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: SecondaryPanel, FileListActive: true, ActivePanel: SecondaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("right panel bottom = %q, want full cursor name", bottom)
	}
}

func TestPanelBottomCenterOverlaySpanSecondaryPanelNoIndicators(t *testing.T) {
	t.Parallel()
	rect := Rect{X: 40, Y: 0, Width: 40, Height: 10}
	ctx := PanelBottomIndicatorContext{PanelID: SecondaryPanel, Styles: theme.Default()}
	start, end, ok := panelBottomCenterOverlaySpan(rect, SecondaryPanel, ctx)
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
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("bottom = %q, want full cursor name", bottom)
	}
}

func TestDrawPanelBottomCursorNameHintFallsBackWhenWiderThanOverlaySpan(t *testing.T) {
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
	var fallback CursorNameHintFallback
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{
			PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel,
			SyncDriverPanelID: PrimaryPanel, QuickViewDriverPanelID: -1,
			CursorNameHintFallbackOut: &fallback,
		},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if strings.Contains(bottom, longName) {
		t.Fatalf("bottom = %q, want no hint on border when full name cannot fit between indicators", bottom)
	}
	if fallback.FullName == "" || !strings.Contains(fallback.FullName, longName) {
		t.Fatalf("fallback = %q, want full cursor name for screen row", fallback.FullName)
	}
}

func TestRenderDrawsCursorNameHintAboveFooterWhenTooWideForBorder(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	longName := strings.Repeat("m", 70) + ".txt"
	model := Model{
		Primary: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
			Cursor:  0,
		},
		Secondary:        panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:      PrimaryPanel,
		UseNerdfontIcons: true,
	}
	Render(screen, model, theme.Default())

	statusRow := layoutStatusMessageRowY(height)
	row := tcelltest.TextAt(screen, 0, statusRow, width)
	if !strings.Contains(row, longName) {
		t.Fatalf("row above footer = %q, want full cursor name centered on screen", row)
	}
	bottomY := height - 1
	bottom := tcelltest.TextAt(screen, 0, bottomY, width)
	if strings.Contains(bottom, longName) {
		t.Fatalf("footer row = %q, want cursor name above footer not on footer", bottom)
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
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, longName) {
		t.Fatalf("carousel bottom = %q, want full cursor name between indicators", bottom)
	}
}

func TestPaintPanelBottomCursorNameOverlayClearsStaleLongerName(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	styles := theme.Default()
	chrome := styles.PanelChrome(true, false)
	const y = 0
	startX, endX := 1, 38
	longName := " " + strings.Repeat("w", 30) + ".txt"
	shortName := " " + strings.Repeat("a", 8) + ".txt"

	if !paintPanelBottomCursorNameOverlay(screen, startX, endX, y, longName, chrome.Title, chrome.Frame) {
		t.Fatal("expected long name to fit in overlay span")
	}
	if !paintPanelBottomCursorNameOverlay(screen, startX, endX, y, shortName, chrome.Title, chrome.Frame) {
		t.Fatal("expected short name to fit in overlay span")
	}

	got := tcelltest.TextAt(screen, startX, y, endX-startX+1)
	if strings.Contains(got, strings.Repeat("w", 8)) {
		t.Fatalf("overlay = %q, want no stale long-name glyphs", got)
	}
	if !strings.Contains(got, strings.TrimSpace(shortName)) {
		t.Fatalf("overlay = %q, want short name", got)
	}
	shortIdx := strings.Index(got, strings.TrimSpace(shortName))
	if shortIdx < 0 {
		t.Fatal("short name not found")
	}
	after := got[shortIdx+len(strings.TrimSpace(shortName)):]
	if strings.Trim(after, "─ ") != "" {
		t.Fatalf("after short name = %q, want border dashes only", after)
	}
}

func TestDrawPanelBottomCursorNameHintKeepsPinnedDuringCoalesce(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	oldName := strings.Repeat("o", 30) + ".txt"
	newName := strings.Repeat("n", 30) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 8}
	pinned := " " + oldName
	state := panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: oldName, Path: "/tmp/" + oldName},
			{Name: newName, Path: "/tmp/" + newName},
		},
		Cursor:                 1,
		CursorNameHintCoalesce: true,
		CursorNameHintPinned:   pinned,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{
			PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel,
			SyncDriverPanelID: -1, QuickViewDriverPanelID: -1,
			CursorNameHintPinnedOut: &pinned,
		},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if !strings.Contains(bottom, oldName) {
		t.Fatalf("bottom = %q, want pinned old name during coalesce", bottom)
	}
	if strings.Contains(bottom, newName) {
		t.Fatalf("bottom = %q, want pinned old name not current cursor name during coalesce", bottom)
	}
}

func TestDrawPanelBottomCursorNameHintLatchesPinnedWhenSettled(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()

	longName := strings.Repeat("p", 30) + ".txt"
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 8}
	var pinned string
	state := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: longName, Path: "/tmp/" + longName}},
		Cursor:  0,
	}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{
			PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel,
			SyncDriverPanelID: -1, QuickViewDriverPanelID: -1,
			CursorNameHintPinnedOut: &pinned,
		},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	want := " " + longName
	if pinned != want {
		t.Fatalf("pinned = %q, want %q", pinned, want)
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
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, ActivePanel: SecondaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})

	bottomY := rect.Y + rect.Height - 1
	bottom := tcelltest.TextAt(screen, rect.X, bottomY, rect.Width)
	if strings.Contains(bottom, longName) {
		t.Fatalf("inactive bottom = %q, want no cursor name hint", bottom)
	}
}
