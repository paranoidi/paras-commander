package ui

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func testBrowserMenuDefinitions(tb testing.TB) []menu.Definition {
	tb.Helper()
	km, err := keymap.Default()
	if err != nil {
		tb.Fatalf("keymap.Default: %v", err)
	}
	return menu.BrowserDefinitions(km, false)
}

func TestFormatEntryPrefixesDirectoriesAndNonDirectories(t *testing.T) {
	tests := []struct {
		name  string
		entry localfs.Entry
		want  string
	}{
		{
			name:  "directory",
			entry: localfs.Entry{Name: "src", Type: localfs.EntryDirectory},
			want:  "/src",
		},
		{
			name:  "file",
			entry: localfs.Entry{Name: "main.go", Type: localfs.EntryFile},
			want:  " main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatEntry(tt.entry, 50, false, 0, false, nil, false, 0, "", panel.ListFormatMtime, false)
			nameWidth := panelListNameWidth(50, panel.ListFormatMtime, false, false)
			if nameColumn := strings.TrimRight(got[:nameWidth], " "); nameColumn != tt.want {
				t.Fatalf("name column = %q, want %q", nameColumn, tt.want)
			}
		})
	}
}

func TestFormatEntryFileIconsOmitsDirectorySlash(t *testing.T) {
	entry := localfs.Entry{Name: "src", Type: localfs.EntryDirectory}
	got := formatEntry(entry, 50, true, 0, false, nil, false, 0, "", panel.ListFormatMtime, false)
	nameWidth := panelListNameWidth(50, panel.ListFormatMtime, false, false)
	nameColumn := strings.TrimRight(got[:nameWidth], " ")
	if nameColumn != " src" {
		t.Fatalf("icons mode name column = %q, want space-prefixed dir name without slash", nameColumn)
	}
}

func TestFormatEntrySubtreeSelectionMark(t *testing.T) {
	entry := localfs.Entry{Name: "sub", Path: "/tmp/p/sub", Type: localfs.EntryDirectory}
	got := formatEntry(entry, 50, false, 0, true, nil, false, 0, "", panel.ListFormatMtime, false)
	nameWidth := panelListNameWidth(50, panel.ListFormatMtime, false, false)
	nameColumn := strings.TrimRight(got[:nameWidth], " ")
	if nameColumn != "/sub ○" {
		t.Fatalf("name column = %q, want trailing mark after dir slash prefix", nameColumn)
	}
	gotIcons := formatEntry(entry, 50, true, 0, true, nil, false, 0, "", panel.ListFormatMtime, false)
	nameColumn = strings.TrimRight(gotIcons[:nameWidth], " ")
	if nameColumn != " sub ○" {
		t.Fatalf("icons mode name column = %q, want dirname then trailing mark", nameColumn)
	}
}

func TestFormatEntryJobQueueMark(t *testing.T) {
	entry := localfs.Entry{Name: "file.txt", Path: "/tmp/file.txt", Type: localfs.EntryFile}
	glyph := rune('\uf144')
	got := formatEntry(entry, 50, false, glyph, false, nil, false, 0, "", panel.ListFormatMtime, false)
	nameWidth := panelListNameWidth(50, panel.ListFormatMtime, false, false)
	nameColumn := strings.TrimRight(got[:nameWidth], " ")
	want := " file.txt " + string(glyph)
	if nameColumn != want {
		t.Fatalf("name column = %q, want %q", nameColumn, want)
	}
}

func TestFormatEntryJobQueueMarkBeforeSubtreeSelectionMark(t *testing.T) {
	entry := localfs.Entry{Name: "sub", Path: "/tmp/p/sub", Type: localfs.EntryDirectory}
	glyph := rune('\uf144')
	got := formatEntry(entry, 50, false, glyph, true, nil, false, 0, "", panel.ListFormatMtime, false)
	nameWidth := panelListNameWidth(50, panel.ListFormatMtime, false, false)
	nameColumn := strings.TrimRight(got[:nameWidth], " ")
	want := "/sub " + string(glyph) + " ○"
	if nameColumn != want {
		t.Fatalf("name column = %q, want %q", nameColumn, want)
	}
}

func TestFormatEntryListingFormats(t *testing.T) {
	const rowW = 50
	e := localfs.Entry{
		Name: "x.go", Path: "/tmp/x.go", Type: localfs.EntryFile,
		Size: 100, Mode: 0o644, ModifiedAt: time.Date(2020, 1, 2, 15, 4, 0, 0, time.UTC),
	}
	mtimeRow := formatEntry(e, rowW, false, 0, false, nil, false, 0, "", panel.ListFormatMtime, false)
	if !strings.Contains(mtimeRow, formatTime(e.ModifiedAt)) {
		t.Fatalf("mtime format row = %q, want modified time substring", mtimeRow)
	}
	briefRow := formatEntry(e, rowW, false, 0, false, nil, false, 0, "", panel.ListFormatBrief, false)
	if strings.Contains(briefRow, "2020") {
		t.Fatalf("brief format should omit date: %q", briefRow)
	}
	permRow := formatEntry(e, rowW, false, 0, false, nil, false, 0, "", panel.ListFormatPerm, false)
	if !strings.Contains(permRow, "-rw") && !strings.Contains(permRow, "rw-") {
		t.Fatalf("perm format row = %q, want mode substring", permRow)
	}
	nwBrief := panelListNameWidth(rowW, panel.ListFormatBrief, false, false)
	nwMtime := panelListNameWidth(rowW, panel.ListFormatMtime, false, false)
	if nwBrief <= nwMtime {
		t.Fatalf("brief name width %d should exceed mtime name width %d", nwBrief, nwMtime)
	}
}

