package app

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

var fileIndexActions = []string{
	keymap.ActionFileIndexOpen,
	keymap.ActionFileIndexReveal,
	keymap.ActionFileIndexTagAdd,
	keymap.ActionFileIndexTagRemove,
	keymap.ActionFileIndexScan,
	keymap.ActionFileIndexPrune,
}

func TestFileIndexActionsInHelpAndMenus(t *testing.T) {
	a := newHelpEntriesApp(t)
	var inHelp []string
	for _, e := range a.buildHelpEntries() {
		if strings.HasPrefix(e.ActionID, "fileindex.") {
			if e.Section != "File Index" || e.Keys == "" {
				t.Errorf("help entry %+v, want section File Index with keys", e)
			}
			inHelp = append(inHelp, e.ActionID)
		}
	}
	for _, id := range append([]string{keymap.ActionFileIndexMenu}, fileIndexActions...) {
		if !slices.Contains(inHelp, id) {
			t.Errorf("%s missing from help", id)
		}
	}

	// The fileindex menu lists every action under the lowercase letter of its default chord, and
	// every fileindex chord is Alt+Shift+letter.
	var inMenu, groups []string
	for _, e := range a.keys.FileIndexMenuEntries() {
		if e.GroupTitle != "" {
			groups = append(groups, e.GroupTitle)
			continue
		}
		inMenu = append(inMenu, e.ActionID)
		ch, err := keymap.ParseKey(a.keys.Global.MenuBindingLabel(e.ActionID))
		if err != nil {
			t.Fatal(err)
		}
		if ch.Mod != tcell.ModAlt || !unicode.IsUpper(ch.Rune) || unicode.ToLower(ch.Rune) != e.Key {
			t.Errorf("%s: menu key %q does not match its chord %s", e.ActionID, e.Key, keymap.FormatChord(ch))
		}
	}
	if !slices.Equal(inMenu, fileIndexActions) {
		t.Fatalf("fileindex menu = %v, want %v", inMenu, fileIndexActions)
	}
	if want := []string{"Browse", "Tagging", "Actions"}; !slices.Equal(groups, want) {
		t.Fatalf("fileindex menu groups = %v, want %v", groups, want)
	}

	// The `:` leader menu reaches the fileindex menu, not the actions themselves.
	var leader []string
	for _, e := range a.keys.LeaderMenuEntries() {
		leader = append(leader, e.ActionID)
	}
	if !slices.Contains(leader, keymap.ActionFileIndexMenu) || slices.Contains(leader, keymap.ActionFileIndexScan) {
		t.Fatalf("leader menu actions = %v", leader)
	}
}

func TestFileIndexMenuOpensOnLessThanAndRunsLetter(t *testing.T) {
	a, screen, _, _ := fileIndexApp(t)
	cursorTo(t, a, "harbor.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
	if !a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		t.Fatalf("leader menu = %+v, want the fileindex menu", a.model.LeaderMenu)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if a.model.LeaderMenu.Open {
		t.Fatal("activating an entry should close the menu")
	}
	waitTagPrompt(t, a, screen)
	if fd := a.model.FileDialog; !fd.Open || fd.DialogType != dialog.FileDialogIndexTags || fd.IndexTagsRemove {
		t.Fatalf("dialog = %+v, want the add-tags prompt", fd)
	}

	a.dialogCtrl.CloseFileDialog()
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
	if a.model.LeaderMenu.Open {
		t.Fatal("the menu key again should close the menu")
	}
}

// fileIndexApp builds an app whose cwd holds harbor.txt and meadow.txt, both recorded in a temp
// index the app is configured to use.
func fileIndexApp(t *testing.T) (*App, tcell.SimulationScreen, string, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var batch []db.ScannedFile
	for i, name := range []string{"harbor.txt", "meadow.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := strings.Repeat(string(rune('a'+i)), 32)
		batch = append(batch, db.ScannedFile{Dir: dir, Name: name, Size: int64(len(name)), Mtime: time.Now(), MD5: sum, SHA256: sum + sum})
	}
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()

	screen := newScreen(t, 100, 30)
	opts := testOptions(dir)
	opts.Config.FileIndex.Database = dbPath
	a := newTestApp(t, screen, opts)
	t.Cleanup(func() { _ = a.fileIndex.Close() })
	return a, screen, dir, dbPath
}

// waitTagPrompt lands the tag prompt, which opens once the index check comes back.
func waitTagPrompt(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogIndexTags
	})
}

