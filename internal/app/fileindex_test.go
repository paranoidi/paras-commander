package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/fileindex/db"
	fileindexctrl "github.com/paranoidi/paras-commander/internal/apphandler/fileindex"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/config"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

var fileIndexActions = []string{
	keymap.ActionFileIndexBrowseWith,
	keymap.ActionFileIndexBrowse,
	keymap.ActionFileIndexReveal,
	keymap.ActionFileIndexTags,
	keymap.ActionFileIndexFind,
	keymap.ActionFileIndexReportDifferences,
	keymap.ActionFileIndexScan,
	keymap.ActionFileIndexScanFast,
	keymap.ActionFileIndexPrune,
	keymap.ActionFileIndexFindDuplicates,
	keymap.ActionFileIndexCopies,
	keymap.ActionFileIndexCopiesOutside,
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
	// every fileindex chord is Alt+Shift+letter. Fast scan takes uppercase of scan's letter instead,
	// and find duplicates uppercase of find's.
	var inMenu, groups []string
	groupColumn := map[string]int{}
	for _, e := range a.keys.FileIndexMenuEntries() {
		if e.ActionID == keymap.ActionPanelFindDuplicates {
			t.Error("panel.find-duplicates must not be in the fileindex menu")
		}
		if e.GroupTitle != "" {
			groups = append(groups, e.GroupTitle)
			groupColumn[e.GroupTitle] = e.GroupColumn
			continue
		}
		inMenu = append(inMenu, e.ActionID)
		ch, err := keymap.ParseKey(a.keys.Global.MenuBindingLabel(e.ActionID))
		if err != nil {
			t.Fatal(err)
		}
		wantKey := unicode.ToLower(ch.Rune)
		switch e.ActionID {
		case keymap.ActionFileIndexScanFast:
			wantKey = 'S'
		case keymap.ActionFileIndexFindDuplicates:
			wantKey = 'F'
		}
		if ch.Mod != tcell.ModAlt || !unicode.IsUpper(ch.Rune) || wantKey != e.Key {
			t.Errorf("%s: menu key %q does not match its chord %s", e.ActionID, e.Key, keymap.FormatChord(ch))
		}
	}
	if !slices.Equal(inMenu, fileIndexActions) {
		t.Fatalf("fileindex menu = %v, want %v", inMenu, fileIndexActions)
	}
	if want := []string{"Browse", "Tagging", "Tools", "Actions", "Duplicates"}; !slices.Equal(groups, want) {
		t.Fatalf("fileindex menu groups = %v, want %v", groups, want)
	}
	if groupColumn["Tools"] != groupColumn["Tagging"] {
		t.Fatalf("Tools must stack under Tagging: columns = %v", groupColumn)
	}

	// The `:` leader menu reaches the fileindex menu, not the actions themselves.
	var leader []string
	for _, e := range a.keys.LeaderMenuEntries() {
		leader = append(leader, e.ActionID)
	}
	if !slices.Contains(leader, keymap.ActionPanelFindDuplicates) {
		t.Fatalf("leader menu lost panel.find-duplicates: %v", leader)
	}
	if !slices.Contains(leader, keymap.ActionFileIndexMenu) || slices.Contains(leader, keymap.ActionFileIndexScan) {
		t.Fatalf("leader menu actions = %v", leader)
	}
}

// The menu's letters are case-sensitive: s is the plain scan, S the fast one.
func TestFileIndexMenuScanLetters(t *testing.T) {
	for key, want := range map[rune]string{'s': "fileindex scan queued", 'S': "fileindex fast scan queued"} {
		a, screen, _, _ := fileIndexApp(t)
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone))
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message == want })
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
	a.fileindexCtrl.CloseTagDialog()
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
	return fileIndexAppWith(t, nil)
}

// fileIndexAppWith is fileIndexApp with tune applied to the options before the app is built.
func fileIndexAppWith(t *testing.T, tune func(*Options)) (*App, tcell.SimulationScreen, string, string) {
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
	if err := d.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CommitScanBatch(batch); err != nil {
		t.Fatal(err)
	}
	_ = d.Close()

	screen := newScreen(t, 100, 30)
	opts := testOptions(dir)
	opts.Config.FileIndex.Database = dbPath
	if tune != nil {
		tune(&opts)
	}
	a := newTestApp(t, screen, opts)
	t.Cleanup(func() { _ = a.fileIndex.Close() })
	return a, screen, dir, dbPath
}

// waitTagPrompt lands the tag prompt, which opens once the index check comes back.
func waitTagPrompt(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.IndexTagsDialog.Open
	})
}

func typeTags(a *App, text string) {
	for _, r := range text {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
}

func altKey(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt) }

// seedTags tags entries of dir before the dialog opens.
func seedTags(t *testing.T, a *App, dir string, tags []string, names ...string) {
	t.Helper()
	var paths []string
	for _, n := range names {
		paths = append(paths, filepath.Join(dir, n))
	}
	if _, err := a.fileIndex.EditTags(paths, tags, true, false); err != nil {
		t.Fatal(err)
	}
}