func TestFormatEntryShrunkenNameOnly(t *testing.T) {
	e := localfs.Entry{Name: "readme.md", Type: localfs.EntryFile, Size: 999, ModifiedAt: time.Date(2020, 1, 2, 3, 4, 0, 0, time.UTC)}
	const rowW = 35
	got := formatEntry(e, rowW, false, 0, false, nil, false, 0, "", panel.ListFormatMtime, true)
	if strings.Contains(got, "2020") || strings.Contains(got, "999") {
		t.Fatalf("name-only row should omit date/size: %q", got)
	}
	if !strings.Contains(strings.TrimSpace(got), "readme.md") {
		t.Fatalf("name-only row = %q, want name substring", got)
	}
	if utf8.RuneCountInString(got) != rowW {
		t.Fatalf("row width = %d, want %d", utf8.RuneCountInString(got), rowW)
	}
}

type testDiskUsageMap map[string]int64

func (m testDiskUsageMap) ByteSize(absPath string) (int64, bool) {
	n, ok := m[absPath]
	return n, ok
}

func (testDiskUsageMap) PendingForPanel(string, int) bool { return false }

func (testDiskUsageMap) DiskScanBusy() bool { return false }

func (testDiskUsageMap) DiskScanExcluded(string, bool, uint64, bool, func(string) bool) bool {
	return false
}

func TestFormatEntryDirectorySizeUsesDiskUsageCache(t *testing.T) {
	const rowW = 50
	nameWidth := panelListNameWidth(rowW, panel.ListFormatMtime, false, false)
	dir := localfs.Entry{Name: "projects", Path: "/home/u/projects", Type: localfs.EntryDirectory}
	cache := testDiskUsageMap{"/home/u/projects": 5000}
	got := formatEntry(dir, rowW, false, 0, false, cache, false, 0, "", panel.ListFormatMtime, false)
	want := fmt.Sprintf("%-*s %*s  %-*s", nameWidth, "/projects", panelListSizeCells, formatByteSizeListed(5000), panelListModTimeCells, "")
	if got != want {
		t.Fatalf("full row = %q, want %q", got, want)
	}
}

func TestFormatEntryDirectorySizeEmptyWithoutCache(t *testing.T) {
	const rowW = 50
	nameWidth := panelListNameWidth(rowW, panel.ListFormatMtime, false, false)
	dir := localfs.Entry{Name: "empty", Path: "/tmp/empty", Type: localfs.EntryDirectory}
	got := formatEntry(dir, rowW, false, 0, false, nil, false, 0, "", panel.ListFormatMtime, false)
	want := fmt.Sprintf("%-*s %*s  %-*s", nameWidth, "/empty", panelListSizeCells, "", panelListModTimeCells, "")
	if got != want {
		t.Fatalf("full row = %q, want %q", got, want)
	}
}

func TestRenderUsesYellowForegroundForSelectedEntry(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 30)

	styles := theme.Default()
	selected := localfs.Entry{Name: "a.txt", Path: "/tmp/a.txt"}
	model := Model{
		Left: panel.State{
			Path:          pathloc.MustParse("/tmp"),
			Entries:       []localfs.Entry{selected, localfs.Entry{Name: "b.txt", Path: "/tmp/b.txt"}},
			Cursor:        1,
			SelectedPaths: map[string]bool{selected.Path: true},
		},
		Right:       panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel: LeftPanel,
	}

	Render(screen, model, styles)

	_, rowStyle, _ := screen.Get(1, 3)
	if rowStyle != styles.PanelRowSelected {
		t.Fatalf("selected row style = %v, want theme selected style %v", rowStyle, styles.PanelRowSelected)
	}
}

func TestRenderPanelTitleLeavesBorderAfterSingleTrailingSpace(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}

	styles := theme.Default()
	Render(screen, model, styles)

	titlePrefix := tcelltest.TextAt(screen, 0, 1, 9)
	if titlePrefix != "┌─ /tmp ─" {
		t.Fatalf("panel title prefix = %q, want border immediately after title padding", titlePrefix)
	}
}

func TestRenderDrawsLeftPanelPulldownWithKeymapLabels(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	km, err := keymap.Default()
	if err != nil {
		t.Fatalf("keymap.Default: %v", err)
	}
	model := Model{
		Left:            panel.State{Path: pathloc.MustParse("/tmp")},
		Right:           panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel:     LeftPanel,
		MenuDefinitions: menu.BrowserDefinitions(km, false),
		Menu: menu.State{
			Open:         true,
			PulldownOpen: true,
			ActiveMenu:   0,
		},
	}
	styles := theme.Default()
	Render(screen, model, styles)

	rowQuick := strings.TrimSpace(tcelltest.TextAt(screen, 1, 2, 72))
	if !strings.Contains(rowQuick, "Quick view") || !strings.Contains(rowQuick, "S-F3") {
		t.Fatalf("quick view row = %q, want Quick view with S-F3", rowQuick)
	}
	rowSort := strings.TrimSpace(tcelltest.TextAt(screen, 1, 3, 72))
	if !strings.Contains(rowSort, "Sort") || !strings.Contains(rowSort, "C-s") {
		t.Fatalf("sort row = %q, want Sort with C-s", rowSort)
	}
	rowHidden := strings.TrimSpace(tcelltest.TextAt(screen, 1, 4, 72))
	if !strings.Contains(rowHidden, "Toggle hidden") || !strings.Contains(rowHidden, "M-.") {
		t.Fatalf("hidden row = %q, want Toggle hidden with M-.", rowHidden)
	}
}

