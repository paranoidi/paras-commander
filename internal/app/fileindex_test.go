package app

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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
	keymap.ActionFileIndexScan,
	keymap.ActionFileIndexPrune,
	keymap.ActionFileIndexTagAdd,
	keymap.ActionFileIndexTagRemove,
}

func TestFileIndexActionsInHelpAndMenus(t *testing.T) {
	a := newHelpEntriesApp(t)
	var inHelp []string
	for _, e := range a.buildHelpEntries() {
		if strings.HasPrefix(e.ActionID, "fileindex.") {
			if e.Section != "fileindex" || e.Keys == "" {
				t.Errorf("help entry %+v, want section fileindex with keys", e)
			}
			inHelp = append(inHelp, e.ActionID)
		}
	}
	for _, id := range append([]string{keymap.ActionFileIndexMenu}, fileIndexActions...) {
		if !slices.Contains(inHelp, id) {
			t.Errorf("%s missing from help", id)
		}
	}

	// The fileindex menu lists every action under the letter of its default chord: lowercase
	// for Ctrl+letter, uppercase for Alt+Shift+letter.
	var inMenu []string
	for _, e := range a.keys.FileIndexMenuEntries() {
		inMenu = append(inMenu, e.ActionID)
		ch, err := keymap.ParseKey(a.keys.Global.MenuBindingLabel(e.ActionID))
		if err != nil {
			t.Fatal(err)
		}
		if ch.Rune != e.Key && rune('a'+ch.Key-tcell.KeyCtrlA) != e.Key {
			t.Errorf("%s: menu key %q does not match its chord %s", e.ActionID, e.Key, keymap.FormatChord(ch))
		}
	}
	if !slices.Equal(inMenu, fileIndexActions) {
		t.Fatalf("fileindex menu = %v, want %v", inMenu, fileIndexActions)
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
	a, _, _, _ := fileIndexApp(t)
	cursorTo(t, a, "harbor.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
	if !a.leaderMenuOpen(ui.LeaderMenuFileIndex) {
		t.Fatalf("leader menu = %+v, want the fileindex menu", a.model.LeaderMenu)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	if a.model.LeaderMenu.Open {
		t.Fatal("activating an entry should close the menu")
	}
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

func TestFileIndexMissingIndexReportedOnce(t *testing.T) {
	screen := newScreen(t, 100, 30)
	a := newTestApp(t, screen, testOptions(t.TempDir()))
	a.dispatch(keymap.ActionFileIndexOpen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return strings.Contains(a.model.Message, "no fileindex database")
	})
	if a.activePanel().Path.Scheme() == pathloc.SchemeFileIndex {
		t.Fatal("panel must stay put without an index")
	}
	if a.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("view = %v", a.model.ViewMode)
	}
}