func harborTags(t *testing.T, a *App, dir string) map[string][]string {
	t.Helper()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	tags, err := d.EntryTags(dir, []string{"harbor.txt", "meadow.txt"})
	if err != nil {
		t.Fatal(err)
	}
	return tags
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

	a.dispatch(keymap.ActionFileIndexTags)
	waitTagPrompt(t, a, screen)
	if got := a.model.IndexTagsDialog.Targets; !slices.Equal(got, []string{filepath.Join(dir, "harbor.txt")}) {
		t.Fatalf("targets = %v", got)
	}
	typeTags(a, "clover, field")
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !a.model.IndexTagsDialog.Open {
		t.Fatal("Enter in the input should keep the dialog open")
	}
	if a.model.IndexTagsDialog.Input != "" {
		t.Fatalf("input = %q, want it cleared", a.model.IndexTagsDialog.Input)
	}
	a.handleKey(altKey('o'))
	if a.model.IndexTagsDialog.Open {
		t.Fatal("OK should close the dialog")
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

func TestFileIndexTagEditorShowsAndTogglesTags(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	seedTags(t, a, dir, []string{"amber"}, "harbor.txt", "meadow.txt")
	seedTags(t, a, dir, []string{"birch"}, "harbor.txt")
	seedTags(t, a, dir, []string{"cedar"}, "meadow.txt")
	cursorTo(t, a, "harbor.txt")

	a.dispatch(keymap.ActionFileIndexTags)
	waitTagPrompt(t, a, screen)
	st := &a.model.IndexTagsDialog
	want := []dialog.IndexTag{
		{Name: "amber", Initial: dlg.CheckOn, State: dlg.CheckOn},
		{Name: "birch", Initial: dlg.CheckOn, State: dlg.CheckOn},
		{Name: "cedar"},
	}
	if !slices.Equal(st.Tags, want) {
		t.Fatalf("tags = %+v", st.Tags)
	}
	checkDialogInvariants(t, a)
	// Into the grid, then jump to and swap two tags with the keyboard.
	a.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if st.Focus != dialog.IndexTagsGrid {
		t.Fatalf("focus = %q, want the grid", st.Focus)
	}
	checkDialogInvariants(t, a)
	typeTags(a, "c")
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if !st.Open || st.JumpQuery != "" {
		t.Fatalf("Esc should clear the jump text first: open %v, query %q", st.Open, st.JumpQuery)
	}
	typeTags(a, "b ") // Space toggles and ends this search...
	if st.JumpQuery != "" {
		t.Fatalf("jump text after Space = %q", st.JumpQuery)
	}
	typeTags(a, "c") // ...so the next letter starts a new one, and Enter ends it the same way.
	if st.JumpQuery != "c" {
		t.Fatalf("jump text = %q", st.JumpQuery)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if st.JumpQuery != "" {
		t.Fatalf("jump text after Enter = %q", st.JumpQuery)
	}
	a.handleKey(altKey('o'))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.Message == "Added 1 tag, removed 1 tag on 1 entry"
	})
	if got := harborTags(t, a, dir); !slices.Equal(got["harbor.txt"], []string{"amber", "cedar"}) {
		t.Fatalf("tags = %v", got)
	}
}

func TestFileIndexTagEditorArrowsLeaveTheGrid(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	seedTags(t, a, dir, []string{"amber", "birch", "cedar"}, "harbor.txt")
	cursorTo(t, a, "harbor.txt")
	a.dispatch(keymap.ActionFileIndexTags)
	waitTagPrompt(t, a, screen)
	st := &a.model.IndexTagsDialog
	press := func(k tcell.Key, want dlg.ID) {
		t.Helper()
		a.handleKey(tcell.NewEventKey(k, 0, tcell.ModNone))
		if st.Focus != want {
			t.Fatalf("focus = %q, want %q", st.Focus, want)
		}
	}
	press(tcell.KeyDown, dialog.IndexTagsGrid)
	press(tcell.KeyUp, dialog.IndexTagsInput)
	press(tcell.KeyDown, dialog.IndexTagsGrid)
	press(tcell.KeyDown, dialog.IndexTagsGrid) // the second row
	press(tcell.KeyDown, dialog.IndexTagsRecursive)
	press(tcell.KeyDown, dlg.OK)
	press(tcell.KeyUp, dialog.IndexTagsRecursive)
	press(tcell.KeyUp, dialog.IndexTagsGrid)

	// One jump match leaves nothing to cycle: arrows move on and drop the jump text.
	typeTags(a, "cedar")
	press(tcell.KeyDown, dialog.IndexTagsRecursive)
	if st.JumpQuery != "" {
		t.Fatalf("jump text = %q", st.JumpQuery)
	}
}

func TestFileIndexTagEditorPlainLettersOutsideTheGrid(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	seedTags(t, a, dir, []string{"orchid"}, "harbor.txt")
	cursorTo(t, a, "harbor.txt")
	open := func() *dialog.IndexTagsDialogState {
		t.Helper()
		a.dispatch(keymap.ActionFileIndexTags)
		waitTagPrompt(t, a, screen)
		st := &a.model.IndexTagsDialog
		a.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // the grid
		return st
	}

	st := open()
	typeTags(a, "o") // a jump in the grid, not OK
	if !st.Open || st.JumpQuery != "o" {
		t.Fatalf("open %v, jump text %q", st.Open, st.JumpQuery)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // Recursive
	typeTags(a, "r")
	if st.Focus != dialog.IndexTagsRecursive || !st.Recursive {
		t.Fatalf("focus %q, recursive %v", st.Focus, st.Recursive)
	}
	typeTags(a, "c")
	if st.Open {
		t.Fatal("c on Recursive should press Cancel")
	}

	st = open()
	a.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // Recursive
	a.handleKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // the buttons
	typeTags(a, "o")
	if st.Open {
		t.Fatal("o on the buttons should press OK")
	}
}

func TestFileIndexTagEditorCancelDiscards(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "harbor.txt")
	a.dispatch(keymap.ActionFileIndexTags)
	waitTagPrompt(t, a, screen)
	typeTags(a, "clover")
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.model.IndexTagsDialog.Open {
		t.Fatal("Esc should close the dialog")
	}
	if got := harborTags(t, a, dir); len(got) != 0 {
		t.Fatalf("tags = %v, want nothing written", got)
	}
}

func TestFileIndexTagEditorKeepsMixedTag(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	seedTags(t, a, dir, []string{"amber"}, "harbor.txt")
	p := a.activePanel()
	p.SelectedPaths = map[string]bool{filepath.Join(dir, "harbor.txt"): true, filepath.Join(dir, "meadow.txt"): true}

	a.dispatch(keymap.ActionFileIndexTags)
	waitTagPrompt(t, a, screen)
	if got := a.model.IndexTagsDialog.Tags[0].State; got != dlg.CheckMixed {
		t.Fatalf("state = %v, want mixed", got)
	}
	typeTags(a, "clover")
	a.handleKey(altKey('o'))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return strings.HasPrefix(a.model.Message, "Added")
	})
	got := harborTags(t, a, dir)
	if !slices.Equal(got["harbor.txt"], []string{"amber", "clover"}) || !slices.Equal(got["meadow.txt"], []string{"clover"}) {
		t.Fatalf("tags = %v", got)
	}
}

func TestFileIndexTagEditorScansUnknownWithAddedTagsOnly(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("willow"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedTags(t, a, dir, []string{"amber"}, "harbor.txt")
	a.fileindexCtrl.HandleTags(fileindexctrl.TagsPayload{
		Added: []string{"clover"}, AddRes: fi.TagEdit{Unknown: []string{filepath.Join(dir, "willow.txt")}},
	})
	if a.model.Message != "Scanning 1 entry to tag" {
		t.Fatalf("message = %q", a.model.Message)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		d, err := a.fileIndex.DB()
		if err != nil {
			return false
		}
		tags, err := d.EntryTags(dir, []string{"willow.txt"})
		return err == nil && slices.Equal(tags["willow.txt"], []string{"clover"})
	})
}

func TestFileIndexOpenAndReveal(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "meadow.txt")

	a.dispatch(keymap.ActionFileIndexBrowse)
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

	a.dispatch(keymap.ActionFileIndexTags)
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
	if got := ui.PanelTitlePath("fileindex:///srv/orchard", "", 40); got != a.styles.IconFileIndex()+" /srv/orchard" {
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
	a.dispatch(keymap.ActionFileIndexBrowse)
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
	a.dispatch(keymap.ActionFileIndexTags)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, initPrompt)
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	waitTagPrompt(t, a, screen)
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
	a.dispatch(keymap.ActionFileIndexBrowse)
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
	var opened []string
	oldOpen := runDetachedXDGOpen
	runDetachedXDGOpen = func(p string) error { opened = append(opened, p); return nil }
	t.Cleanup(func() { runDetachedXDGOpen = oldOpen })
	a.config.Panels.OpenFilesExternally = true
	a.openFileExternally(pathloc.FileIndexPrefix + want)
	if !slices.Equal(opened, []string{want}) {
		t.Fatalf("external open = %v, want %v", opened, want)
	}
	gone := filepath.Join(dir, "willow", "birch")
	if got := existingHostDir(gone); got != dir {
		t.Fatalf("existingHostDir(%q) = %q, want %q", gone, got, dir)
	}
}

// The fileindex menu dims Browse filesystem on a plain panel; Browse index toggles, so it never
// dims.
func TestFileIndexMenuDimsReveal(t *testing.T) {
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
		dir       string
		withIndex bool
		want      string
	}{
		{"/srv/orchard", false, "Browse filesystem"},
		{"/srv/orchard", true, ""},
		{"fileindex:///srv/orchard", false, ""},
	} {
		p, err := pathloc.Parse(tc.dir)
		if err != nil {
			t.Fatal(err)
		}
		a.activePanel().Path = p
		a.activePanel().WithIndex = tc.withIndex
		a.toggleFileIndexMenu()
		if got := dimmed(); (tc.want == "" && len(got) != 0) || (tc.want != "" && (len(got) != 1 || !got[tc.want])) {
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

// TestFileIndexScanClearsSelection checks both scans run on the selection and then drop it.
func TestFileIndexScanClearsSelection(t *testing.T) {
	for _, action := range []string{keymap.ActionFileIndexScan, keymap.ActionFileIndexScanFast} {
		t.Run(action, func(t *testing.T) {
			a, screen, dir, _ := fileIndexApp(t)
			selected := filepath.Join(dir, "harbor.txt")
			a.activePanel().AddSelection(selected)
			a.dispatch(action) // queued once the index is open, off the UI thread
			drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
				return a.activePanel().SelectedPathCount() == 0
			})
			all := a.jobState.AllJobs()
			if len(all) != 1 || len(all[0].Sources) != 1 || all[0].Sources[0].String() != selected {
				t.Fatalf("jobs = %+v, want one scan of the selection", all)
			}
		})
	}
}

// openIndexFind browses the app's directory through fileindex:// and opens the find dialog there.
func openIndexFind(t *testing.T, a *App, screen tcell.SimulationScreen, dir string) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
	a.dispatch(keymap.ActionPanelFindDialog)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexFindDialog.Open })
}

func typeIndexFindQuery(t *testing.T, a *App, screen tcell.SimulationScreen, query string, want int) {
	t.Helper()
	for _, r := range query {
		a.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		st := a.model.IndexFindDialog
		return !st.Searching && len(st.Results) == want
	})
}