func TestRenderDrawsFilePulldownMenu(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	model := Model{
		Left:            panel.State{Path: pathloc.MustParse("/tmp")},
		Right:           panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel:     LeftPanel,
		MenuDefinitions: testBrowserMenuDefinitions(t),
		Menu: menu.State{
			Open:         true,
			PulldownOpen: true,
			ActiveMenu:   menu.DefaultIndex(),
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	menuText := tcelltest.TextAt(screen, 8, 2, 40)
	if !strings.Contains(menuText, "View") || !strings.Contains(menuText, "F3") {
		t.Fatalf("first file menu row = %q, want View with F3", menuText)
	}
	// File menu pulldown top-left: after first bar item " Left " (6 runes + 1 gap) => x=7.
	_, pulldownBorder, _ := screen.Get(7, 1)
	if pulldownBorder != styles.MenuDropdownFrame {
		t.Fatalf("pulldown border style = %v, want menu.dropdown.frame %v", pulldownBorder, styles.MenuDropdownFrame)
	}
	menuText = tcelltest.TextAt(screen, 8, 5, 40)
	if !strings.Contains(menuText, "Copy") || !strings.Contains(menuText, "F5") {
		t.Fatalf("copy file menu row = %q, want Copy with F5", menuText)
	}

	_, rowStyle, _ := screen.Get(8, 2)
	if rowStyle != styles.MenuDropdownSelected {
		t.Fatalf("selected menu style = %v, want theme menu dropdown selected style %v", rowStyle, styles.MenuDropdownSelected)
	}

	_, shortcutStyle, _ := screen.Get(9, 2)
	shortcutForeground, shortcutBackground, _ := shortcutStyle.Decompose()
	wantForeground, _, _ := styles.MenuDropdownAccent.Decompose()
	_, wantBackground, _ := styles.MenuDropdownSelected.Decompose()
	if shortcutForeground != wantForeground || shortcutBackground != wantBackground {
		t.Fatalf("selected shortcut fg/bg = %v/%v, want %v/%v", shortcutForeground, shortcutBackground, wantForeground, wantBackground)
	}
}

func TestRenderUsesBlockedPanelFrameWhenMenuOpen(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel: LeftPanel,
		Menu: menu.State{
			Open: true,
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	_, cornerStyle, _ := screen.Get(0, 1)
	if cornerStyle != styles.PanelBlockedFrame {
		t.Fatalf("left panel border style = %v, want panel blocked frame %v", cornerStyle, styles.PanelBlockedFrame)
	}
}

func TestRenderUsesBlockedPanelFrameWhenSortDialogOpen(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel: LeftPanel,
		SortDialog: SortDialogState{
			Open:    true,
			PanelID: LeftPanel,
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	_, cornerStyle, _ := screen.Get(0, 1)
	if cornerStyle != styles.PanelBlockedFrame {
		t.Fatalf("left panel border style = %v, want panel blocked frame %v", cornerStyle, styles.PanelBlockedFrame)
	}
}

func TestRenderUsesBlockedPanelFrameWhenListingFormatDialogOpen(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/tmp")},
		ActivePanel: LeftPanel,
		ListingFormatDialog: ListingFormatDialogState{
			Open:    true,
			PanelID: LeftPanel,
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	_, cornerStyle, _ := screen.Get(0, 1)
	if cornerStyle != styles.PanelBlockedFrame {
		t.Fatalf("left panel border style = %v, want panel blocked frame %v", cornerStyle, styles.PanelBlockedFrame)
	}
}

func TestRenderThemeDialogPreviewShowsActiveUnblockedLeftPanel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	styles := theme.Default()
	leftEntry := localfs.Entry{Name: "a.txt", Path: "/tmp/a.txt"}
	model := Model{
		Left: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{leftEntry},
			Cursor:  0,
		},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
		ThemeDialog: ThemeDialogState{
			Open:        true,
			CurrentName: "default",
			Choices:     []ThemeChoice{{Name: "default", Label: "Default"}},
		},
	}

	Render(screen, model, styles)

	_, leftBorder, _ := screen.Get(0, 1)
	if leftBorder != styles.PanelActiveFrame {
		t.Fatalf("left panel border = %v, want active panel border for theme preview", leftBorder)
	}
	_, rowStyle, _ := screen.Get(1, 3)
	if rowStyle != styles.PanelCursorActive {
		t.Fatalf("left list cursor style = %v, want %v (active cursor for theme preview)", rowStyle, styles.PanelCursorActive)
	}
}

func TestRenderDrawsThemeDialog(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)
	width, _ := screen.Size()

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		ThemeDialog: ThemeDialogState{
			Open:        true,
			Selected:    1,
			CurrentName: "default",
			Choices: []ThemeChoice{
				{Name: "default", Label: "Default"},
				{Name: "test-theme", Label: "Test Theme"},
			},
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	// Dialog now has dual-column layout with preview.
	// Theme list is left column, second item at row 3.
	row := tcelltest.TextAt(screen, 0, 3, width)
	if !strings.Contains(row, "Test Theme") {
		t.Fatalf("theme dialog row = %q, want Test Theme", strings.TrimRight(row, " "))
	}
	styleCol := strings.Index(row, "Test Theme")
	if styleCol < 0 {
		t.Fatal("Catppuccin Frappe substring not found in row")
	}
	_, rowStyle, _ := screen.Get(styleCol, 3)
	wantStyle := styles.DialogOptionActive
	if rowStyle != wantStyle {
		t.Fatalf("selected theme row style = %v, want dialog.option.active %v", rowStyle, wantStyle)
	}
}

func TestRenderDrawsMessageDialog(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)
	width, _ := screen.Size()

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		MessageDialog: MessageDialogState{
			Open:    true,
			Title:   "Error",
			Message: "theme reload failed: disk full",
		},
	}

	styles := theme.Default()
	Render(screen, model, styles)

	found := false
	for y := 0; y < 20; y++ {
		line := tcelltest.TextAt(screen, 0, y, width)
		if strings.Contains(line, "theme reload failed") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected message dialog body on screen")
	}
	foundOK := false
	for y := 0; y < 20; y++ {
		line := tcelltest.TextAt(screen, 0, y, width)
		if strings.Contains(line, "OK") && strings.Contains(line, "[") {
			foundOK = true
			break
		}
	}
	if !foundOK {
		t.Fatal("expected OK button row")
	}
}

func TestMenuBarInteractiveFalseWhenFullscreenFilePreview(t *testing.T) {
	t.Parallel()
	m := Model{ViewMode: ViewFilePreview}
	if m.MenuBarInteractive() {
		t.Fatal("fullscreen file preview must not use interactive menu bar")
	}
}

func TestRenderBlankMenuBarRowWhenFullscreenFilePreview(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80

	styles := theme.Default()
	model := Model{
		Left:                      panel.State{Path: pathloc.MustParse("/tmp")},
		Right:                     panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:               LeftPanel,
		ViewMode:                  ViewFilePreview,
		FullscreenFilePreviewDraw: FilePreviewState{Open: true, Phase: FilePreviewPhaseDone, CombinedText: "hi\n"},
	}

	Render(screen, model, styles)

	top := tcelltest.TextAt(screen, 0, 0, width)
	if strings.Contains(top, "File") || strings.Contains(top, "Left") {
		t.Fatalf("menu row = %q, want blank (no pulldown menu labels)", top)
	}
}

func TestRenderBlankMenuBarRowWhenModalDialogOpen(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80

	styles := theme.Default()
	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		FileDialog: FileDialogState{
			Open:       true,
			DialogType: FileDialogMkdir,
			Fields:     []FileDialogField{{Label: "Name", Value: "x"}},
		},
	}

	Render(screen, model, styles)

	top := tcelltest.TextAt(screen, 0, 0, width)
	if strings.Contains(top, "File") || strings.Contains(top, "Left") {
		t.Fatalf("menu row = %q, want blank (no menu labels)", top)
	}
	_, menuStyle, _ := screen.Get(0, 0)
	if menuStyle != styles.MenuBar {
		t.Fatalf("top-left cell style = %v, want MenuBar %v", menuStyle, styles.MenuBar)
	}
	titlePrefix := tcelltest.TextAt(screen, 0, 1, 9)
	if titlePrefix != "┌─ /tmp ─" {
		t.Fatalf("panel title prefix = %q, want border on row below menu", titlePrefix)
	}
}

