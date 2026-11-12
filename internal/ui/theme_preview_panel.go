package ui

import (
	"io/fs"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// themeSamplePanelDir is the directory the sample panel lists; nothing under it exists.
const themeSamplePanelDir = "/srv/meadow"

// themeSamplePanelDiskPainter is a DiskUsagePainter over fixed data: sizes for two folders, one
// folder still being scanned and one the scan skips.
type themeSamplePanelDiskPainter struct{ sizes map[string]int64 }

func (p themeSamplePanelDiskPainter) ByteSize(path string) (int64, bool) {
	n, ok := p.sizes[path]
	return n, ok
}
func (themeSamplePanelDiskPainter) FileCount(string) (int64, bool) { return 0, false }
func (themeSamplePanelDiskPainter) PendingForPanel(path string, _ int) bool {
	return path == themeSamplePanelDir+"/lantern"
}
func (p themeSamplePanelDiskPainter) SizePendingDue(path string, id int) bool {
	return p.PendingForPanel(path, id)
}
func (themeSamplePanelDiskPainter) DiskScanBusy() bool { return true }
func (themeSamplePanelDiskPainter) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}
func (themeSamplePanelDiskPainter) IsKnownExcluded(path string) bool {
	return path == themeSamplePanelDir+"/quarry"
}

// themeSamplePanel returns the sample panel's state, pinned paths and job marks: fixed entries
// (no disk access, no clock) that between them show the row states a listing can have.
func themeSamplePanel() (panel.State, map[string]struct{}, []JobPathMark, themeSamplePanelDiskPainter) {
	at := func(day int) time.Time { return time.Date(2026, time.March, day, 9, 30, 0, 0, time.UTC) }
	p := func(name string) string { return themeSamplePanelDir + "/" + name }
	dir := func(name string) localfs.Entry {
		return localfs.Entry{Name: name, Path: p(name), Type: localfs.EntryDirectory, Mode: fs.ModeDir | 0o755, ModifiedAt: at(3)}
	}
	file := func(name string, size int64, mode fs.FileMode) localfs.Entry {
		return localfs.Entry{Name: name, Path: p(name), Type: localfs.EntryFile, Size: size, Mode: mode, ModifiedAt: at(5)}
	}
	link := func(name string, broken bool) localfs.Entry {
		return localfs.Entry{Name: name, Path: p(name), Type: localfs.EntrySymlink, Mode: fs.ModeSymlink | 0o777, ModifiedAt: at(4), BrokenLink: broken}
	}
	denied := file("secret.key", 1700, 0)
	denied.AccessDenied = true
	parent := dir("..")
	parent.Path = "/srv"
	entries := []localfs.Entry{
		parent, dir("harbor"), dir("lantern"), dir("quarry"),
		link("current", false), link("stale", true),
		file("main.go", 8420, 0o644), file("notes.md", 2310, 0o644), file("photo.png", 482113, 0o644),
		file("archive.zip", 7340032, 0o644), file("deploy.sh", 940, 0o755), file("cache.log", 51200, 0o644),
		denied, file("todo.txt", 312, 0o644), file("readme.md", 1830, 0o644),
	}
	root := pathloc.MustParse(themeSamplePanelDir)
	st := panel.State{
		Path: root, Entries: entries, Columns: panel.ColumnsModified,
		Cursor:          8, // photo.png, which is also selected
		GitColumnActive: true,
		GitByPath: map[string]gitstatus.Cell{
			p("main.go"):   {Unstaged: gitstatus.Modified},
			p("notes.md"):  {Unstaged: gitstatus.New},
			p("todo.txt"):  {Staged: gitstatus.New},
			p("cache.log"): {Unstaged: gitstatus.Ignored},
			p("harbor"):    {Unstaged: gitstatus.Modified},
		},
	}
	st.AddSelection(p("notes.md"))
	st.AddSelection(p("photo.png"))
	st.AddNewFileMarks(root, []string{"todo.txt"})
	st.AddRenameMarks(root, []string{"main.go"})
	marks := []JobPathMark{
		{ID: "1", Type: string(jobs.TypeMove), Status: string(jobs.StatusRunning), Sources: []string{p("archive.zip")}, Destination: "/srv/orchard", DestIsDir: true},
		{ID: "2", Type: string(jobs.TypeCopy), Status: string(jobs.StatusQueued), Sources: []string{p("deploy.sh")}, Destination: "/srv/orchard", DestIsDir: true},
		{ID: "3", Type: string(jobs.TypeDelete), Status: string(jobs.StatusRunning), Sources: []string{p("cache.log")}},
	}
	disk := themeSamplePanelDiskPainter{sizes: map[string]int64{p("harbor"): 3 << 30, p("current"): 0}}
	return st, map[string]struct{}{p("readme.md"): {}}, marks, disk
}

// drawThemeSamplePanel draws the sample listing with drawPanel, the routine the app's panels
// use, at x, y, width wide: as the active panel, then below it a short window of the same listing
// as the inactive panel (its cursor on an unselected file next to selected ones). A frame is cut
// short (closed early) above maxY.
func drawThemeSamplePanel(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme, model Model) {
	activeRows := dialog.ThemePanelRows - dialog.ThemeInactivePanelRows
	drawThemeSampleListing(screen, Rect{X: x, Y: y, Width: width, Height: min(maxY-y, activeRows)}, true, styles, model)
	y += activeRows
	drawThemeSampleListing(screen, Rect{X: x, Y: y, Width: width, Height: min(maxY-y, dialog.ThemeInactivePanelRows)}, false, styles, model)
}

func drawThemeSampleListing(screen tcell.Screen, rect Rect, active bool, styles theme.Theme, model Model) {
	if rect.Width < 12 || rect.Height < 3 {
		return
	}
	st, pinned, marks, disk := themeSamplePanel()
	id := PrimaryPanel
	if !active {
		id = SecondaryPanel
		st.Cursor, st.ScrollOffset = 6, 5 // main.go, between the broken link and two selected files
	}
	drawPanel(screen, rect, st,
		PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
		PanelContext{PanelID: id, FileListActive: active, CursorRowActive: active, ActivePanel: PrimaryPanel, SyncDriverPanelID: -1, QuickViewDriverPanelID: -1},
		PanelDisplayConfig{
			ShowIcons: model.UseNerdfontIcons, Painter: disk, ShowDiskUsage: true, JobMarks: marks, PinnedPaths: pinned,
			ScrollbarShowInactive: true, CarouselLayout: panelcarousel.DefaultLayout(), UserHomeDir: "/home/user",
		})
}