func TestFindDialogOnFileIndexPanelOpensIndexFind(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	if a.model.FindDialog.Open || a.inputMode() != InputModeIndexFindDialog {
		t.Fatalf("find open=%v mode=%v, want only the index find dialog", a.model.FindDialog.Open, a.inputMode())
	}
	if st := a.model.IndexFindDialog; st.Root != dir || st.JumpMode {
		t.Fatalf("state = %+v", st)
	}
	typeIndexFindQuery(t, a, screen, "HARBOR", 1)
	if r := a.model.IndexFindDialog.Results[0]; r.Path != filepath.Join(dir, "harbor.txt") || r.Rel != "harbor.txt" {
		t.Fatalf("result = %+v", r)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if a.model.IndexFindDialog.Open {
		t.Fatal("Esc should close the dialog")
	}
}

func TestFindDialogOnLocalPanelStillOpensFind(t *testing.T) {
	a, _, _, _ := fileIndexApp(t)
	a.dispatch(keymap.ActionPanelFindDialog)
	if !a.model.FindDialog.Open || a.model.IndexFindDialog.Open {
		t.Fatal("a local panel must open the walking Find dialog")
	}
}

func TestIndexFindIgnoresStaleResults(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	typeIndexFindQuery(t, a, screen, "harbor", 1)
	st := &a.model.IndexFindDialog
	a.fileindexCtrl.HandleFind(fileindexctrl.FindPayload{Gen: st.Gen - 1, Results: []dialog.IndexFindResult{{Path: "/x", Rel: "x"}, {Path: "/y", Rel: "y"}}})
	if len(st.Results) != 1 {
		t.Fatalf("a stale result replaced the list: %+v", st.Results)
	}
	a.fileindexCtrl.HandleFind(fileindexctrl.FindPayload{Gen: st.Gen, Results: []dialog.IndexFindResult{{Path: "/x", Rel: "x"}}})
	if len(st.Results) != 1 || st.Results[0].Path != "/x" {
		t.Fatalf("the current result was dropped: %+v", st.Results)
	}
}

func TestIndexFindEnterWithMarksAddsSelections(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	typeIndexFindQuery(t, a, screen, ".txt", 2)
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if n := a.model.IndexFindDialog.MarkedCount(); n != 2 {
		t.Fatalf("marked = %d, want 2", n)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if a.model.IndexFindDialog.Open {
		t.Fatal("Enter should close the dialog")
	}
	sel := a.activePanel().SelectedPaths
	for _, name := range []string{"harbor.txt", "meadow.txt"} {
		if !sel["fileindex://"+filepath.Join(dir, name)] {
			t.Errorf("%s not selected: %v", name, sel)
		}
	}
	if a.model.Message != "Added 2 to selection" {
		t.Fatalf("message = %q", a.model.Message)
	}
}

func TestFileIndexFindFromLocalPanelSelectsHostPaths(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	a.dispatch(keymap.ActionFileIndexFind)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexFindDialog.Open })
	if st := a.model.IndexFindDialog; st.Root != dir || st.FromIndexPanel {
		t.Fatalf("root = %q fromIndex = %v, want %q local", st.Root, st.FromIndexPanel, dir)
	}
	typeIndexFindQuery(t, a, screen, ".txt", 2)
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if sel := a.activePanel().SelectedPaths; !sel[filepath.Join(dir, "harbor.txt")] {
		t.Fatalf("harbor.txt not selected as a plain path: %v", sel)
	}
}

func TestIndexFindEnterWithoutMarksNavigates(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	typeIndexFindQuery(t, a, screen, "meadow", 1)
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		e, ok := a.activePanel().CurrentEntry()
		return ok && e.Name == "meadow.txt" && !a.activePanel().ListingPending
	})
	if a.model.IndexFindDialog.Open || a.activePanel().PathString() != "fileindex://"+dir {
		t.Fatalf("open=%v path=%s", a.model.IndexFindDialog.Open, a.activePanel().PathString())
	}
}

func TestIndexFindPinToggleOnResult(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	typeIndexFindQuery(t, a, screen, "harbor", 1)
	want := "fileindex://" + filepath.Join(dir, "harbor.txt")
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt))
	if len(a.model.PinnedItems) != 1 || a.model.PinnedItems[0].Path != want || !a.model.IndexFindDialog.Open {
		t.Fatalf("pinned = %+v open=%v, want %s pinned and the dialog open", a.model.PinnedItems, a.model.IndexFindDialog.Open, want)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt))
	if len(a.model.PinnedItems) != 0 {
		t.Fatalf("second toggle should unpin: %+v", a.model.PinnedItems)
	}
}

func TestIndexFindGroupSelectAndParentDirs(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexFind(t, a, screen, dir)
	typeIndexFindQuery(t, a, screen, ".txt", 2)
	req := findctrl.GroupSelectRequest{Mode: findctrl.GroupSelectModeSelect, Pattern: "harb*"}
	if files, dirs := a.fileindexCtrl.CountGroupMatches(req, false); files != 1 || dirs != 0 {
		t.Fatalf("preview = %d files %d dirs, want 1 file", files, dirs)
	}
	a.fileindexCtrl.ApplyGroupSelect(req)
	st := a.model.IndexFindDialog
	if st.MarkedCount() != 1 || !st.Marked[filepath.Join(dir, "harbor.txt")] {
		t.Fatalf("marked = %v", st.Marked)
	}
	req.Mode = findctrl.GroupSelectModeUnselect
	a.fileindexCtrl.ApplyGroupSelect(req)
	if a.model.IndexFindDialog.MarkedCount() != 0 {
		t.Fatalf("unselect left marks: %v", a.model.IndexFindDialog.Marked)
	}
	// every result already sits in the panel's own directory: nothing to select
	a.handleKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if a.model.Message != "Already in the containing directory" || !a.model.IndexFindDialog.Open {
		t.Fatalf("message = %q open=%v", a.model.Message, a.model.IndexFindDialog.Open)
	}
	// F6 opens the group dialog in its index context
	a.handleKey(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone))
	if gs := a.model.GroupSelect; !gs.Open || gs.Context != "index-find" {
		t.Fatalf("group select = %+v", gs)
	}
}

func TestIndexFindSearchOnlySelectionsAndHiddenDepth(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	sub := filepath.Join(dir, "orchard")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("c", 32)
	if _, err := d.CommitScanBatch([]db.ScannedFile{{Dir: sub, Name: "thistle.txt", Size: 1, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
	a.activePanel().BulkAddSelections([]string{"fileindex://" + sub}, func(string) bool { return true })
	a.dispatch(keymap.ActionPanelFindDialog)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.IndexFindDialog.Open })
	st := &a.model.IndexFindDialog
	if !st.ShowSearchSelectionsOption || !slices.Equal(st.SelectionDirRoots, []string{sub}) {
		t.Fatalf("selection scope = %v %v", st.ShowSearchSelectionsOption, st.SelectionDirRoots)
	}
	a.fileindexCtrl.ApplyFindEvent(dlgEvent(dialog.IndexFindSelections))
	typeIndexFindQuery(t, a, screen, ".txt", 1) // only orchard/thistle.txt, not the panel directory's files
	if !st.SearchOnlySelections || st.Results[0].Rel != "orchard/thistle.txt" {
		t.Fatalf("scoped = %v results = %+v", st.SearchOnlySelections, st.Results)
	}
	// depth 1 keeps direct children; the radios and the hidden checkbox rerun the query
	st.Focus = dialog.IndexFindDepth
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	if st.MaxDepth != 1 || st.Focus != dialog.IndexFindQuery {
		t.Fatalf("depth = %d focus = %q", st.MaxDepth, st.Focus)
	}
}

func dlgEvent(id dlg.ID) dlg.Event { return dlg.Event{ID: id} }

// Sizing an index directory (panel.dir-size) fills its size column from the index.
func TestFileIndexDirSizeFromIndex(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	parent := pathloc.FileIndexPrefix + filepath.Dir(dir)
	if err := a.navigatePanelToDirectory(a.model.ActivePanel, parent, filepath.Base(dir)); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == parent && !p.ListingPending
	})
	cursorTo(t, a, filepath.Base(dir))
	entry, _ := a.activePanel().CurrentEntry()
	a.scanCursorDirSize()
	want := int64(len("harbor.txt") + len("meadow.txt"))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		n, ok := a.disk.engine.ByteSize(entry.Path)
		return ok && n == want
	})
	if n, _ := a.disk.engine.FileCount(entry.Path); n != 2 {
		t.Fatalf("file count = %d, want 2", n)
	}
}

// browseWith turns the overlay on for the active panel and waits for its listing.
func browseWith(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexBrowseWith)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.WithIndex && !p.ListingPending && p.Path.IsLocal()
	})
}

func entryStatus(a *App, name string) (string, bool) {
	for _, e := range a.activePanel().Entries {
		if e.Name == name {
			return e.IndexStatus, true
		}
	}
	return "", false
}