func cursorTo(t *testing.T, a *App, name string) {
	t.Helper()
	p := a.activePanel()
	for i, e := range p.Entries {
		if e.Name == name {
			p.Cursor = i
			return
		}
	}
	t.Fatalf("%s not listed in %s", name, p.PathString())
}

func TestFileIndexTagPromptTagsCursorEntry(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "harbor.txt")

	a.dispatch(keymap.ActionFileIndexTagAdd)
	waitTagPrompt(t, a, screen)
	fd := a.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogIndexTags || fd.IndexTagsRemove {
		t.Fatalf("dialog = %+v, want the add-tags prompt", fd)
	}
	if !slices.Equal(fd.IndexTagsTargets, []string{filepath.Join(dir, "harbor.txt")}) {
		t.Fatalf("targets = %v", fd.IndexTagsTargets)
	}
	for _, r := range "clover, field" {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if a.model.FileDialog.Open {
		t.Fatal("OK should close the prompt")
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return strings.HasPrefix(a.model.Message, "Added")
	})
	if a.model.Message != "Added 2 tags on 1 entry" {
		t.Fatalf("message = %q", a.model.Message)
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	tags, err := d.EntryTags(dir, []string{"harbor.txt", "meadow.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tags["harbor.txt"], []string{"clover", "field"}) || len(tags["meadow.txt"]) != 0 {
		t.Fatalf("tags = %v", tags)
	}
}

func TestFileIndexOpenAndReveal(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "meadow.txt")

	a.dispatch(keymap.ActionFileIndexOpen)
	want := "fileindex://" + dir
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == want && !p.ListingPending
	})
	p := a.activePanel()
	if p.Path.Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("panel = %s", p.PathString())
	}
	var names []string
	for _, e := range p.Entries {
		names = append(names, e.Name)
	}
	if !slices.Equal(names, []string{"harbor.txt", "meadow.txt"}) {
		t.Fatalf("index listing = %v", names)
	}
	if e, ok := p.CurrentEntry(); !ok || e.Name != "meadow.txt" {
		t.Fatalf("cursor = %+v, want the entry kept", e)
	}

	cursorTo(t, a, "harbor.txt")
	a.dispatch(keymap.ActionFileIndexReveal)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == dir && !p.ListingPending
	})
	if e, ok := a.activePanel().CurrentEntry(); !ok || e.Name != "harbor.txt" {
		t.Fatalf("revealed cursor = %+v", e)
	}
}

func TestFileIndexOutdatedIndexAsksToUpgrade(t *testing.T) {
	screen := newScreen(t, 100, 30)
	opts := testOptions(t.TempDir())
	dbPath := opts.Config.FileIndex.Database
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, nil, 0o644); err != nil { // never initialised: needs `init`
		t.Fatal(err)
	}
	a := newTestApp(t, screen, opts)
	t.Cleanup(func() { _ = a.fileIndex.Close() })

	a.dispatch(keymap.ActionFileIndexTagAdd)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		fd := a.model.FileDialog
		return fd.Open && fd.DialogType == dialog.FileDialogIndexInit
	})
	if !a.model.FileDialog.IndexUpgrade || a.model.FileDialog.Message != dbPath {
		t.Fatalf("prompt = upgrade %v at %q, want the upgrade prompt for %q", a.model.FileDialog.IndexUpgrade, a.model.FileDialog.Message, dbPath)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitTagPrompt(t, a, screen)
	if !strings.HasPrefix(a.model.Message, "Upgraded fileindex database") {
		t.Fatalf("message = %q", a.model.Message)
	}
	if _, err := a.fileIndex.DB(); err != nil {
		t.Fatalf("after upgrade = %v", err)
	}
}

func TestFileIndexPathShownWithIconPrefix(t *testing.T) {
	a := newHelpEntriesApp(t)
	a.config.UI.UseNerdfontIcons = true
	a.setStyles(a.styles)
	if got := ui.PanelTitlePath("fileindex:///srv/orchard", "", 40); got != a.styles.IconFileIndex()+":///srv/orchard" {
		t.Fatalf("with icons = %q", got)
	}
	a.config.UI.UseNerdfontIcons = false
	a.setStyles(a.styles)
	if got := ui.PanelTitlePath("fileindex:///srv/orchard", "", 40); got != "fileindex:///srv/orchard" {
		t.Fatalf("without icons = %q", got)
	}
}