func TestQuickFilterStartBlocked(t *testing.T) {
	t.Parallel()
	var m Model
	if m.QuickFilterStartBlocked() {
		t.Fatal("empty model should not block quick filter")
	}
	m.Menu.Open = true
	if !m.QuickFilterStartBlocked() {
		t.Fatal("open menu should block")
	}
	m = Model{MessageDialog: MessageDialogState{Open: true}}
	if !m.QuickFilterStartBlocked() {
		t.Fatal("message dialog should block")
	}
	m = Model{HelpView: HelpViewState{Open: true}}
	if m.QuickFilterStartBlocked() {
		t.Fatal("help view alone should not block (legacy parity with shouldStartFilter)")
	}
}

func TestAuxiliaryViewDialogKeysBlocked(t *testing.T) {
	t.Parallel()
	var m Model
	if m.AuxiliaryViewDialogKeysBlocked() {
		t.Fatal("empty model should not block")
	}
	m.TransferDialog.Open = true
	if !m.AuxiliaryViewDialogKeysBlocked() {
		t.Fatal("transfer dialog should block")
	}
}

func TestRenderMenuBarShowsActiveFilePermissionString(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80
	const permRightMargin = 1

	mode := fs.FileMode(0o754) | fs.ModeDir
	want := localfs.UnixModeString(mode)

	styles := theme.Default()
	model := Model{
		Left: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{{Name: "d", Path: "/tmp/d", Type: localfs.EntryDirectory, Mode: mode}},
			Cursor:  0,
		},
		Right:             panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:       LeftPanel,
		MenuDefinitions:   testBrowserMenuDefinitions(t),
		Menu:              menu.State{ActiveMenu: menu.DefaultIndex()},
		MenuBarPermission: want,
	}

	Render(screen, model, styles)

	row := tcelltest.TextAt(screen, 0, 0, width)
	trimmed := strings.TrimRight(row, " ")
	if !strings.HasSuffix(trimmed, want) {
		t.Fatalf("menu row trimmed = %q, want suffix %q", trimmed, want)
	}
	startCol := width - permRightMargin - utf8.RuneCountInString(want)
	_, st, _ := screen.Get(startCol, 0)
	if st != styles.MenuDetail {
		t.Fatalf("permission first cell style = %v, want MenuDetail %v", st, styles.MenuDetail)
	}
}

func TestRenderMenuBarShowsActivitySpinnerAfterPermission(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80
	const permRightMargin = 1

	mode := fs.FileMode(0o755) | fs.ModeDir
	perm := localfs.UnixModeString(mode)
	permW := utf8.RuneCountInString(perm)

	styles := theme.Default()
	model := Model{
		Left: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{{Name: "d", Path: "/tmp/d", Type: localfs.EntryDirectory, Mode: mode}},
			Cursor:  0,
		},
		Right:                  panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:            LeftPanel,
		MenuDefinitions:        testBrowserMenuDefinitions(t),
		Menu:                   menu.State{ActiveMenu: menu.DefaultIndex()},
		MenuBarPermission:      perm,
		MenuBarActivitySpinner: true,
		DiskUsage:              nil,
		SpinPhase:              0,
	}

	Render(screen, model, styles)

	spinnerCol := width - permRightMargin - 1
	wantSpinner := MenuBarSpinnerGlyph(0)
	rCell, st, _ := screen.Get(spinnerCol, 0)
	rFirst, _ := utf8.DecodeRuneInString(rCell)
	if rFirst != wantSpinner {
		t.Fatalf("menu row spinner rune = %q (%U), want %q (%U)", rFirst, rFirst, wantSpinner, wantSpinner)
	}
	if st != styles.MenuSpinner {
		t.Fatalf("spinner style = %v, want MenuSpinner %v", st, styles.MenuSpinner)
	}
	gapCol := spinnerCol - 1
	grCell, _, _ := screen.Get(gapCol, 0)
	gr, _ := utf8.DecodeRuneInString(grCell)
	if gr != ' ' {
		t.Fatalf("gap before spinner = %q, want space", gr)
	}
	permRunes := []rune(perm)
	wantLastPermRune := permRunes[len(permRunes)-1]
	permLastCol := gapCol - 1
	prCell, pst, _ := screen.Get(permLastCol, 0)
	pr, _ := utf8.DecodeRuneInString(prCell)
	if pr != wantLastPermRune {
		t.Fatalf("last permission rune at %d = %q style %v, want %q", permLastCol, pr, pst, wantLastPermRune)
	}
	if pst != styles.MenuDetail {
		t.Fatalf("permission tail style = %v, want MenuDetail", pst)
	}
	if menuBarPermissionTailRuneCount(perm, true) != permW+2 {
		t.Fatalf("tail width mismatch for clip logic")
	}
}

