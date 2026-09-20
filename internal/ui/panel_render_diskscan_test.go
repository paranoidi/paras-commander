package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

type panicDiskScanPainter struct{}

func (panicDiskScanPainter) ByteSize(string) (int64, bool)    { return 0, false }
func (panicDiskScanPainter) FileCount(string) (int64, bool)   { return 0, false }
func (panicDiskScanPainter) PendingForPanel(string, int) bool { return false }
func (panicDiskScanPainter) DiskScanBusy() bool               { return false }
func (panicDiskScanPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	panic("DiskScanExcluded must not run when disk-usage metering is off")
}
func (panicDiskScanPainter) IsKnownExcluded(string) bool { return false }

type cacheOnlyDiskScanPainter struct {
	excluded map[string]bool
}

func (cacheOnlyDiskScanPainter) ByteSize(string) (int64, bool)  { return 0, false }
func (cacheOnlyDiskScanPainter) FileCount(string) (int64, bool) { return 0, false }
func (cacheOnlyDiskScanPainter) PendingForPanel(string, int) bool {
	return false
}
func (cacheOnlyDiskScanPainter) DiskScanBusy() bool { return false }
func (cacheOnlyDiskScanPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	panic("DiskScanExcluded must not run during paint")
}
func (p cacheOnlyDiskScanPainter) IsKnownExcluded(path string) bool {
	return p.excluded[path]
}

func TestDrawPanelSkipsDiskScanExcludedWhenMeteringOff(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 12)

	state := panel.State{
		Path: pathloc.MustParse("/mnt/nas"),
		Entries: []localfs.Entry{
			{Name: "dirA", Path: "/mnt/nas/dirA", Type: localfs.EntryDirectory},
			{Name: "file", Path: "/mnt/nas/file", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 12}
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: theme.Default()},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{ShowIcons: true, Painter: panicDiskScanPainter{}, ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout()})
}

func TestDrawPanelUsesCachedExclusionOnlyWhenMeteringOn(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(40, 12)

	excluded := "/mnt/nas/dirA"
	state := panel.State{
		Path: pathloc.MustParse("/mnt/nas"),
		Entries: []localfs.Entry{
			{Name: "dirA", Path: excluded, Type: localfs.EntryDirectory},
			{Name: "file", Path: "/mnt/nas/file", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	rect := Rect{X: 0, Y: 0, Width: 40, Height: 12}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{
			ShowIcons: true, ShowDiskUsage: true,
			Painter:               cacheOnlyDiskScanPainter{excluded: map[string]bool{excluded: true}},
			ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(),
		})
	if !screenHasFolderIcon(screen, rect, styles.FolderIcon(theme.FolderIconExcluded)) {
		t.Fatal("classified exclusion must paint the excluded folder icon")
	}
}

func TestDrawPanelCarouselUsesCachedExclusionOnlyWhenMeteringOn(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const width, height = 100, 16
	screen.SetSize(width, height)

	excluded := "/mnt/nas/dirA"
	state := panel.State{
		Path: pathloc.MustParse("/mnt/nas"),
		Entries: []localfs.Entry{
			{Name: "dirA", Path: excluded, Type: localfs.EntryDirectory},
		},
		Cursor:       0,
		CarouselMode: true,
	}
	rect := Rect{X: 0, Y: 0, Width: width, Height: height}
	styles := theme.Default()
	drawPanel(screen, rect, state,
		PanelStyleConfig{Styles: styles},
		PanelContext{PanelID: PrimaryPanel, FileListActive: true, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{
			ShowIcons: true, ShowDiskUsage: true,
			Painter:               cacheOnlyDiskScanPainter{excluded: map[string]bool{excluded: true}},
			ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(),
		})
	if !screenHasFolderIcon(screen, rect, styles.FolderIcon(theme.FolderIconExcluded)) {
		t.Fatal("classified exclusion must paint the excluded folder icon in carousel")
	}
}

func screenHasFolderIcon(screen tcell.SimulationScreen, rect Rect, icon string) bool {
	want, _ := utf8.DecodeRuneInString(icon)
	if want == utf8.RuneError {
		return false
	}
	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		for x := rect.X; x < rect.X+rect.Width; x++ {
			ch, _, _ := screen.Get(x, y)
			if r, _ := utf8.DecodeRuneInString(ch); r == want {
				return true
			}
		}
	}
	return false
}