func TestFileIndexStartBrowseWith(t *testing.T) {
	a, _, _, _ := fileIndexApp(t)
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if a.panelByID(id).WithIndex {
			t.Fatalf("panel %d starts with the overlay by default", id)
		}
	}

	a, screen, _, _ := fileIndexAppWith(t, func(o *Options) { o.Config.FileIndex.StartBrowseWith = true })
	for _, id := range []int{ui.PrimaryPanel, ui.SecondaryPanel} {
		if !a.panelByID(id).WithIndex {
			t.Fatalf("panel %d does not start in Browse with fileindex", id)
		}
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		// The fixture's recorded mtime differs from the file's, so the row reads "changed";
		// any status proves the overlay ran (a plain listing leaves it empty).
		st, _ := entryStatus(a, "harbor.txt")
		return st != ""
	})
	a.render() // Run's first render reconciles the panels, which switches the default columns on
	cols := a.model.MetaResults[a.model.ActivePanel]
	if len(cols) != 1 || cols[0].EntryName != config.FileIndexTagsColumn {
		t.Fatalf("meta columns = %+v, want the %s column", cols, config.FileIndexTagsColumn)
	}
}

func TestFileIndexBrowseWithOverlayAndToggle(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionPanelRefresh)
	browseWith(t, a, screen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		_, ok := entryStatus(a, "meadow.txt")
		return ok
	})
	if st, _ := entryStatus(a, "meadow.txt"); st != "missing" {
		t.Fatalf("meadow.txt (gone from disk) = %q, want a missing ghost row", st)
	}
	if st, _ := entryStatus(a, "willow.txt"); st != "new" {
		t.Fatalf("willow.txt (not indexed) = %q, want new", st)
	}
	if a.activePanel().GitColumnActive {
		t.Fatal("the index strip replaces git status while WithIndex is on")
	}

	// The mode survives navigation to another local directory.
	sub := filepath.Join(dir, "lantern")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.activePanel().Load(sub); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == sub && !p.ListingPending
	})
	if !a.activePanel().WithIndex {
		t.Fatal("WithIndex must persist across chdir")
	}

	// Toggling off drops the overlay (and the ghost rows with it).
	a.dispatch(keymap.ActionFileIndexBrowseWith)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return !a.activePanel().WithIndex })
}

// Browse filesystem leaves Browse with fileindex: the panel stays on its directory, without the
// overlay and its ghost rows.
func TestFileIndexRevealLeavesBrowseWith(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionPanelRefresh)
	browseWith(t, a, screen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		_, ok := entryStatus(a, "meadow.txt")
		return ok
	})

	a.dispatch(keymap.ActionFileIndexReveal)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		_, ghost := entryStatus(a, "meadow.txt")
		return !ghost && !a.activePanel().ListingPending
	})
	p := a.activePanel()
	if p.WithIndex || p.PathString() != dir {
		t.Fatalf("WithIndex = %v, path = %s, want off on %s", p.WithIndex, p.PathString(), dir)
	}
}

func TestFileIndexBrowseWithRefusesSFTP(t *testing.T) {
	a, screen, _, _ := fileIndexApp(t)
	a.activePanel().Path = pathloc.MustParse("sftp://paras@example.invalid/srv")
	a.dispatch(keymap.ActionFileIndexBrowseWith)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message != "" })
	if a.activePanel().WithIndex {
		t.Fatal("browse-with must not turn on for an sftp panel")
	}
	if !strings.Contains(a.model.Message, "local paths only") {
		t.Fatalf("message = %q", a.model.Message)
	}
}

func TestFileIndexGhostRowRefusesCopyAndDeletesFromIndex(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionPanelRefresh)
	browseWith(t, a, screen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		_, ok := entryStatus(a, "meadow.txt")
		return ok
	})
	cursorTo(t, a, "meadow.txt")

	a.dispatch(keymap.ActionCopy)
	if a.model.TransferDialog.Open {
		t.Fatal("copy of a ghost row must be refused")
	}
	if !strings.Contains(a.model.Message, "Only in the index: meadow.txt") {
		t.Fatalf("message = %q", a.model.Message)
	}

	// Delete a local file together with the ghost row: the real file goes from disk, the ghost row from the index.
	p := a.activePanel()
	p.SelectedPaths = map[string]bool{
		filepath.Join(dir, "harbor.txt"):                           true,
		pathloc.FileIndexPrefix + filepath.Join(dir, "meadow.txt"): true,
	}
	dispatchFileOp(t, a, keymap.ActionFileDelete)
	if a.model.FileDialog.Open {
		if !strings.Contains(a.model.FileDialog.Note, "removed from the index") {
			t.Fatalf("delete note = %q, want the ghost note", a.model.FileDialog.Note)
		}
		a.dialogCtrl.ExecuteDelete()
	}
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	// The delete started on a browse-with panel: the index follows the real delete, and the
	// ghost row goes through the index backend.
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, ghostFound, _ := d.FileInfo(filepath.Join(dir, "meadow.txt"))
		_, realFound, _ := d.FileInfo(filepath.Join(dir, "harbor.txt"))
		_, onDisk := os.Stat(filepath.Join(dir, "harbor.txt"))
		return !ghostFound && !realFound && onDisk != nil
	})
}

// openIndexedParent browses the parent of the fixture directory with fileindex, recording the
// drift requests the panel makes.
func openIndexedParent(t *testing.T, a *App, screen tcell.SimulationScreen, dir string, reqs *[]panel.IndexDriftRequest) {
	t.Helper()
	p := a.activePanel()
	p.ScheduleIndexDrift = func(r panel.IndexDriftRequest) bool { *reqs = append(*reqs, r); return true }
	parent := filepath.Dir(dir)
	if err := a.navigatePanelToDirectory(a.model.ActivePanel, parent, filepath.Base(dir)); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return p.PathString() == parent && !p.ListingPending })
	browseWith(t, a, screen)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		st, _ := entryStatus(a, filepath.Base(dir))
		return st == localfs.IndexStatusIndexed
	})
}

// A plain listing only runs the depth-limited automatic pass; the unlimited pass runs when the
// user asks for a directory size.
func TestFileIndexDirSizeRequestsUnlimitedDrift(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	var reqs []panel.IndexDriftRequest
	openIndexedParent(t, a, screen, dir, &reqs)
	for _, r := range reqs {
		if r.MaxDepth != config.DefaultFileIndexBrowserScanMaxDepth {
			t.Fatalf("listing load requested %+v, want only the automatic depth", r)
		}
	}
	if len(reqs) == 0 {
		t.Fatal("the automatic pass did not run")
	}
	reqs = nil
	cursorTo(t, a, filepath.Base(dir))
	a.scanCursorDirSize()
	if len(reqs) != 1 || reqs[0].MaxDepth != 0 || len(reqs[0].Subdirs) != 1 || reqs[0].Subdirs[0] != filepath.Base(dir) {
		t.Fatalf("drift requests = %+v, want one unlimited request for %s", reqs, filepath.Base(dir))
	}
}

// browser_scan_max_depth = 0 turns the automatic pass off.
func TestFileIndexAutoDriftOff(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	var reqs []panel.IndexDriftRequest
	a.activePanel().IndexAutoDriftDepth = (config.FileIndexConfig{BrowserScanMaxDepth: 0}).AutoDriftDepth()
	openIndexedParent(t, a, screen, dir, &reqs)
	if len(reqs) != 0 {
		t.Fatalf("automatic pass ran with depth 0: %+v", reqs)
	}
}

// Selecting an indexed directory requests its unlimited drift once per listing, even after the
// automatic pass already ran.
func TestFileIndexSelectionRequestsDriftOnce(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	var reqs []panel.IndexDriftRequest
	openIndexedParent(t, a, screen, dir, &reqs)
	reqs = nil
	p := a.activePanel()
	cursorTo(t, a, filepath.Base(dir))
	entry, _ := p.CurrentEntry()
	for range 3 {
		p.TogglePathSelection(entry.Path)
		a.reconcileSelectionSizeScans(a.model.ActivePanel)
	}
	if len(reqs) != 1 || reqs[0].MaxDepth != 0 || reqs[0].Subdirs[0] != filepath.Base(dir) {
		t.Fatalf("drift requests = %+v, want exactly one unlimited request", reqs)
	}
}

// An automatic drift result arriving after an explicit one must not change the status.
func TestFileIndexAutoDriftNeverOverwritesExplicit(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	var reqs []panel.IndexDriftRequest
	openIndexedParent(t, a, screen, dir, &reqs)
	id, p, name := a.model.ActivePanel, a.activePanel(), filepath.Base(dir)
	send := func(explicit bool, status string) {
		a.applyIndexDrift(indexDriftPayload{panelID: id, dir: p.Path, dirs: map[string]string{name: status}, explicit: explicit})
	}
	send(false, localfs.IndexStatusUnchecked)
	if st, _ := entryStatus(a, name); st != localfs.IndexStatusUnchecked {
		t.Fatalf("status = %q, want unchecked", st)
	}
	send(true, localfs.IndexStatusIndexed)
	send(false, localfs.IndexStatusUnchecked)
	if st, _ := entryStatus(a, name); st != localfs.IndexStatusIndexed {
		t.Fatalf("status = %q, a late automatic result must not override the explicit one", st)
	}
}

