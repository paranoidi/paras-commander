package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
)

type partialDiskUsagePainter struct {
	sizes map[string]int64
}

func (p partialDiskUsagePainter) ByteSize(path string) (int64, bool) {
	if p.sizes == nil {
		return 0, false
	}
	n, ok := p.sizes[path]
	return n, ok
}

func (partialDiskUsagePainter) FileCount(string) (int64, bool) { return 0, false }
func (partialDiskUsagePainter) PendingForPanel(string, int) bool { return false }
func (partialDiskUsagePainter) DiskScanBusy() bool               { return true }
func (partialDiskUsagePainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}

func TestPaintDiskUsageBrowserPanelsOnlyScopedPanel(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const w, h = 80, 24
	screen.SetSize(w, h)

	root := "/scan/root"
	scanned := root + "/alpha"
	model := Model{
		ViewMode:       ViewBrowser,
		ActivePanel:    PrimaryPanel,
		ActiveSubFocus: SubFocusFileList,
		Primary: panel.State{
			Path: pathloc.MustParse(root),
			Entries: []localfs.Entry{
				{Name: "alpha", Path: scanned, Type: localfs.EntryDirectory},
				{Name: "notes.txt", Path: root + "/notes.txt", Type: localfs.EntryFile, Size: 12},
			},
			Cursor: 0,
		},
		Secondary: panel.State{
			Path: pathloc.MustParse("/elsewhere"),
			Entries: []localfs.Entry{
				{Name: "beta", Path: "/elsewhere/beta", Type: localfs.EntryDirectory},
			},
		},
		DiskUsageShown:      true,
		DiskUsagePanelID:    PrimaryPanel,
		DiskUsageScanOrigin: root,
		DiskUsageScanRoots:  []string{scanned, root + "/notes.txt"},
		DiskUsage: partialDiskUsagePainter{
			sizes: map[string]int64{scanned: 4096},
		},
		MenuBarActivitySpinner: true,
		SpinPhase:              1,
	}
	styles := theme.Default()
	layout := CalculateLayoutWithOrientation(w, h, true, PanelPaneSplit{
		ActivePanel:     PrimaryPanel,
		ActivePercent:   50,
		InactivePercent: 50,
	}, SplitHorizontal)

	Render(screen, model, styles)
	secondaryBefore := hashScreenRegion(screen, layout.Secondary)
	primaryBefore := hashScreenRegion(screen, layout.Primary)

	model.DiskUsage = partialDiskUsagePainter{
		sizes: map[string]int64{scanned: 8192},
	}
	model.SpinPhase = 2

	if !PaintDiskUsageBrowserPanelsOnly(screen, layout, model, styles) {
		t.Fatal("expected partial disk-usage paint to succeed")
	}

	if got := hashScreenRegion(screen, layout.Secondary); got != secondaryBefore {
		t.Fatal("secondary panel outside scan scope should be unchanged")
	}
	if got := hashScreenRegion(screen, layout.Primary); got == primaryBefore {
		t.Fatal("primary panel in scan scope should be repainted")
	}
}

func TestPaintBrowserListNavPanelOnlySkipsOtherColumn(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	const w, h = 80, 24
	screen.SetSize(w, h)

	model := Model{
		ViewMode:       ViewBrowser,
		ActivePanel:    SecondaryPanel,
		ActiveSubFocus: SubFocusFileList,
		Primary: panel.State{
			Path: pathloc.MustParse("/nas/share"),
			Entries: []localfs.Entry{
				{Name: "slowdir", Path: "/nas/share/slowdir", Type: localfs.EntryDirectory},
			},
		},
		Secondary: panel.State{
			Path: pathloc.MustParse("/local/home"),
			Entries: []localfs.Entry{
				{Name: "alpha", Path: "/local/home/alpha", Type: localfs.EntryDirectory},
				{Name: "beta", Path: "/local/home/beta", Type: localfs.EntryDirectory},
			},
			Cursor: 1,
		},
	}
	styles := theme.Default()
	layout := CalculateLayoutWithOrientation(w, h, true, PanelPaneSplit{
		ActivePanel:     SecondaryPanel,
		ActivePercent:   50,
		InactivePercent: 50,
	}, SplitHorizontal)

	Render(screen, model, styles)
	primaryBefore := hashScreenRegion(screen, layout.Primary)
	model.Secondary.Cursor = 0
	if !PaintBrowserListNavPanelOnly(screen, layout, model, styles, SecondaryPanel) {
		t.Fatal("expected list-nav partial paint to succeed")
	}
	if got := hashScreenRegion(screen, layout.Primary); got != primaryBefore {
		t.Fatal("inactive NAS column should be unchanged during active-panel list nav paint")
	}
}

func TestPaintDiskUsageBrowserPanelsOnlyRejectsNonBrowser(t *testing.T) {
	t.Parallel()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)
	layout := CalculateLayoutWithOrientation(80, 24, true, PanelPaneSplit{}, SplitHorizontal)
	model := Model{ViewMode: ViewJobs}
	if PaintDiskUsageBrowserPanelsOnly(screen, layout, model, theme.Default()) {
		t.Fatal("expected false for non-browser view")
	}
}

func hashScreenRegion(screen tcell.Screen, rect Rect) uint64 {
	if rect.Width <= 0 || rect.Height <= 0 {
		return 0
	}
	hsh := uint64(14695981039346656037)
	for y := rect.Y; y < rect.Y+rect.Height; y++ {
		for x := rect.X; x < rect.X+rect.Width; x++ {
			str, sty, width := screen.Get(x, y)
			hsh ^= uint64(width)
			hsh *= 1099511628211
			for i := 0; i < len(str); i++ {
				hsh ^= uint64(str[i])
				hsh *= 1099511628211
			}
			fg, bg, attr := sty.Decompose()
			hsh ^= uint64(fg)
			hsh *= 1099511628211
			hsh ^= uint64(bg)
			hsh *= 1099511628211
			hsh ^= uint64(attr)
			hsh *= 1099511628211
		}
	}
	return hsh
}