func TestFileIndexMissingIndexAsksToCreate(t *testing.T) {
	screen := newScreen(t, 100, 30)
	dir := t.TempDir()
	opts := testOptions(dir)
	dbPath := opts.Config.FileIndex.Database
	a := newTestApp(t, screen, opts)
	t.Cleanup(func() { _ = a.fileIndex.Close() })
	initPrompt := func() bool {
		fd := a.model.FileDialog
		return fd.Open && fd.DialogType == dialog.FileDialogIndexInit
	}

	// No: nothing is created and the action does not run.
	a.dispatch(keymap.ActionFileIndexOpen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, initPrompt)
	if a.model.FileDialog.Message != dbPath {
		t.Fatalf("prompt path = %q, want %q", a.model.FileDialog.Message, dbPath)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.model.FileDialog.Open {
		t.Fatal("Esc should close the prompt")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("declining must not create the database")
	}
	if a.activePanel().Path.Scheme() == pathloc.SchemeFileIndex {
		t.Fatal("panel must stay put without an index")
	}

	// Yes: the database is created and the waiting action continues.
	a.dispatch(keymap.ActionFileIndexTagAdd)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, initPrompt)
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.FileDialog.Open && a.model.FileDialog.DialogType == dialog.FileDialogIndexTags
	})
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database not created: %v", err)
	}
	if !strings.HasPrefix(a.model.Message, "Created fileindex database") {
		t.Fatalf("message = %q", a.model.Message)
	}
	if a.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("view = %v", a.model.ViewMode)
	}
}

func TestFileIndexHostPathFeatures(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "meadow.txt")
	a.dispatch(keymap.ActionFileIndexOpen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.Path.Scheme() == pathloc.SchemeFileIndex && !p.ListingPending
	})
	cursorTo(t, a, "meadow.txt")
	want := filepath.Join(dir, "meadow.txt")

	if got, err := a.resolveLocalEditFilePath(); err != nil || got != want {
		t.Fatalf("edit path = %q, %v; want %q", got, err, want)
	}
	if got := shellInsertPathList(a.activePanel()); !slices.Equal(got, []string{want}) {
		t.Fatalf("send paths = %v, want %v", got, want)
	}
	if got := a.localActivePanelDir(); got != dir {
		t.Fatalf("panel dir = %q, want %q", got, dir)
	}
	gone := filepath.Join(dir, "willow", "birch")
	if got := existingHostDir(gone); got != dir {
		t.Fatalf("existingHostDir(%q) = %q, want %q", gone, got, dir)
	}
}

// The fileindex menu dims the browse action for the side the active panel is already on.
func TestFileIndexMenuDimsCurrentBrowseSide(t *testing.T) {
	a := newHelpEntriesApp(t)
	dimmed := func() map[string]bool {
		out := map[string]bool{}
		for _, it := range a.model.LeaderMenu.Items {
			if it.GroupTitle == "" && it.Disabled {
				out[it.Label] = true
			}
		}
		return out
	}
	for _, tc := range []struct {
		dir, want string
	}{
		{"/srv/orchard", "Browse filesystem"},
		{"fileindex:///srv/orchard", "Browse index"},
	} {
		p, err := pathloc.Parse(tc.dir)
		if err != nil {
			t.Fatal(err)
		}
		a.activePanel().Path = p
		a.toggleFileIndexMenu()
		if got := dimmed(); len(got) != 1 || !got[tc.want] {
			t.Errorf("%s: dimmed = %v, want only %q", tc.dir, got, tc.want)
		}
		a.closeLeaderMenu()
	}
}

// The scan row names what fileindex.scan targets: the selection, else the directory.
func TestFileIndexMenuScanLabel(t *testing.T) {
	a := newHelpEntriesApp(t)
	scanLabel := func() string {
		a.toggleFileIndexMenu()
		defer a.closeLeaderMenu()
		for _, it := range a.model.LeaderMenu.Items {
			if it.Key == 's' {
				return it.Label
			}
		}
		return ""
	}
	if got := scanLabel(); got != "Scan directory" {
		t.Fatalf("no selection: %q", got)
	}
	a.activePanel().AddSelection(filepath.Join(a.activePanel().PathString(), "meadow"))
	if got := scanLabel(); got != "Scan selection" {
		t.Fatalf("with selection: %q", got)
	}
}