func TestFileIndexMenuSetsLegendOtherMenusDoNot(t *testing.T) {
	a, _, _, _ := fileIndexApp(t)
	a.dispatchActionLikeKeyboardShortcut(keymap.ActionFileIndexMenu)
	leg := a.model.LeaderMenu.Legend
	if len(leg) != len(fileIndexLegendStates) || leg[0].Label != "scanning" || leg[len(leg)-1].Label != "indexed" {
		t.Fatalf("legend = %+v", leg)
	}
	if want := a.styles.IconFileIndexState("changed"); leg[2].Icon != string(want) {
		t.Fatalf("changed icon = %q, want %q", leg[2].Icon, string(want))
	}
	a.closeLeaderMenu()
	a.dispatchActionLikeKeyboardShortcut(keymap.ActionPanelGitFilterMenu)
	if !a.leaderMenuOpen(ui.LeaderMenuGitFilter) || a.model.LeaderMenu.Legend != nil {
		t.Fatalf("git filter menu = %+v, want no legend", a.model.LeaderMenu)
	}
}

// copiesApp is an app whose cwd holds harbor.txt and thistle.txt, with meadow/harbor.txt a copy
// of the first, all fully hashed in the index.
func copiesApp(t *testing.T) (*App, tcell.SimulationScreen, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	var batch []db.ScannedFile
	for name, letter := range map[string]string{"harbor.txt": "a", "thistle.txt": "b", "meadow/harbor.txt": "a"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := strings.Repeat(letter, 32)
		batch = append(batch, db.ScannedFile{
			Dir: filepath.Join(dir, filepath.Dir(name)), Name: filepath.Base(name), Size: int64(len(name)), Mtime: time.Now(), MD5: sum, SHA256: sum + sum,
		})
	}
	if err := os.WriteFile(filepath.Join(dir, "tundra.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	emptySum := strings.Repeat("c", 32)
	batch = append(batch, db.ScannedFile{Dir: dir, Name: "tundra.txt", Mtime: time.Now(), MD5: emptySum, SHA256: emptySum + emptySum})
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Init(); err != nil {
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
	return a, screen, dir
}

func TestFileIndexCopiesJumpsToChosenCopy(t *testing.T) {
	for _, onIndex := range []bool{false, true} {
		a, screen, dir := copiesApp(t)
		wantDir := filepath.Join(dir, "meadow")
		if onIndex {
			a.dispatch(keymap.ActionFileIndexBrowse)
			drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
				return a.activePanel().PathString() == "fileindex://"+dir && !a.activePanel().ListingPending
			})
			wantDir = "fileindex://" + wantDir
		}
		cursorTo(t, a, "harbor.txt")
		a.dispatch(keymap.ActionFileIndexCopies)
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.PathPicker.Open })
		st := a.model.PathPicker
		if st.Purpose != dialog.PathPickerPurposeNavigateFile || st.IndexPanel != onIndex || st.Title != "Other copies of harbor.txt (1)" ||
			len(st.Items) != 1 || st.Items[0].Path != filepath.Join(dir, "meadow", "harbor.txt") {
			t.Fatalf("onIndex=%v picker = %+v", onIndex, st)
		}
		a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
			p := a.activePanel()
			e, ok := p.CurrentEntry()
			return p.PathString() == wantDir && !p.ListingPending && ok && e.Name == "harbor.txt"
		})
		if a.model.PathPicker.Open {
			t.Fatalf("onIndex=%v: picker still open after choosing", onIndex)
		}
	}
}

func TestFileIndexCopiesMessages(t *testing.T) {
	a, screen, _ := copiesApp(t)
	cursorTo(t, a, "thistle.txt")
	a.dispatch(keymap.ActionFileIndexCopies)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message == "No other copies of thistle.txt" })
	if a.model.PathPicker.Open {
		t.Fatal("no copies must not open the picker")
	}
	cursorTo(t, a, "tundra.txt")
	a.dispatch(keymap.ActionFileIndexCopies)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message == "Empty file, copies not tracked: tundra.txt" })
	if a.model.PathPicker.Open {
		t.Fatal("an empty file must not open the picker")
	}
	cursorTo(t, a, "meadow")
	a.dispatch(keymap.ActionFileIndexCopies)
	if a.model.Message != "Other copies works on files" {
		t.Fatalf("message = %q", a.model.Message)
	}
}

// openIndexPanel shows dir as a fileindex:// panel.
func openIndexPanel(t *testing.T, a *App, screen tcell.SimulationScreen, dir string) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexBrowse)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		p := a.activePanel()
		return p.PathString() == "fileindex://"+dir && !p.ListingPending
	})
}

func TestFileIndexPanelDeleteChangesIndexOnlyAndRefreshes(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	// A file the index does not know sits in the directory; the disk is not touched at all.
	sub := filepath.Join(dir, "lantern")
	if err := os.MkdirAll(filepath.Join(sub, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"lamp.txt", "node_modules/pebble.txt"} {
		if err := os.WriteFile(filepath.Join(sub, f), []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("c", 32)
	if _, err := idx.CommitScanBatch([]db.ScannedFile{{Dir: sub, Name: "lamp.txt", Size: int64(len("lamp.txt")), Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	openIndexPanel(t, a, screen, dir)
	cursorTo(t, a, "lantern")
	dispatchFileOp(t, a, keymap.ActionFileDelete)
	if !a.model.FileDialog.Open || !strings.Contains(a.model.FileDialog.Note, "Removes from the fileindex only") {
		t.Fatalf("delete dialog = open %v, note %q", a.model.FileDialog.Open, a.model.FileDialog.Note)
	}
	a.dialogCtrl.ExecuteDelete()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, indexed, _ := d.FileInfo(filepath.Join(sub, "lamp.txt"))
		_, listed := entryStatus(a, "lantern")
		return !indexed && !listed
	})
	if _, found, _ := db.DirByPath(d, sub); found {
		t.Fatal("the directory record survived")
	}
	for _, f := range []string{"lamp.txt", "node_modules/pebble.txt"} {
		if _, err := os.Stat(filepath.Join(sub, f)); err != nil {
			t.Fatalf("a browse delete touched the disk: %v", err)
		}
	}
}

func TestFileIndexPanelRenameChangesIndexOnly(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	openIndexPanel(t, a, screen, dir)
	cursorTo(t, a, "harbor.txt")
	dispatchFileOp(t, a, keymap.ActionFileRename)
	if !strings.Contains(a.model.FileDialog.Note, "Renames in the fileindex only") {
		t.Fatalf("rename note = %q", a.model.FileDialog.Note)
	}
	a.model.FileDialog.Fields[0].Value = "anchor.txt"
	a.dialogCtrl.ExecuteFileDialog()
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, oldIndexed, _ := d.FileInfo(filepath.Join(dir, "harbor.txt"))
		_, newIndexed, _ := d.FileInfo(filepath.Join(dir, "anchor.txt"))
		_, listed := entryStatus(a, "anchor.txt")
		return !oldIndexed && newIndexed && listed
	})
	if _, err := os.Stat(filepath.Join(dir, "harbor.txt")); err != nil {
		t.Fatalf("a browse rename touched the disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "anchor.txt")); err == nil {
		t.Fatal("a browse rename created a file on disk")
	}
}

func TestFileIndexPanelMoveInsideIndexChangesRecordsOnly(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	other := filepath.Join(dir, "meadow")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	sum := strings.Repeat("c", 32)
	if _, err := idx.CommitScanBatch([]db.ScannedFile{{Dir: other, Name: "clover.txt", Size: 1, Mtime: time.Now(), MD5: sum, SHA256: sum + sum}}); err != nil {
		t.Fatal(err)
	}
	openIndexPanel(t, a, screen, dir)
	cursorTo(t, a, "harbor.txt")
	a.dialogCtrl.ActivateMoveAction()
	if !a.model.TransferDialog.Open {
		t.Fatalf("move dialog did not open: %q", a.model.Message)
	}
	a.model.TransferDialog.Destination.Value = "fileindex://" + other
	a.dialogCtrl.TransferDialogOK()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, oldIndexed, _ := idx.FileInfo(filepath.Join(dir, "harbor.txt"))
		_, newIndexed, _ := idx.FileInfo(filepath.Join(other, "harbor.txt"))
		_, listed := entryStatus(a, "harbor.txt")
		return !oldIndexed && newIndexed && !listed
	})
	if _, err := os.Stat(filepath.Join(dir, "harbor.txt")); err != nil {
		t.Fatalf("a browse move touched the disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, "harbor.txt")); err == nil {
		t.Fatal("a browse move put a file on disk")
	}
}

func TestFileIndexPanelMoveOutIsRefused(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	other := filepath.Join(dir, "meadow")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	openIndexPanel(t, a, screen, dir)
	cursorTo(t, a, "harbor.txt")
	a.dialogCtrl.ActivateMoveAction()
	a.model.TransferDialog.Destination.Value = other
	a.dialogCtrl.TransferDialogOK()
	if a.model.Message != ops.IndexMoveOutRefusal {
		t.Fatalf("message = %q, want the move-out refusal", a.model.Message)
	}
	if n := len(a.jobState.AllJobs()); n != 0 {
		t.Fatalf("%d jobs queued, want none", n)
	}
}

// A dedup delete of fileindex:// paths acts on the disk and the index, unlike a delete on a
// fileindex:// panel; a file already gone from disk only loses its record.
func TestFileIndexDedupDeleteRemovesFileAndRecord(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	dedupHost{appShellHost: appShellHost{app: a}}.EnqueueDeleteJob([]string{
		pathloc.FileIndexPrefix + filepath.Join(dir, "harbor.txt"),
		pathloc.FileIndexPrefix + filepath.Join(dir, "meadow.txt"),
	}, false)
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, harbor, _ := d.FileInfo(filepath.Join(dir, "harbor.txt"))
		_, meadow, _ := d.FileInfo(filepath.Join(dir, "meadow.txt"))
		_, onDisk := os.Stat(filepath.Join(dir, "harbor.txt"))
		return !harbor && !meadow && onDisk != nil
	})
}