func TestRenderTransientStatusAboveFooterAfterThemeDialog(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	const width = 80

	styles := theme.Default()
	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		ThemeDialog: ThemeDialogState{
			Open:     true,
			Selected: 0,
			Focus:    0,
			Choices:  []ThemeChoice{{Name: "default", Label: "default"}},
		},
		Message:        "theme-reload-err",
		MessageUrgency: MessageUrgencyCritical,
	}

	Render(screen, model, styles)

	statusRow := layoutStatusMessageRowY(24)
	banner := tcelltest.TextAt(screen, 0, statusRow, width)
	if !strings.Contains(banner, "theme-reload") {
		t.Fatalf("status row %d = %q, want transient status visible after theme dialog", statusRow, banner)
	}
	top := tcelltest.TextAt(screen, 0, 0, width)
	if strings.Contains(top, "theme-reload") {
		t.Fatalf("menu row = %q, status should not be on top row", top)
	}
}

func TestDrawFooterShowsViewEditAndOmitsUnusedFKeys(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	drawFooter(screen, Rect{X: 0, Y: 0, Width: 80, Height: 1}, styles, menu.FunctionKeys)

	str0, st0, _ := screen.Get(0, 0)
	if str0 != "F" || st0 != styles.FooterKey {
		t.Fatalf("footer must start flush-left with F1 key at col 0, got %q style %v", str0, st0)
	}

	line := tcelltest.TextAt(screen, 0, 0, 70)
	if !strings.Contains(line, "View") || !strings.Contains(line, "Edit") {
		t.Fatalf("footer = %q, want F3 View and F4 Edit placeholders", line)
	}
	if strings.Contains(line, "F11") || strings.Contains(line, "F12") {
		t.Fatalf("footer should not list empty F11/F12 slots, got %q", line)
	}
}

func TestDrawFooterUsesFullRowForFunctionKeys(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	drawFooter(screen, Rect{X: 0, Y: 0, Width: 80, Height: 1}, styles, menu.FunctionKeys)
	lastS, _, _ := screen.Get(79, 0)
	lastR, _ := utf8.DecodeRuneInString(lastS)
	if lastR != 't' {
		t.Fatalf("key band should reach col 79 (last Quit rune); got %q", lastR)
	}
}

func TestDrawFooterSpacesKeyHintWithLabelStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	drawFooter(screen, Rect{X: 0, Y: 0, Width: 80, Height: 1}, styles, []menu.FunctionKey{menu.FooterEscClose})

	line := tcelltest.TextAt(screen, 0, 0, 80)
	idx := strings.Index(line, "Esc")
	if idx < 0 {
		t.Fatalf("footer line = %q, want Esc", line)
	}
	if idx != 0 {
		t.Fatalf("single footer item should be flush-left, Esc at %d, want 0", idx)
	}
	for col := idx; col < idx+3; col++ {
		_, st, _ := screen.Get(col, 0)
		if st != styles.FooterKey {
			t.Fatalf("Esc rune at col %d: style = %v, want FooterKey", col, st)
		}
	}
	gapStr, gapSt, _ := screen.Get(idx+3, 0)
	var gapR rune
	if gapStr != "" {
		gapR, _ = utf8.DecodeRuneInString(gapStr)
	}
	if gapR != ' ' {
		t.Fatalf("gap col = %q, want space between key and hint", gapR)
	}
	if gapSt != styles.FooterLabel {
		t.Fatalf("gap after key uses style %v, want FooterLabel (no key-colored pad)", gapSt)
	}
	_, hintSt, _ := screen.Get(idx+4, 0)
	if hintSt != styles.FooterLabel {
		t.Fatalf("first hint char style = %v, want FooterLabel", hintSt)
	}
}

func TestRenderDrawsStatusMessage(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		Message:     "Refreshed",
	}

	styles := theme.Default()
	Render(screen, model, styles)

	statusRow := layoutStatusMessageRowY(12)
	banner := tcelltest.TextAt(screen, 0, statusRow, width)
	if !strings.Contains(banner, "Refreshed") {
		t.Fatalf("status row %d = %q, want status message overlay", statusRow, banner)
	}
	wantStart := (width-len([]rune(FormatToastDisplay("Refreshed"))))/2 + 1
	msgStr, msgSt, _ := screen.Get(wantStart, statusRow)
	msgR, _ := utf8.DecodeRuneInString(msgStr)
	if msgR != 'R' {
		t.Fatalf("status message should be centered (starts at col %d); got %q style %v", wantStart, msgR, msgSt)
	}
	if msgSt != styles.MessageInfo {
		t.Fatalf("status message style = %v, want MessageInfo", msgSt)
	}
	menuRow := tcelltest.TextAt(screen, 0, 0, width)
	if strings.Contains(menuRow, "Refreshed") {
		t.Fatalf("menu row should not show status message, got %q", menuRow)
	}
	footerText := tcelltest.TextAt(screen, 0, 11, width)
	if strings.Contains(footerText, "Refreshed") {
		t.Fatalf("footer line should not duplicate status message, got %q", footerText)
	}
}