func TestFileIndexNormalLocalPanelDeleteFollows(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	cursorTo(t, a, "harbor.txt")
	dispatchFileOp(t, a, keymap.ActionFileDelete)
	if strings.Contains(a.model.FileDialog.Note, "fileindex") {
		t.Fatalf("a normal local panel's delete dialog mentions the index: %q", a.model.FileDialog.Note)
	}
	a.dialogCtrl.ExecuteDelete()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, err := os.Stat(filepath.Join(dir, "harbor.txt"))
		all := a.jobState.AllJobs()
		return err != nil && len(all) == 1 && all[0].Status == jobs.StatusCompleted
	})
	d, err := a.fileIndex.DB()
	if err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		_, found, _ := d.FileInfo(filepath.Join(dir, "harbor.txt"))
		return !found
	})
}

// rescan records dir's files with their real size and mtime (the fixture records "now").
func rescan(t *testing.T, a *App, dir string) {
	t.Helper()
	if err := a.fileIndex.ScanTask([]string{dir}).Run(context.Background(), nopTaskReporter{}); err != nil {
		t.Fatal(err)
	}
}

func TestFileIndexReportDifferences(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	rescan(t, a, dir)
	a.dispatch(keymap.ActionFileIndexReportDifferences)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.Message == "No differences"
	})
	// The job's own terminal event must not replace the result with "Job completed".
	// Applying the events (PollEvents) is what finishes the job in the state.
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(time.Millisecond) {
		a.jobsCtrl.PollEvents()
		if j := a.jobState.AllJobs(); len(j) > 0 && j[len(j)-1].Status.IsFinished() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for the differences job to finish")
		}
	}
	if a.model.Message != "No differences" || a.model.MessageUrgency != ui.MessageUrgencySuccess {
		t.Fatalf("message = %q urgency %v, want No differences / success", a.model.Message, a.model.MessageUrgency)
	}

	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionFileIndexReportDifferences)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.PathPicker.Open })
	st := a.model.PathPicker
	if want := "Differences under " + filepath.Base(dir) + " (2)"; st.Title != want || st.Purpose != dialog.PathPickerPurposeNavigateFile || st.IndexPanel {
		t.Fatalf("picker = %q purpose %v index panel %v, want %q", st.Title, st.Purpose, st.IndexPanel, want)
	}
	got := map[string]dialog.PathPickerItem{}
	for _, it := range st.Items {
		got[filepath.Base(it.Path)] = it
	}
	if it := got["willow.txt"]; it.Source != "new" || it.PathMissing {
		t.Errorf("willow.txt row = %+v, want a new, present path", it)
	}
	if it := got["meadow.txt"]; it.Source != "missing" || !it.PathMissing {
		t.Errorf("meadow.txt row = %+v, want a missing path", it)
	}
	if !slices.ContainsFunc(a.jobState.AllJobs(), func(j *jobs.Job) bool {
		return j.Task != nil && j.Task.Results != nil && j.Task.Results.Label == "2 differences"
	}) {
		t.Fatal("no job kept the 2 differences")
	}
	// Choosing a row shows the file in the panel.
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return !a.model.PathPicker.Open })
}

// lastTask is the task of the newest job.
func lastTask(t *testing.T, a *App) *jobs.Task {
	t.Helper()
	all := a.jobState.AllJobs()
	if len(all) == 0 || all[len(all)-1].Task == nil {
		t.Fatal("no task job queued")
	}
	return all[len(all)-1].Task
}

// differencesApp is a fileIndexApp whose index differs from disk by one missing and one new file.
func differencesApp(t *testing.T) (*App, tcell.SimulationScreen, string) {
	t.Helper()
	a, screen, dir, _ := fileIndexApp(t)
	rescan(t, a, dir)
	if err := os.Remove(filepath.Join(dir, "meadow.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "willow.txt"), []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}
	return a, screen, dir
}

// requestDifferences dispatches the report and waits until its job has a result in sight: the
// payload is not handled yet, so the test decides the state it lands in.
func requestDifferences(t *testing.T, a *App) {
	t.Helper()
	a.dispatch(keymap.ActionFileIndexReportDifferences)
	if a.model.Message == "fileindex differences queued" {
		t.Fatal("the queued toast showed at once")
	}
}

func TestFileIndexReportDifferencesBusyUserGetsToastNotPicker(t *testing.T) {
	a, screen, dir := differencesApp(t)
	requestDifferences(t, a)
	a.model.MessageDialog.Open = true // a dialog is open when the result lands
	want := "2 differences under " + filepath.Base(dir) + ": see Jobs view"
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.Message == want })
	if a.model.PathPicker.Open || a.model.MessageUrgency != ui.MessageUrgencySuccess {
		t.Fatalf("picker %v urgency %v, want no picker and success", a.model.PathPicker.Open, a.model.MessageUrgency)
	}
	if r := lastTask(t, a).Results; r == nil || r.Label != "2 differences" {
		t.Fatalf("job results = %+v, want kept", r)
	}
}

func TestFileIndexReportDifferencesOtherViewGetsToast(t *testing.T) {
	a, screen, _ := differencesApp(t)
	requestDifferences(t, a)
	a.model.ViewMode = ui.ViewMessages
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return strings.HasSuffix(a.model.Message, ": see Jobs view")
	})
	if a.model.PathPicker.Open || lastTask(t, a).Results == nil {
		t.Fatalf("picker %v, results %+v, want a toast and kept results", a.model.PathPicker.Open, lastTask(t, a).Results)
	}
}

func TestFileIndexReportDifferencesSlowResultKeepsResultOnFinishedJob(t *testing.T) {
	a, screen, dir := differencesApp(t)
	requestDifferences(t, a)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.PathPicker.Open })
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(time.Millisecond) {
		a.jobsCtrl.PollEvents()
		if j := a.jobState.AllJobs(); j[len(j)-1].Status.IsFinished() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for the job to finish")
		}
	}
	// A result that lands late, after the terminal event: it only notifies.
	a.dialogCtrl.ClosePathPicker()
	task := lastTask(t, a)
	task.Results = nil
	a.jobsCtrl.SyncJobsList()
	late := fileindexctrl.DifferencesPayload{
		PanelID: a.model.ActivePanel, Root: dir, Task: task, Resolved: new(bool),
		Started: time.Now().Add(-config.DifferencesFastWindow - time.Second),
		Diffs:   []fi.Difference{{Path: filepath.Join(dir, "willow.txt"), State: fi.DiffNew}},
	}
	a.fileindexCtrl.HandleDifferences(late)
	if want := "1 difference under " + filepath.Base(dir) + ": see Jobs view"; a.model.Message != want || a.model.MessageUrgency != ui.MessageUrgencySuccess {
		t.Fatalf("message = %q urgency %v, want %q / success", a.model.Message, a.model.MessageUrgency, want)
	}
	if a.model.PathPicker.Open {
		t.Fatal("a slow result opened the picker")
	}
	if task.Results == nil || task.Results.Label != "1 difference" {
		t.Fatalf("results = %+v, want kept", task.Results)
	}
	list := a.model.JobsList
	if len(list) == 0 || list[len(list)-1].Results != "1 difference" {
		t.Fatalf("jobs list = %+v, want the label on the job entry", list)
	}
}