func TestRenderStatusMessageDoesNotOverlayMenuBarPermission(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	const width = 80

	mode := fs.FileMode(0o755) | fs.ModeDir
	perm := localfs.UnixModeString(mode)

	model := Model{
		Left: panel.State{
			Path:    pathloc.MustParse("/tmp"),
			Entries: []localfs.Entry{{Name: "d", Path: "/tmp/d", Type: localfs.EntryDirectory, Mode: mode}},
			Cursor:  0,
		},
		Right:             panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:       LeftPanel,
		MenuDefinitions:   testBrowserMenuDefinitions(t),
		Menu:              menu.State{ActiveMenu: menu.DefaultIndex()},
		MenuBarPermission: perm,
		Message:           "Saved",
	}
	styles := theme.Default()
	Render(screen, model, styles)

	statusRow := layoutStatusMessageRowY(12)
	banner := tcelltest.TextAt(screen, 0, statusRow, width)
	if !strings.Contains(banner, "Saved") {
		t.Fatalf("status row = %q, want message above footer", banner)
	}
	wantLastCol := width - 1
	lastStr, lastSt, _ := screen.Get(wantLastCol, 0)
	lastR, _ := utf8.DecodeRuneInString(lastStr)
	if lastR == 'd' && lastSt == styles.MessageInfo {
		t.Fatalf("menu permission area should not be covered by status message")
	}
}

func TestRenderStatusMessageUsesUrgencyStyle(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	model := Model{
		Left:           panel.State{Path: pathloc.MustParse("/tmp")},
		Right:          panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:    LeftPanel,
		Message:        "o_O",
		MessageUrgency: MessageUrgencyWarn,
	}
	styles := theme.Default()
	Render(screen, model, styles)

	statusRow := layoutStatusMessageRowY(12)
	col := (80-len([]rune(FormatToastDisplay("o_O"))))/2 + 1
	_, st, _ := screen.Get(col, statusRow)
	if st != styles.MessageWarn {
		t.Fatalf("urgency style = %v, want MessageWarn", st)
	}
}

func TestRenderStatusMessageLeavesMenuLabelsVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
		Message:     "Hi",
	}
	styles := theme.Default()
	model.Menu.Open = false
	Render(screen, model, styles)

	// First menu label is " Left " — column 1 is 'L' in Menu style when menu pulldown is closed.
	_, st, _ := screen.Get(1, 0)
	if st != styles.MenuBar {
		t.Fatalf("col 1 style = %v, want Menu (menu not covered by status fill)", st)
	}
	statusRow := layoutStatusMessageRowY(12)
	col := (80 - len([]rune("Hi"))) / 2
	_, msgSt, _ := screen.Get(col, statusRow)
	if msgSt != styles.MessageInfo {
		t.Fatalf("message cell style = %v, want MessageInfo", msgSt)
	}
}

// layoutStatusMessageRowY is the row immediately above the footer for a terminal height.
func layoutStatusMessageRowY(height int) int {
	return height - 2
}

func TestRenderDrawsPanelLocalFuzzyInputOverlay(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "main.go", Path: "/tmp/main.go"}},
		Filter:  panel.FilterState{CaseInsensitive: true},
	}
	left.OpenFilter(5)
	for _, r := range "ma" {
		left.AppendFilterRune(r, 5)
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}

	Render(screen, model, theme.Default())

	filterText := tcelltest.TextAt(screen, 2, 1, 20)
	if !strings.Contains(filterText, "> ma") {
		t.Fatalf("filter row = %q, want fuzzy query with > prefix", filterText)
	}
	rightPanelText := tcelltest.TextAt(screen, 42, 7, 10)
	if strings.Contains(rightPanelText, "ma") {
		t.Fatalf("right panel text = %q, want fuzzy input scoped to active panel", rightPanelText)
	}
}

func TestRenderFuzzyInputUsesNomatchStyleWhenNoMatches(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "main.go", Path: "/tmp/main.go"}},
		Filter:  panel.FilterState{CaseInsensitive: true},
	}
	left.OpenFilter(5)
	for _, r := range "zzz" {
		left.AppendFilterRune(r, 5)
	}
	styles := theme.Default()
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}

	Render(screen, model, styles)

	// Title row y=1 with menu bar; titleX=2; "> zzz" places first 'z' at x=4.
	_, cellSt, _ := screen.Get(4, 1)
	if cellSt != styles.FuzzyInputNomatch {
		t.Fatalf("query cell style = %v, want FuzzyInputNomatch", cellSt)
	}
}

func TestRenderHighlightsFilterMatches(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "main.go", Path: "/tmp/main.go"}},
		Filter:  panel.FilterState{CaseInsensitive: true},
	}
	left.OpenFilter(5)
	for _, r := range "ma" {
		left.AppendFilterRune(r, 5)
	}
	styles := theme.Default()
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}

	Render(screen, model, styles)

	_, highlightedStyle, _ := screen.Get(2, 3)
	highlightForeground, _, _ := highlightedStyle.Decompose()
	wantForeground, _, _ := styles.FuzzyHighlightCursor.Decompose()
	if highlightForeground != wantForeground {
		t.Fatalf("highlight foreground = %v, want %v", highlightForeground, wantForeground)
	}
}

func TestRenderDrawsSyncIndicatorOnLeftDriverBottomBorder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:              panel.State{Path: pathloc.MustParse("/tmp")},
		Right:             panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:       LeftPanel,
		SyncFollowEnabled: true,
		SyncFollowPanel:   LeftPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if !strings.Contains(leftBottom, "Sync →") {
		t.Fatalf("left bottom border = %q, want it to contain %q", leftBottom, "Sync →")
	}
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if strings.Contains(rightBottom, "Sync") {
		t.Fatalf("right bottom border = %q, want no Sync indicator on the follower", rightBottom)
	}
}

func TestRenderDrawsSyncIndicatorOnRightDriverBottomBorder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:              panel.State{Path: pathloc.MustParse("/tmp")},
		Right:             panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:       RightPanel,
		SyncFollowEnabled: true,
		SyncFollowPanel:   RightPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if !strings.Contains(rightBottom, "← Sync") {
		t.Fatalf("right bottom border = %q, want it to contain %q", rightBottom, "← Sync")
	}
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, "Sync") {
		t.Fatalf("left bottom border = %q, want no Sync indicator on the follower", leftBottom)
	}
}

func TestRenderDrawsQuickViewIndicatorOnLeftActivePanel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:             panel.State{Path: pathloc.MustParse("/tmp")},
		Right:            panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:      LeftPanel,
		QuickViewEnabled: true,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if !strings.Contains(leftBottom, "Quick view →") {
		t.Fatalf("left bottom border = %q, want it to contain %q", leftBottom, "Quick view →")
	}
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if strings.Contains(rightBottom, "Quick view") {
		t.Fatalf("right bottom border = %q, want no Quick view indicator on inactive panel", rightBottom)
	}
}

func TestRenderDrawsQuickViewIndicatorOnRightActivePanel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:             panel.State{Path: pathloc.MustParse("/tmp")},
		Right:            panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:      RightPanel,
		QuickViewEnabled: true,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if !strings.Contains(rightBottom, "← Quick view") {
		t.Fatalf("right bottom border = %q, want it to contain %q", rightBottom, "← Quick view")
	}
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, "Quick view") {
		t.Fatalf("left bottom border = %q, want no Quick view indicator on inactive panel", leftBottom)
	}
}

func TestRenderOmitsQuickViewIndicatorWhenDisabled(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, "Quick view") {
		t.Fatalf("left bottom border = %q, want no Quick view indicator when off", leftBottom)
	}
}

func TestRenderQuickViewPreviewTitleShowsPathAndFilename(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	projects := "/home/user/projects"
	model := Model{
		Left:             panel.State{Path: pathloc.MustParse("/tmp")},
		Right:            panel.State{Path: pathloc.MustParse(projects)},
		ActivePanel:      LeftPanel,
		QuickViewEnabled: true,
		FilePreviewDraw: FilePreviewState{
			Open:      true,
			Phase:     FilePreviewPhaseDone,
			TitleBase: "readme.md",
			Path:      filepath.Join(projects, "readme.md"),
		},
		UserHomeDir: "/home/user",
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	topRow := tcelltest.TextAt(screen, leftWidth, 1, width-leftWidth)
	if !strings.Contains(topRow, "projects") {
		t.Fatalf("inactive title row = %q, want directory path segment", topRow)
	}
	if !strings.Contains(topRow, "readme.md") {
		t.Fatalf("inactive title row = %q, want filename", topRow)
	}
	if strings.Contains(topRow, " / ") && strings.Contains(topRow, "%") {
		t.Fatalf("inactive title row = %q, want no free-space volume stats", topRow)
	}
}

func TestRenderHideInactivePanelFullWidthAndOtherPathIndicator(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	model := Model{
		Left:              panel.State{Path: pathloc.MustParse("/tmp")},
		Right:             panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel:       LeftPanel,
		HideInactivePanel: true,
	}
	Render(screen, model, theme.Default())

	bottomY := height - 2
	fullBottom := tcelltest.TextAt(screen, 0, bottomY, width)
	if !strings.Contains(fullBottom, "/var") && !strings.Contains(fullBottom, "var") {
		t.Fatalf("bottom border = %q, want inactive path indicator", fullBottom)
	}
	if strings.Contains(fullBottom, "Sync") {
		t.Fatalf("bottom border = %q, want no sync while hidden", fullBottom)
	}
}

func TestRenderOmitsSyncIndicatorWhenDisabled(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	// Note: zero value of SyncFollowEnabled is false; SyncFollowPanel zero value
	// (== LeftPanel) must NOT trigger the indicator on its own.
	model := Model{
		Left:        panel.State{Path: pathloc.MustParse("/tmp")},
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, "Sync") {
		t.Fatalf("left bottom border = %q, want no Sync indicator when sync is off", leftBottom)
	}
}

func TestRenderDrawsSelectionsBottomHintOnInactiveFilePanel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	crossDir := "/var/other.txt"
	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "here.txt", Path: "/tmp/here.txt"}},
		SelectedPaths: map[string]bool{
			crossDir: true,
		},
	}
	right := panel.State{
		Path:          pathloc.MustParse("/var"),
		Entries:       []localfs.Entry{{Name: "other.txt", Path: crossDir}},
		SelectedPaths: map[string]bool{crossDir: true},
	}

	model := Model{
		Left:        left,
		Right:       right,
		ActivePanel: RightPanel,
	}
	styles := theme.Default()
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if !strings.Contains(leftBottom, " Selections ") {
		t.Fatalf("inactive left bottom = %q, want substring %q", leftBottom, " Selections ")
	}
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if strings.Contains(rightBottom, " Selections ") {
		t.Fatalf("active right bottom = %q, want no selections hint on active column", rightBottom)
	}

	// Frame "─" uses panel frame style; padded title uses inactive title style (not merged into one overlay).
	dashR, dashSt, _ := screen.Get(1, bottomY)
	if dashR != "─" {
		t.Fatalf("left bottom frame dash = %q, want '─'", dashR)
	}
	if dashSt != styles.PanelInactiveFrame {
		t.Fatalf("left bottom frame dash style = %v, want PanelInactiveFrame", dashSt)
	}
	_, titleSt, _ := screen.Get(2, bottomY)
	if titleSt != styles.PanelInactiveTitle {
		t.Fatalf("selections hint padded segment style = %v, want PanelInactiveTitle", titleSt)
	}
}