func TestFileIndexReportDifferencesAlwaysShowsEmptyAndErrors(t *testing.T) {
	a, _, dir, _ := fileIndexApp(t)
	a.model.MessageDialog.Open = true
	a.model.ViewMode = ui.ViewJobs
	old := time.Now().Add(-time.Hour)
	for _, c := range []struct {
		p    fileindexctrl.DifferencesPayload
		want string
	}{
		{fileindexctrl.DifferencesPayload{}, "No differences"},
		{fileindexctrl.DifferencesPayload{Err: fi.ErrNotIndexed}, filepath.Base(dir) + " is not in the index"},
	} {
		c.p.PanelID, c.p.Root, c.p.Started, c.p.Resolved = a.model.ActivePanel, dir, old, new(bool)
		a.model.Message = ""
		a.fileindexCtrl.HandleDifferences(c.p)
		if a.model.Message != c.want {
			t.Errorf("message = %q, want %q", a.model.Message, c.want)
		}
	}
	a.model.Message = ""
	a.fileindexCtrl.HandleDifferences(fileindexctrl.DifferencesPayload{Root: dir, Err: errors.New("boom"), Resolved: new(bool)})
	if !a.model.MessageDialog.Open || a.model.Message == "" && a.model.MessageDialog.Message == "" {
		t.Error("an error report must show")
	}
}

// waitingReport holds the differences lease with a blocker task, requests a report and waits until
// its job is queued behind the blocker. It returns the report's task and job id.
func waitingReport(t *testing.T, a *App, screen tcell.SimulationScreen) (*jobs.Task, string) {
	t.Helper()
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	a.jobsCtrl.AddTaskJob(&jobs.Task{Title: "blocker", Lease: fi.DifferencesLease, Run: func(ctx context.Context, _ jobs.TaskReporter) error {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}}, nil)
	<-started
	a.dispatch(keymap.ActionFileIndexReportDifferences)
	var task *jobs.Task
	var id string
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		for _, j := range a.jobState.AllJobs() {
			if j.Task != nil && j.Task.Title == "fileindex differences" {
				task, id = j.Task, j.ID
			}
		}
		return task != nil
	})
	return task, id
}

func TestFileIndexDifferencesQueuedToast(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	task, _ := waitingReport(t, a, screen)
	resolved := new(bool)
	a.model.Message = ""
	a.fileindexCtrl.HandleDifferencesQueued(fileindexctrl.DifferencesQueuedPayload{Resolved: resolved, Task: task})
	if a.model.Message != "fileindex differences queued" || a.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("message = %q urgency %v, want the queued toast / info", a.model.Message, a.model.MessageUrgency)
	}
	// A fast result resolves the report: the timer firing after it leaves its message alone.
	a.fileindexCtrl.HandleDifferences(fileindexctrl.DifferencesPayload{PanelID: a.model.ActivePanel, Root: dir, Started: time.Now(), Resolved: resolved})
	a.fileindexCtrl.HandleDifferencesQueued(fileindexctrl.DifferencesQueuedPayload{Resolved: resolved, Task: task})
	if a.model.Message != "No differences" {
		t.Fatalf("message = %q, want No differences", a.model.Message)
	}
}

// A report canceled while it waits for the lease never runs, so nothing resolves it: the timer
// must not toast "queued" for a job that is gone.
func TestFileIndexDifferencesQueuedToastSkipsCanceledJob(t *testing.T) {
	a, screen, _, _ := fileIndexApp(t)
	task, id := waitingReport(t, a, screen)
	if !a.jobState.CancelJob(id) {
		t.Fatal("cancel found no job")
	}
	a.model.Message = ""
	a.fileindexCtrl.HandleDifferencesQueued(fileindexctrl.DifferencesQueuedPayload{Resolved: new(bool), Task: task})
	if a.model.Message != "" {
		t.Fatalf("message = %q, want none for a canceled report", a.model.Message)
	}
}

func TestFileIndexShowResultsFromJobsView(t *testing.T) {
	a, screen, dir := differencesApp(t)
	requestDifferences(t, a)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.PathPicker.Open })
	a.dialogCtrl.ClosePathPicker()

	openAtJob := func() {
		a.jobsCtrl.OpenJobsView()
		a.jobsCtrl.SyncJobsList()
		a.model.JobsView.Selected = len(a.model.JobsList) - 1
	}
	openAtJob()
	if !slices.ContainsFunc(menu.FunctionKeysJobsView(false, false, a.jobsCtrl.SelectedJobHasResults()), func(fk menu.FunctionKey) bool {
		return fk.ActionID == keymap.ActionJobsShowResults
	}) {
		t.Fatal("the footer lacks the results key for a job with results")
	}

	// Cancel returns to the jobs view.
	a.dispatch(keymap.ActionJobsShowResults)
	if !a.model.PathPicker.Open || a.model.ViewMode != ui.ViewJobs {
		t.Fatalf("picker %v view %v, want the picker over the jobs view", a.model.PathPicker.Open, a.model.ViewMode)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.model.PathPicker.Open || a.model.ViewMode != ui.ViewJobs {
		t.Fatalf("after cancel: picker %v view %v, want the jobs view", a.model.PathPicker.Open, a.model.ViewMode)
	}

	// Confirming a path shows it in the browser.
	a.dispatch(keymap.ActionJobsShowResults)
	a.model.PathPicker.Selected = slices.IndexFunc(a.model.PathPicker.Items, func(it dialog.PathPickerItem) bool {
		return it.Path == filepath.Join(dir, "willow.txt")
	})
	a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.ViewMode == ui.ViewBrowser })
	if a.model.PathPicker.Open {
		t.Fatal("the picker stayed open after confirming")
	}
	if e, ok := a.activePanel().CurrentEntry(); !ok || e.Name != "willow.txt" {
		t.Fatalf("cursor entry = %+v, want willow.txt", e)
	}
}

func TestFileIndexShowResultsWithoutResultsWarns(t *testing.T) {
	a, _, _, _ := fileIndexApp(t)
	a.fileindexCtrl.Scan()
	a.jobsCtrl.OpenJobsView()
	a.jobsCtrl.SyncJobsList()
	a.model.JobsView.Selected = 0
	if a.jobsCtrl.SelectedJobHasResults() {
		t.Fatal("a scan job has no results")
	}
	a.dispatch(keymap.ActionJobsShowResults)
	if a.model.Message != "This job has no results" || a.model.MessageUrgency != ui.MessageUrgencyWarn || a.model.PathPicker.Open {
		t.Fatalf("message = %q urgency %v picker %v", a.model.Message, a.model.MessageUrgency, a.model.PathPicker.Open)
	}
}

func TestFileIndexReportDifferencesTargetsCursorDirectory(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	sub := filepath.Join(dir, "lantern")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	a.dispatch(keymap.ActionPanelRefresh)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { _, ok := entryStatus(a, "lantern"); return ok })
	cursorTo(t, a, "lantern")
	a.dispatch(keymap.ActionFileIndexReportDifferences)
	// The directory is not in the index: the report says so instead of listing the panel's files.
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.model.Message == "lantern is not in the index"
	})
}

func TestFileIndexReportDifferencesCanceledShowsNothing(t *testing.T) {
	a, _, dir, _ := fileIndexApp(t)
	a.fileindexCtrl.HandleDifferences(fileindexctrl.DifferencesPayload{
		PanelID: a.model.ActivePanel, Root: dir, Err: context.Canceled, Resolved: new(bool),
	})
	if a.model.PathPicker.Open || a.model.Message != "" {
		t.Fatalf("a canceled report showed something: picker %v, message %q", a.model.PathPicker.Open, a.model.Message)
	}
}

// openCopiesPicker opens the other-copies picker for harbor.txt (one copy, in meadow/).
func openCopiesPicker(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	cursorTo(t, a, "harbor.txt")
	a.dispatch(keymap.ActionFileIndexCopies)
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return a.model.PathPicker.Open })
}