func TestRenderDrawsSelectionsBottomHintOnInactiveRightFilePanel(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	outsideSel := "/tmp/outside.txt"
	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
	}
	right := panel.State{
		Path:    pathloc.MustParse("/var"),
		Entries: []localfs.Entry{{Name: "r.txt", Path: "/var/r.txt"}},
		SelectedPaths: map[string]bool{
			outsideSel: true,
		},
	}

	model := Model{
		Left:        left,
		Right:       right,
		ActivePanel: LeftPanel,
	}
	styles := theme.Default()
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	rightBottom := tcelltest.TextAt(screen, leftWidth, bottomY, width-leftWidth)
	if !strings.Contains(rightBottom, " Selections ") {
		t.Fatalf("inactive right bottom = %q, want substring %q", rightBottom, " Selections ")
	}

	// Trailing frame dash before ┘ uses PanelFrame; padded title uses inactive title style.
	lastIn := width - 2
	dashR, dashSt, _ := screen.Get(lastIn, bottomY)
	if dashR != "─" {
		t.Fatalf("right bottom frame dash = %q, want '─'", dashR)
	}
	if dashSt != styles.PanelInactiveFrame {
		t.Fatalf("right bottom frame dash style = %v, want PanelInactiveFrame", dashSt)
	}
	_, titleSt, _ := screen.Get(lastIn-1, bottomY)
	if titleSt != styles.PanelInactiveTitle {
		t.Fatalf("selections hint padded last cell style = %v, want PanelInactiveTitle", titleSt)
	}
}

func TestRenderOmitsSelectionsBottomHintWhenStripVisible(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	crossDir := "/var/other.txt"
	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "here.txt", Path: "/tmp/here.txt"}},
		SelectedPaths: map[string]bool{
			crossDir: true,
		},
	}

	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: LeftPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, " Selections ") {
		t.Fatalf("left column bottom = %q, want no file-panel selections hint when strip is visible", leftBottom)
	}
}

func TestRenderDrawsGitignoreBottomHint(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	left := panel.State{
		Path:            pathloc.MustParse("/tmp"),
		Entries:         []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
		Gitignore:       gitignore.NewCache(),
		GitignoreActive: true,
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	styles := theme.Default()
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if !strings.Contains(leftBottom, " Gitignore ") {
		t.Fatalf("left bottom = %q, want substring %q", leftBottom, " Gitignore ")
	}

	dashR, dashSt, _ := screen.Get(1, bottomY)
	if dashR != "─" {
		t.Fatalf("left bottom frame dash = %q, want '─'", dashR)
	}
	if dashSt != styles.PanelInactiveFrame {
		t.Fatalf("left bottom frame dash style = %v, want PanelInactiveFrame", dashSt)
	}
	_, labelSt, _ := screen.Get(2, bottomY)
	if labelSt != styles.PanelInactiveFrame {
		t.Fatalf("gitignore hint label style = %v, want PanelInactiveFrame", labelSt)
	}
}

func TestRenderOmitsGitignoreBottomHintOutsideWorkTree(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	left := panel.State{
		Path:            pathloc.MustParse("/tmp"),
		Entries:         []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
		Gitignore:       gitignore.NewCache(),
		GitignoreActive: false,
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, " Gitignore ") {
		t.Fatalf("left bottom = %q, want no gitignore hint outside a Git work tree", leftBottom)
	}
}

func TestRenderDrawsDotfilesHiddenBottomHint(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	sym := styles.SymbolHiddenDotfiles()
	left := panel.State{
		Path:                 pathloc.MustParse("/tmp"),
		Entries:              []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
		DotfilesHiddenActive: true,
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if !strings.Contains(leftBottom, sym) {
		t.Fatalf("left bottom = %q, want dotfiles-hidden glyph %q", leftBottom, sym)
	}
}

func TestRenderOmitsDotfilesHiddenBottomHintWhenShowHidden(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	sym := styles.SymbolHiddenDotfiles()
	left := panel.State{
		Path:                 pathloc.MustParse("/tmp"),
		Entries:              []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
		ShowHidden:           true,
		DotfilesHiddenActive: true,
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, sym) {
		t.Fatalf("left bottom = %q, want no dotfiles-hidden glyph when show hidden is on", leftBottom)
	}
}

func TestRenderOmitsDotfilesHiddenBottomHintWhenInactive(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	styles := theme.Default()
	sym := styles.SymbolHiddenDotfiles()
	left := panel.State{
		Path:    pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	Render(screen, model, styles)

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, sym) {
		t.Fatalf("left bottom = %q, want no dotfiles-hidden glyph when inactive", leftBottom)
	}
}

func TestRenderOmitsGitignoreBottomHintWhenShowHidden(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	const width, height = 80, 12
	screen.SetSize(width, height)

	left := panel.State{
		Path:            pathloc.MustParse("/tmp"),
		Entries:         []localfs.Entry{{Name: "a.txt", Path: "/tmp/a.txt"}},
		ShowHidden:      true,
		Gitignore:       gitignore.NewCache(),
		GitignoreActive: false,
	}
	model := Model{
		Left:        left,
		Right:       panel.State{Path: pathloc.MustParse("/var")},
		ActivePanel: RightPanel,
	}
	Render(screen, model, theme.Default())

	leftWidth := width / 2
	bottomY := height - 2
	leftBottom := tcelltest.TextAt(screen, 0, bottomY, leftWidth)
	if strings.Contains(leftBottom, " Gitignore ") {
		t.Fatalf("left bottom = %q, want no gitignore hint when show hidden is on", leftBottom)
	}
}