func TestFileIndexCopiesPickerMarksAddToSelection(t *testing.T) {
	for _, onIndex := range []bool{false, true} {
		a, screen, dir := copiesApp(t)
		copyPath := filepath.Join(dir, "meadow", "harbor.txt")
		want := copyPath
		if onIndex {
			openIndexPanel(t, a, screen, dir)
			want = "fileindex://" + copyPath
		}
		openCopiesPicker(t, a, screen)
		a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
		if !a.model.PathPicker.Marked[copyPath] {
			t.Fatalf("onIndex=%v: Insert did not mark: %v", onIndex, a.model.PathPicker.Marked)
		}
		a.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		if a.model.PathPicker.Open || !a.activePanel().SelectedPaths[want] || a.model.Message != "Added 1 to selection" {
			t.Fatalf("onIndex=%v: open=%v selected=%v message=%q", onIndex, a.model.PathPicker.Open, a.activePanel().SelectedPaths, a.model.Message)
		}
		if a.activePanel().PathString() == filepath.Dir(want) {
			t.Fatalf("onIndex=%v: applying marks must not jump to the copy", onIndex)
		}
	}
}

// ui.open-primary / ui.open-secondary target the visual slot, so a swapped layout must send
// Shift+Right to Primary (drawn second) and Shift+Left to Secondary (drawn first).
func TestFileIndexCopiesPickerOpenInSlotFollowsSwap(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, right := range []bool{false, true} {
			a, screen, dir := copiesApp(t)
			a.model.SwapPanes = swap
			target := filepath.Join(dir, "meadow")
			// Expected identity is spelled out per case, not derived from the helper.
			wantPanel, otherPanel := &a.model.Primary, &a.model.Secondary
			if right != swap {
				wantPanel, otherPanel = &a.model.Secondary, &a.model.Primary
			}
			otherBefore := otherPanel.PathString()
			openCopiesPicker(t, a, screen)
			key := tcell.KeyLeft
			if right {
				key = tcell.KeyRight
			}
			a.handleKey(tcell.NewEventKey(key, 0, tcell.ModShift))
			drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool { return wantPanel.PathString() == target })
			if wantPanel.PathString() != target || otherPanel.PathString() != otherBefore || !a.model.PathPicker.Open {
				t.Fatalf("swap=%v right=%v: want=%q other=%q (was %q) open=%v, want target %q",
					swap, right, wantPanel.PathString(), otherPanel.PathString(), otherBefore, a.model.PathPicker.Open, target)
			}
		}
	}
}

func TestFileIndexCopiesPickerSpaceIsFilterText(t *testing.T) {
	a, screen, _ := copiesApp(t)
	openCopiesPicker(t, a, screen)
	a.handleKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if st := a.model.PathPicker; len(st.Marked) != 0 || st.Query != " " {
		t.Fatalf("space: marked=%v query=%q", st.Marked, st.Query)
	}
}

func TestFileIndexCopiesPickerPins(t *testing.T) {
	for _, onIndex := range []bool{false, true} {
		a, screen, dir := copiesApp(t)
		want := filepath.Join(dir, "meadow", "harbor.txt")
		if onIndex {
			openIndexPanel(t, a, screen, dir)
			want = "fileindex://" + want
		}
		openCopiesPicker(t, a, screen)
		a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModAlt))
		if !a.model.PathPicker.Open || len(a.model.PinnedItems) != 1 || a.model.PinnedItems[0].Path != want || a.model.PinnedItems[0].IsDir {
			t.Fatalf("onIndex=%v: open=%v pins=%+v, want %s", onIndex, a.model.PathPicker.Open, a.model.PinnedItems, want)
		}
		// The row shows the pin mark whichever kind of path it was pinned by.
		a.render()
		screen.Show()
		w, h := screen.Size()
		cells, _, _ := screen.GetContents()
		found := false
		for y := range h {
			var row strings.Builder
			for x := range w {
				row.WriteString(string(cells[y*w+x].Runes))
			}
			found = found || (strings.Contains(row.String(), "harbor.txt") && strings.ContainsRune(row.String(), a.styles.IconPinRune()) && strings.Contains(row.String(), "meadow"))
		}
		if !found {
			t.Fatalf("onIndex=%v: pinned row shows no pin mark", onIndex)
		}
	}
}

func TestFileIndexCopiesPickerOpensInEitherPanel(t *testing.T) {
	a, screen, dir := copiesApp(t)
	wantDir := filepath.Join(dir, "meadow")
	openCopiesPicker(t, a, screen)
	active := a.model.ActivePanel
	for _, tc := range []struct {
		key     tcell.ModMask
		arrow   tcell.Key
		panelID int
	}{
		{tcell.ModShift, tcell.KeyLeft, ui.PrimaryPanel},
		{tcell.ModShift, tcell.KeyRight, ui.SecondaryPanel},
	} {
		a.handleKey(tcell.NewEventKey(tc.arrow, 0, tc.key))
		drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
			p := a.panelByID(tc.panelID)
			e, ok := p.CurrentEntry()
			return p.PathString() == wantDir && !p.ListingPending && ok && e.Name == "harbor.txt"
		})
		if !a.model.PathPicker.Open || a.model.ActivePanel != active {
			t.Fatalf("panel %d: picker open=%v, active panel %d, want %d", tc.panelID, a.model.PathPicker.Open, a.model.ActivePanel, active)
		}
	}
}

func TestFileIndexCopiesPickerViewReturnsToPicker(t *testing.T) {
	a, screen, _ := copiesApp(t)
	openCopiesPicker(t, a, screen)
	a.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	a.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if a.model.PathPicker.Open || !a.model.FullscreenFilePreview.Open {
		t.Fatalf("picker open=%v, viewer open=%v", a.model.PathPicker.Open, a.model.FullscreenFilePreview.Open)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if st := a.model.PathPicker; !st.Open || a.model.FullscreenFilePreview.Open || len(st.Marked) != 1 {
		t.Fatalf("after viewer: picker %+v, viewer open=%v", st, a.model.FullscreenFilePreview.Open)
	}
}

func TestFileIndexCopiesPickerEditOpensEditor(t *testing.T) {
	a, screen, dir := copiesApp(t)
	var edited string
	a.runExternalEditor = func(_ context.Context, path string, _ bool) error {
		edited = path
		return nil
	}
	openCopiesPicker(t, a, screen)
	a.handleKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if want := filepath.Join(dir, "meadow", "harbor.txt"); edited != want || !a.model.PathPicker.Open {
		t.Fatalf("edited %q, want %q; picker open=%v", edited, want, a.model.PathPicker.Open)
	}
}

func TestFilePickerRefusesViewAndEditOfMissingFile(t *testing.T) {
	a, _, dir := copiesApp(t)
	a.dialogCtrl.OpenFilePicker("Differences", []dialog.PathPickerItem{
		{Source: "missing", Path: filepath.Join(dir, "gone.txt"), PathMissing: true},
	}, false)
	a.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if !a.model.PathPicker.Open || a.model.FullscreenFilePreview.Open || a.model.Message != "View: file is missing" {
		t.Fatalf("view: picker=%v viewer=%v message=%q", a.model.PathPicker.Open, a.model.FullscreenFilePreview.Open, a.model.Message)
	}
	a.handleKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if a.model.Message != "Edit: file is missing" {
		t.Fatalf("edit: message=%q", a.model.Message)
	}
}

func TestFilePickerFooterAdvertisesActions(t *testing.T) {
	a, screen, _ := copiesApp(t)
	openCopiesPicker(t, a, screen)
	hints := map[string]string{}
	for _, k := range a.pathPickerColumnsFooterKeys() {
		hints[k.Hint] = k.KeyLabel
	}
	for _, hint := range []string{"View", "Edit", "Open ◄", "Open ►"} {
		if hints[hint] == "" {
			t.Errorf("footer misses %q: %v", hint, hints)
		}
	}
}

// An index change during a same-directory listing must start a new one: the listing in flight may
// have read the index before the change. A navigation in flight is left alone.
func TestReloadIndexPanelsSupersedesListingInFlight(t *testing.T) {
	dir := t.TempDir()
	app := newApp(t, newScreen(t, 80, 24), dir)
	p := &app.model.Primary
	p.WithIndex = true

	p.ListingPending, p.ListingPendingPath = true, p.Path.String()
	before := app.panelAsyncLoadGen[ui.PrimaryPanel].Load()
	app.refreshIndexViews()
	if app.panelAsyncLoadGen[ui.PrimaryPanel].Load() == before {
		t.Fatal("the refresh was dropped because a listing of the same directory was in flight")
	}

	p.ListingPending, p.ListingPendingPath = true, filepath.Join(dir, "elsewhere")
	before = app.panelAsyncLoadGen[ui.PrimaryPanel].Load()
	app.refreshIndexViews()
	if app.panelAsyncLoadGen[ui.PrimaryPanel].Load() != before {
		t.Fatal("the refresh replaced a navigation in flight")
	}
}
