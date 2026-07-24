package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func waitDedupDone(t *testing.T, app *App) comparepkg.DedupSnapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDedupUpdates(dedupctrl.WakePayload{})
		snap := app.model.DedupSnapshot
		if snap.Phase == comparepkg.DedupDone || snap.Phase == comparepkg.DedupError {
			return snap
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("dedup scan did not finish")
	return comparepkg.DedupSnapshot{}
}

func TestDedupViewFindsDuplicates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unique.txt"), []byte("only-me"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	if !app.model.DedupProgressDialog.Open {
		t.Fatal("DedupProgressDialog should be open after starting scan")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser during scan", app.model.ViewMode)
	}

	snap := waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after scan", app.model.ViewMode)
	}
	if snap.Phase != comparepkg.DedupDone {
		t.Fatalf("phase = %v (%s)", snap.Phase, snap.Err)
	}
	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snap.Groups))
	}
	if len(app.model.DedupList) != 2 {
		t.Fatalf("DedupList = %d, want 2 (sub dir collapsed + a.txt)", len(app.model.DedupList))
	}

	app.render() // exercise drawDedupView; must not panic

	// Esc closes back to the browser.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("after esc ViewMode = %v, want ViewBrowser", app.model.ViewMode)
	}
}

func TestDedupViewTrimsSingleChainDisplayRoot(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "alpha", "bravo", "charlie")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "copper.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "willow.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	snap := waitDedupDone(t, app)

	wantDisplay := filepath.Join(dir, "alpha", "bravo", "charlie")
	if got := snap.EffectiveDisplayRoot().String(); got != wantDisplay {
		t.Fatalf("EffectiveDisplayRoot = %q, want %q", got, wantDisplay)
	}
	if snap.Root.String() != dir {
		t.Fatalf("scan Root = %q, want %q", snap.Root, dir)
	}
	if app.model.Message != "Duplicates view re-rooted" {
		t.Fatalf("message = %q, want display-root trim toast", app.model.Message)
	}
	if len(app.model.DedupList) != 2 {
		t.Fatalf("DedupList = %d, want 2 files at trimmed root", len(app.model.DedupList))
	}
	for _, row := range app.model.DedupList {
		if row.Value.Kind != ui.DedupRowFile {
			t.Fatalf("row kind = %v, want file (no empty alpha/bravo chain)", row.Value.Kind)
		}
		if row.Depth != 0 {
			t.Fatalf("row depth = %d, want 0 under trimmed display root", row.Depth)
		}
	}
}

func TestDedupViewTrimsToBranchParentDisplayRoot(t *testing.T) {
	dir := t.TempDir()
	for _, rel := range []string{
		filepath.Join("test-cases", "diff-a", "copper.txt"),
		filepath.Join("test-cases", "diff-b", "willow.txt"),
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	snap := waitDedupDone(t, app)

	wantDisplay := filepath.Join(dir, "test-cases")
	if got := snap.EffectiveDisplayRoot().String(); got != wantDisplay {
		t.Fatalf("EffectiveDisplayRoot = %q, want %q", got, wantDisplay)
	}
	if app.model.Message != "Duplicates view re-rooted" {
		t.Fatalf("message = %q, want display-root trim toast", app.model.Message)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("MessageUrgency = %v, want info", app.model.MessageUrgency)
	}
	if len(app.model.DedupList) != 2 {
		t.Fatalf("DedupList = %d, want diff-a + diff-b at trimmed root", len(app.model.DedupList))
	}
	names := map[string]bool{}
	for _, row := range app.model.DedupList {
		if row.Value.Kind != ui.DedupRowDir || row.Depth != 0 {
			t.Fatalf("row = %+v, want depth-0 dir", row)
		}
		names[row.Value.Display] = true
	}
	if !names["diff-a"] || !names["diff-b"] {
		t.Fatalf("top-level dirs = %v, want diff-a and diff-b", names)
	}
}

func TestDedupViewSkipsSymlinks(t *testing.T) {
	rootDir := t.TempDir()
	scan := filepath.Join(rootDir, "scan")
	hidden := filepath.Join(rootDir, "hidden", "other")
	if err := os.MkdirAll(scan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(hidden, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scan, "real1.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scan, "real2.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hidden, "hidden.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real1.txt", filepath.Join(scan, "link1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "hidden", "other"), filepath.Join(scan, "other-link")); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, scan)
	app.openFindDuplicates()
	snap := waitDedupDone(t, app)

	if len(snap.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(snap.Groups))
	}
	for _, f := range snap.Groups[0].Files {
		if f.Rel != "real1.txt" && f.Rel != "real2.txt" {
			t.Fatalf("unexpected duplicate member %q", f.Rel)
		}
	}
}

func TestDedupViewPlainLeftDoesNotCloseView(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup", app.model.ViewMode)
	}

	// Plain Left has no dedup binding (tree collapse moved to Alt+Left): it must not close
	// the view or quit the app, just do nothing.
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if quit {
		t.Fatal("KeyLeft in dedup view must not quit the application")
	}
	if app.model.ViewMode != ui.ViewDedup {
		t.Fatalf("ViewMode = %v, want ViewDedup after KeyLeft", app.model.ViewMode)
	}
}

func TestDedupViewAltArrowJumpsBetweenVisibleDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("meadow", "beacon.txt"),
		filepath.Join("orchard", "lantern.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	mainSelDir := func(wantRel string) {
		t.Helper()
		row := app.model.DedupList[app.model.DedupView.Main.Selected]
		if row.Value.Kind != ui.DedupRowDir || row.Value.DirRel != wantRel {
			t.Fatalf("main selected = %+v, want dir %q", row.Value, wantRel)
		}
	}
	copiesSelDir := func(wantRel string) {
		t.Helper()
		row := app.model.DedupCopiesList[app.model.DedupView.Copies.Selected]
		if row.Value.Kind != ui.DedupRowDir || row.Value.DirRel != wantRel {
			t.Fatalf("copies selected = %+v, want dir %q", row.Value, wantRel)
		}
	}

	// Main pane: meadow (dir), orchard (dir), lantern.txt (file).
	mainSelDir("meadow")
	before := app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	if app.model.DedupView.Main.Selected != before {
		t.Fatalf("Alt+Up on first dir moved cursor from %d to %d", before, app.model.DedupView.Main.Selected)
	}

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	mainSelDir("orchard")

	before = app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	if app.model.DedupView.Main.Selected != before {
		t.Fatal("Alt+Down on last dir should stay (next row is a file)")
	}

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)) // file row
	if app.model.DedupList[app.model.DedupView.Main.Selected].Value.Kind != ui.DedupRowFile {
		t.Fatal("expected file row after Down from orchard")
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	mainSelDir("orchard")

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	before = app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	if app.model.DedupView.Main.Selected != before {
		t.Fatal("Alt+Down on last row should stay when no next dir exists")
	}

	// Copies pane: cursor is on root file; Tab focuses copies dirs.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus copies pane")
	}
	copiesSelDir("meadow")

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	copiesSelDir("orchard")

	before = app.model.DedupView.Copies.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModAlt))
	if app.model.DedupView.Copies.Selected != before {
		t.Fatal("Alt+Down on last copies dir should stay")
	}

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	copiesSelDir("meadow")

	before = app.model.DedupView.Copies.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModAlt))
	if app.model.DedupView.Copies.Selected != before {
		t.Fatal("Alt+Up on first copies dir should stay")
	}
}

func TestDedupViewCopiesPaneTabFocusAndMark(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt"), filepath.Join("meadow", "beacon.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Dirs mode starts collapsed; Down from the first directory row selects the root file.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// Cursor on a duplicate file: copies pane lists the two OTHER copies.
	if got := len(app.model.DedupCopiesList); got == 0 {
		t.Fatal("copies pane empty for selected file")
	}
	var copyFiles []string
	selAbs := app.model.DedupList[app.model.DedupView.Main.Selected].Value.AbsKey
	for _, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowFile {
			copyFiles = append(copyFiles, r.Value.AbsKey)
			if r.Value.AbsKey == selAbs {
				t.Fatal("copies pane contains the selected file itself")
			}
		}
	}
	if len(copyFiles) != 2 {
		t.Fatalf("copies pane files = %v, want 2 other copies", copyFiles)
	}

	// Tab focuses the copies pane; Down moves its cursor, not the main one.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus the copies pane")
	}
	mainSel := app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.DedupView.Main.Selected != mainSel {
		t.Fatal("Down in copies pane moved the main cursor")
	}

	// Space marks the focused copy.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1 after marking in copies pane", app.model.DedupView.MarkedCount)
	}

	// Tab returns to the main pane; moving the main cursor rebuilds the copies pane.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if app.model.DedupView.FocusCopies {
		t.Fatal("second Tab did not return focus to the main pane")
	}

	app.render() // twin panes must render without panicking
}

func TestDedupViewCopiesPaneFolderSelectToggle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("meadow", "beacon.txt"),
		filepath.Join("orchard", "lantern.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Select root lantern.txt (dirs mode: meadow, orchard, then root file).
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus copies pane")
	}

	copies := app.model.DedupCopiesList
	if len(copies) < 2 {
		t.Fatalf("copies list = %d rows, want at least meadow + orchard dirs", len(copies))
	}
	if copies[0].Value.Kind != ui.DedupRowDir || copies[0].Value.DirRel != "meadow" {
		t.Fatalf("first copy row = %+v, want meadow dir", copies[0].Value)
	}
	if app.model.DedupView.Copies.Selected != 0 {
		t.Fatalf("copies cursor = %d, want 0 on meadow", app.model.DedupView.Copies.Selected)
	}

	meadowLantern := filepath.Join(dir, "meadow", "lantern.txt")
	meadowBeacon := filepath.Join(dir, "meadow", "beacon.txt")
	orchardLantern := filepath.Join(dir, "orchard", "lantern.txt")

	// Insert on meadow: mark all files under meadow, advance to orchard.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 2 {
		t.Fatalf("MarkedCount = %d, want 2 after meadow folder mark", app.model.DedupView.MarkedCount)
	}
	if !app.model.DedupView.Marked[meadowLantern] || !app.model.DedupView.Marked[meadowBeacon] {
		t.Fatalf("meadow files not marked: %v", app.model.DedupView.Marked)
	}
	if app.model.DedupView.Marked[orchardLantern] {
		t.Fatal("orchard file should not be marked yet")
	}
	sel := app.model.DedupView.Copies.Selected
	if sel < 0 || sel >= len(copies) || copies[sel].Value.DirRel != "orchard" {
		t.Fatalf("cursor on %q after meadow mark, want orchard dir", copies[sel].Value.DirRel)
	}

	// Move back to meadow and Insert again: clear meadow marks.
	for i, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Copies.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("MarkedCount = %d, want 0 after meadow folder clear", app.model.DedupView.MarkedCount)
	}

	// Partial mark under meadow, then folder Insert clears all under meadow.
	app.dedupCtrl.ExpandAll()
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)) // meadow dir
	for i, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowFile && r.Value.File.Rel == "meadow/beacon.txt" {
			app.model.DedupView.Copies.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)) // mark beacon only
	if app.model.DedupView.MarkedCount != 1 || !app.model.DedupView.Marked[meadowBeacon] {
		t.Fatalf("partial mark failed: count=%d marked=%v", app.model.DedupView.MarkedCount, app.model.DedupView.Marked)
	}
	for i, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Copies.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("folder clear after partial mark: count=%d, want 0", app.model.DedupView.MarkedCount)
	}
	if app.model.DedupView.Marked[meadowBeacon] || app.model.DedupView.Marked[meadowLantern] {
		t.Fatalf("meadow marks remain: %v", app.model.DedupView.Marked)
	}
}

func TestDedupViewMainPaneFolderSelectToggle(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("orchard", "lantern.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{
		filepath.Join("meadow", "beacon.txt"),
		filepath.Join("orchard", "beacon.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup2"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	if app.model.DedupView.FocusCopies {
		t.Fatal("file-tree panel must be focused on open")
	}
	main := app.model.DedupList
	if len(main) < 2 {
		t.Fatalf("main list = %d rows, want at least meadow + orchard dirs", len(main))
	}
	if main[0].Value.Kind != ui.DedupRowDir || main[0].Value.DirRel != "meadow" {
		t.Fatalf("first main row = %+v, want meadow dir", main[0].Value)
	}
	if app.model.DedupView.Main.Selected != 0 {
		t.Fatalf("main cursor = %d, want 0 on meadow", app.model.DedupView.Main.Selected)
	}

	meadowLantern := filepath.Join(dir, "meadow", "lantern.txt")
	meadowBeacon := filepath.Join(dir, "meadow", "beacon.txt")
	orchardLantern := filepath.Join(dir, "orchard", "lantern.txt")
	rootLantern := filepath.Join(dir, "lantern.txt")

	// Insert on meadow in file-tree: mark all files under meadow, advance to orchard.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 2 {
		t.Fatalf("MarkedCount = %d, want 2 after meadow folder mark", app.model.DedupView.MarkedCount)
	}
	if !app.model.DedupView.Marked[meadowLantern] || !app.model.DedupView.Marked[meadowBeacon] {
		t.Fatalf("meadow files not marked: %v", app.model.DedupView.Marked)
	}
	if app.model.DedupView.Marked[orchardLantern] || app.model.DedupView.Marked[rootLantern] {
		t.Fatalf("non-meadow files marked: %v", app.model.DedupView.Marked)
	}
	sel := app.model.DedupView.Main.Selected
	if sel < 0 || sel >= len(main) || main[sel].Value.DirRel != "orchard" {
		t.Fatalf("cursor on %q after meadow mark, want orchard dir", main[sel].Value.DirRel)
	}

	// Move back to meadow and Insert again: clear meadow marks.
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("MarkedCount = %d, want 0 after meadow folder clear", app.model.DedupView.MarkedCount)
	}

	// Partial mark under meadow, then folder Insert clears all under meadow.
	app.dedupCtrl.ExpandAll()
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)) // orchard dir
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowFile && r.Value.File.Rel == "meadow/beacon.txt" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)) // mark beacon only
	if app.model.DedupView.MarkedCount != 1 || !app.model.DedupView.Marked[meadowBeacon] {
		t.Fatalf("partial mark failed: count=%d marked=%v", app.model.DedupView.MarkedCount, app.model.DedupView.Marked)
	}
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("folder clear after partial mark: count=%d, want 0", app.model.DedupView.MarkedCount)
	}
	if app.model.DedupView.Marked[meadowBeacon] || app.model.DedupView.Marked[meadowLantern] {
		t.Fatalf("meadow marks remain: %v", app.model.DedupView.Marked)
	}

	// C-k keep on meadow/lantern, then folder Insert must not mark kept files.
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowFile && r.Value.File.Rel == "meadow/lantern.txt" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(dedupCtrlK())
	if !app.model.DedupView.Kept[meadowLantern] {
		t.Fatalf("meadow lantern not kept: %v", app.model.DedupView.Kept)
	}
	for i, r := range app.model.DedupList {
		if r.Value.Kind == ui.DedupRowDir && r.Value.DirRel == "meadow" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.Marked[meadowLantern] {
		t.Fatal("kept meadow lantern must not be marked for deletion")
	}
	if !app.model.DedupView.Marked[meadowBeacon] {
		t.Fatal("non-kept meadow beacon should be marked after folder Insert")
	}
}

func TestDedupViewCopiesPaneSelectAll(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("meadow", "beacon.txt"),
		filepath.Join("orchard", "lantern.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Select root lantern.txt and focus the copies pane.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus copies pane")
	}

	meadowLantern := filepath.Join(dir, "meadow", "lantern.txt")
	meadowBeacon := filepath.Join(dir, "meadow", "beacon.txt")
	orchardLantern := filepath.Join(dir, "orchard", "lantern.txt")

	star := tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModNone)
	if id, ok := app.keys.Global.Lookup(star); !ok || id != keymap.ActionPanelInvertSelection {
		t.Fatalf("* key = %q %v, want panel.invert-selection", id, ok)
	}

	// * in main pane is a no-op.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not return focus to main pane")
	}
	app.handleDedupViewKey(star)
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("main pane * marked %d files, want no-op", app.model.DedupView.MarkedCount)
	}

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))

	// * marks all copy-pane files (three other copies).
	app.handleDedupViewKey(star)
	if app.model.DedupView.MarkedCount != 3 {
		t.Fatalf("MarkedCount = %d, want 3 after select all", app.model.DedupView.MarkedCount)
	}
	for _, p := range []string{meadowLantern, meadowBeacon, orchardLantern} {
		if !app.model.DedupView.Marked[p] {
			t.Fatalf("%q not marked after select all", p)
		}
	}

	// * again clears all copy-pane marks.
	app.handleDedupViewKey(star)
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("MarkedCount = %d, want 0 after second *", app.model.DedupView.MarkedCount)
	}

	// Partial mark, then * selects the rest.
	app.model.DedupView.Marked[meadowBeacon] = true
	app.model.DedupView.MarkedCount = 1
	app.model.DedupView.MarkedReclaimBytes = int64(len("dup"))
	app.handleDedupViewKey(star)
	if app.model.DedupView.MarkedCount != 3 {
		t.Fatalf("MarkedCount = %d, want 3 after select all from partial", app.model.DedupView.MarkedCount)
	}

	// Collapsed copies pane still includes hidden files.
	app.dedupCtrl.CollapseAll()
	app.handleDedupViewKey(star) // clear all
	app.handleDedupViewKey(star) // select all
	if app.model.DedupView.MarkedCount != 3 {
		t.Fatalf("collapsed select all: MarkedCount = %d, want 3", app.model.DedupView.MarkedCount)
	}
}

func TestDedupViewRightExpandsCollapsedDirInPlace(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "orchard", "deep", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("orchard", "deep", "deeper", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.dedupCtrl.CollapseAll()
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	rows := app.model.DedupList
	if len(rows) != 2 || rows[0].ID != "d:orchard" {
		t.Fatalf("collapsed rows = %+v, want [d:orchard, root lantern]", rows)
	}

	// First Right on orchard: expand only, cursor stays on the folder row.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	rows = app.model.DedupList
	sel := app.model.DedupView.Main.Selected
	if sel != 0 || rows[sel].ID != "d:orchard" {
		t.Fatalf("cursor on %q at %d, want orchard dir at 0", rows[sel].ID, sel)
	}
	if got := len(rows); got != 3 { // orchard, deep (collapsed), root lantern.txt
		t.Fatalf("rows after first Right = %d, want 3", got)
	}
	if !rows[0].Expanded {
		t.Fatal("orchard should be expanded after first Right")
	}
}

func TestDedupViewRightDescendsToFirstFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "orchard", "deep", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("orchard", "deep", "deeper", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.dedupCtrl.CollapseAll()
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))

	// Expand orchard in place, then descend on the second Right.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	rows := app.model.DedupList
	sel := app.model.DedupView.Main.Selected
	if sel < 0 || sel >= len(rows) || rows[sel].Value.Kind != ui.DedupRowFile {
		t.Fatalf("cursor on %q (kind=%v), want first file under orchard", rows[sel].ID, rows[sel].Value.Kind)
	}
	if got := rows[sel].Value.File.Rel; got != "orchard/deep/deeper/lantern.txt" {
		t.Fatalf("selected file rel = %q, want orchard/deep/deeper/lantern.txt", got)
	}
	if got := len(rows); got != 5 { // orchard, deep, deeper, deeper/lantern.txt, root lantern.txt
		t.Fatalf("rows after descend = %d, want 5", got)
	}
	if !rows[0].Expanded {
		t.Fatal("orchard should stay expanded after descend")
	}
}

func TestDedupViewRightOnExpandedDirDoesNotCollapse(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Expand meadow in place, then descend on the second Right without collapsing.
	app.dedupCtrl.CollapseAll()
	for i, r := range app.model.DedupList {
		if r.ID == "d:meadow" {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	beforeRows := len(app.model.DedupList)
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if got := len(app.model.DedupList); got != beforeRows {
		t.Fatalf("rows after Right on expanded dir = %d, want %d (no collapse)", got, beforeRows)
	}
	sel := app.model.DedupView.Main.Selected
	row := app.model.DedupList[sel]
	if row.Value.Kind != ui.DedupRowFile || row.Value.File.Rel != "meadow/lantern.txt" {
		t.Fatalf("cursor on %+v, want meadow/lantern.txt file", row)
	}
}

func TestDedupViewCopiesPaneRightDescendsToFirstFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("meadow", "beacon.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)) // root lantern.txt
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if !app.model.DedupView.FocusCopies {
		t.Fatal("Tab did not focus copies pane")
	}
	app.dedupCtrl.CollapseAll()
	for i, r := range app.model.DedupCopiesList {
		if r.ID == "d:meadow" {
			app.model.DedupView.Copies.Selected = i
			break
		}
	}

	// First Right expands meadow in place; second Right descends to the first file.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	rows := app.model.DedupCopiesList
	sel := app.model.DedupView.Copies.Selected
	if sel < 0 || sel >= len(rows) || rows[sel].ID != "d:meadow" {
		t.Fatalf("copies cursor on %q at %d, want meadow dir after first Right", rows[sel].ID, sel)
	}
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	rows = app.model.DedupCopiesList
	sel = app.model.DedupView.Copies.Selected
	if sel < 0 || sel >= len(rows) || rows[sel].Value.Kind != ui.DedupRowFile {
		t.Fatalf("copies cursor on %q (kind=%v), want first file under meadow", rows[sel].ID, rows[sel].Value.Kind)
	}
	if got := rows[sel].Value.File.Rel; got != "meadow/beacon.txt" {
		t.Fatalf("copies selected file rel = %q, want meadow/beacon.txt", got)
	}
}

func TestDedupViewTreeCollapseAndModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lantern.txt", filepath.Join("meadow", "lantern.txt")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("dirs-mode rows = %d, want 2 (meadow dir collapsed + root lantern.txt)", got)
	}

	// Ctrl+T switches to the groups tree: one collapsed group header.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if app.model.DedupView.TreeDirs {
		t.Fatal("Ctrl+T did not switch to groups tree mode")
	}
	if got := len(app.model.DedupList); got != 1 {
		t.Fatalf("groups-mode rows = %d, want 1 (collapsed header)", got)
	}

	// Right expands the group header; the copy row appears.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("after expand rows = %d, want 2", got)
	}
	// Left collapses again.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt))
	if got := len(app.model.DedupList); got != 1 {
		t.Fatalf("after collapse rows = %d, want 1", got)
	}
	// Right again re-expands.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("after re-expand rows = %d, want 2", got)
	}

	// Ctrl+T switches back to the directory tree.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if !app.model.DedupView.TreeDirs {
		t.Fatal("Ctrl+T did not switch back to directory tree mode")
	}
	if got := len(app.model.DedupList); got != 2 {
		t.Fatalf("dirs-mode rows = %d, want 2 (collapse state preserved)", got)
	}
}

func TestDedupViewMarkAndDelete(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	// Insert marks the selected file for deletion.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if got := len(app.dedupCtrl.MarkedPaths()); got != 1 {
		t.Fatalf("marked = %d, want 1", got)
	}
	if app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1", app.model.DedupView.MarkedCount)
	}
	if app.model.DedupView.MarkedReclaimBytes != int64(len("dup")) {
		t.Fatalf("MarkedReclaimBytes = %d, want %d", app.model.DedupView.MarkedReclaimBytes, len("dup"))
	}
	// F8 opens the standard delete confirmation dialog listing the marked files.
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !app.deleteDialogOpen() {
		t.Fatal("delete key did not open the delete dialog")
	}
	if got := len(app.model.FileDialog.DeleteEntries); got != 1 {
		t.Fatalf("delete entries = %d, want 1", got)
	}
	// Confirming (Yes) deletes directly: the other duplicate (b.txt) still lives
	// in dir, so the delete leaves no directory empty and the cleanup
	// confirmation must not appear.
	app.executeDelete()
	if app.deleteDialogOpen() {
		t.Fatal("delete dialog not closed after confirm")
	}
	if app.model.DedupEmptyDirsConfirm.Open {
		t.Fatal("empty-dirs confirm dialog opened even though no directory is left empty")
	}
	if len(app.model.DedupSnapshot.Groups) != 0 {
		t.Fatalf("groups after delete = %d, want 0 (group drops below 2)", len(app.model.DedupSnapshot.Groups))
	}
}

// TestDedupViewEmptyDirsConfirmDefaultsYesAndRemoves verifies that answering
// the empty-dirs confirmation with the Yes default removes the now-empty
// directory left behind by the delete, while answering No leaves it.
func TestDedupViewEmptyDirsConfirmDefaultsYesAndRemoves(t *testing.T) {
	for _, removeEmpty := range []bool{true, false} {
		t.Run(map[bool]string{true: "yes", false: "no"}[removeEmpty], func(t *testing.T) {
			// sub is the scan root itself, containing only the duplicate pair, so
			// deleting both leaves it empty.
			parent := t.TempDir()
			sub := filepath.Join(parent, "sub")
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sub, "a.txt"), []byte("dup"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sub, "b.txt"), []byte("dup"), 0o644); err != nil {
				t.Fatal(err)
			}

			screen := newScreen(t, 80, 24)
			app := newApp(t, screen, sub)
			app.openFindDuplicates()
			waitDedupDone(t, app)

			// Mark both duplicate rows (Insert marks + advances cursor, per panel.select-toggle).
			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
			if got := len(app.dedupCtrl.MarkedPaths()); got != 2 {
				t.Fatalf("marked paths = %d, want 2", got)
			}

			app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
			if !app.deleteDialogOpen() {
				t.Fatal("delete key did not open the delete dialog")
			}
			app.executeDelete()
			if !app.model.DedupEmptyDirsConfirm.Open {
				t.Fatal("empty-dirs confirm dialog did not open")
			}
			if app.model.DedupEmptyDirsConfirm.Focus != 0 {
				t.Fatalf("empty-dirs confirm default focus = %d, want 0 (Yes)", app.model.DedupEmptyDirsConfirm.Focus)
			}
			if got := app.model.DedupEmptyDirsConfirm.Dirs; len(got) != 1 || got[0] != "." {
				t.Fatalf("empty-dirs confirm dirs = %v, want [\".\"] (sub is the scan root)", got)
			}
			if removeEmpty {
				app.handleDedupEmptyDirsConfirmKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
			} else {
				app.handleDedupEmptyDirsConfirmKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
			}

			flushBackgroundJobs(t, app)
			waitUntilAppJobsFinished(t, app, 5*time.Second)

			_, err := os.Stat(sub)
			if removeEmpty {
				if !os.IsNotExist(err) {
					t.Fatalf("sub dir stat err = %v, want IsNotExist (dir should be removed)", err)
				}
			} else if err != nil {
				t.Fatalf("sub dir stat err = %v, want nil (dir should remain)", err)
			}
		})
	}
}

func TestDedupViewMenuOmitsBrowserItems(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()
	waitDedupDone(t, app)

	for _, def := range menu.ActiveDefinitions(app.model.MenuDefinitions) {
		switch def.ID {
		case menu.TopPanelLeft, menu.TopPanelRight, menu.TopCommand, menu.TopOptions:
			t.Fatalf("dedup view menu bar includes unavailable top menu %q", def.Label)
		}
		for _, item := range def.Items {
			switch item.Action {
			case "file.edit", "file.view", "copy", "move", "panel.sort-dialog":
				t.Fatalf("dedup view menu includes unavailable item %q", item.Label)
			}
		}
	}
}

func TestDedupViewFooterEscFirst(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()
	waitDedupDone(t, app)
	keys := app.activeFooterKeys()
	if len(keys) == 0 {
		t.Fatal("footer keys empty")
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	if len(keys) < 2 || keys[1].Key != tcell.KeyF1 || keys[1].Hint != "Help" {
		t.Fatalf("footer[1] = %+v, want F1 Help", keys[1])
	}
	var foundDelete, foundRefresh bool
	for _, fk := range keys {
		if fk.Hint == "Delete" {
			foundDelete = true
		}
		if fk.Hint == "Refresh" {
			foundRefresh = true
		}
		if fk.Hint == "Edit" || fk.Hint == "View" || fk.Hint == "Copy" {
			t.Fatalf("footer lists unavailable browser action %q: %+v", fk.Hint, keys)
		}
	}
	if !foundDelete {
		t.Fatalf("footer missing Delete: %+v", keys)
	}
	if !foundRefresh {
		t.Fatalf("footer missing Refresh: %+v", keys)
	}
	assertDedupFooterGroupOnlyHints(t, keys, false)
}

func TestDedupViewFooterSortHintInGroupsMode(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.openFindDuplicates()
	waitDedupDone(t, app)
	if !app.model.DedupView.TreeDirs {
		t.Fatal("dedup view should start in directory-tree mode")
	}
	assertDedupFooterGroupOnlyHints(t, app.activeFooterKeys(), false)

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl))
	if app.model.DedupView.TreeDirs {
		t.Fatal("Ctrl+T did not switch to groups tree mode")
	}
	assertDedupFooterGroupOnlyHints(t, app.activeFooterKeys(), true)
}

func assertDedupFooterGroupOnlyHints(t *testing.T, keys []menu.FunctionKey, wantGroupOnlyHints bool) {
	t.Helper()
	groupOnlyHints := []string{"Sort"}
	for _, hint := range groupOnlyHints {
		found := false
		for _, fk := range keys {
			if fk.Hint == hint {
				found = true
				break
			}
		}
		if wantGroupOnlyHints && !found {
			t.Fatalf("footer missing %q in groups mode: %+v", hint, keys)
		}
		if !wantGroupOnlyHints && found {
			t.Fatalf("footer lists %q in directory-tree mode: %+v", hint, keys)
		}
	}
}

func TestDedupProgressDialogFooterShowsEscAndF10(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())
	app.model.DedupProgressDialog.Open = true
	app.model.DedupSnapshot.Phase = comparepkg.DedupHashing

	keys := app.activeFooterKeys()
	if len(keys) != 2 {
		t.Fatalf("footer keys = %+v, want Esc and F10 only", keys)
	}
	if keys[0] != menu.FooterEscClose {
		t.Fatalf("footer[0] = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyF10 || keys[1].Hint != "Quit" {
		t.Fatalf("footer[1] = %+v, want F10 Quit", keys[1])
	}
}

func dedupCtrlK() *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModCtrl)
}

func TestDedupViewKeepMarking(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(aPath, []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte("dup"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.handleDedupViewKey(dedupCtrlK())
	if !app.model.DedupView.Kept[aPath] {
		t.Fatalf("Kept missing %q: %v", aPath, app.model.DedupView.Kept)
	}
	if app.model.DedupView.MarkedCount != 1 {
		t.Fatalf("MarkedCount = %d, want 1", app.model.DedupView.MarkedCount)
	}
	if !app.model.DedupView.Marked[bPath] {
		t.Fatalf("sibling not marked for deletion: %v", app.model.DedupView.Marked)
	}
	if app.model.Message == "Duplicate keep" {
		t.Fatal("first keep should not show Duplicate keep toast")
	}

	for i, r := range app.model.DedupList {
		if r.Value.AbsKey == aPath {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	markedBefore := app.model.DedupView.MarkedCount
	selBefore := app.model.DedupView.Main.Selected
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if app.model.DedupView.Main.Selected != selBefore {
		t.Fatalf("Insert on kept file moved cursor from %d to %d", selBefore, app.model.DedupView.Main.Selected)
	}
	if app.model.DedupView.MarkedCount != markedBefore {
		t.Fatalf("Insert on kept file changed MarkedCount from %d to %d", markedBefore, app.model.DedupView.MarkedCount)
	}

	for i, r := range app.model.DedupList {
		if r.Value.AbsKey == bPath {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(dedupCtrlK())
	if app.model.Message != "Duplicate keep" {
		t.Fatalf("message = %q, want Duplicate keep toast when switching keeper", app.model.Message)
	}
	if !app.model.DedupView.Kept[bPath] || app.model.DedupView.Kept[aPath] {
		t.Fatalf("keeper switch failed: Kept = %v", app.model.DedupView.Kept)
	}
	if !app.model.DedupView.Marked[aPath] || app.model.DedupView.Marked[bPath] {
		t.Fatalf("keeper switch marks wrong: %v", app.model.DedupView.Marked)
	}

	for i, r := range app.model.DedupList {
		if r.Value.AbsKey == bPath {
			app.model.DedupView.Main.Selected = i
			break
		}
	}
	app.handleDedupViewKey(dedupCtrlK())
	if len(app.model.DedupView.Kept) != 0 {
		t.Fatalf("toggle-off keep left Kept = %v", app.model.DedupView.Kept)
	}
	if app.model.DedupView.MarkedCount != 0 {
		t.Fatalf("toggle-off keep left MarkedCount = %d, want 0", app.model.DedupView.MarkedCount)
	}
}

func TestDedupViewCopiesPaneKeep(t *testing.T) {
	dir := t.TempDir()
	rootLantern := filepath.Join(dir, "lantern.txt")
	meadowLantern := filepath.Join(dir, "meadow", "lantern.txt")
	orchardLantern := filepath.Join(dir, "orchard", "lantern.txt")
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{rootLantern, meadowLantern, orchardLantern} {
		if err := os.WriteFile(p, []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	for i, r := range app.model.DedupCopiesList {
		if r.Value.Kind == ui.DedupRowFile && r.Value.AbsKey == meadowLantern {
			app.model.DedupView.Copies.Selected = i
			break
		}
	}

	app.handleDedupViewKey(dedupCtrlK())
	if !app.model.DedupView.Kept[meadowLantern] {
		t.Fatalf("copy not kept: %v", app.model.DedupView.Kept)
	}
	if !app.model.DedupView.Marked[rootLantern] || !app.model.DedupView.Marked[orchardLantern] {
		t.Fatalf("other copies not marked: %v", app.model.DedupView.Marked)
	}
	if app.model.DedupView.Marked[meadowLantern] {
		t.Fatal("kept copy must not be marked for deletion")
	}
}

func TestDedupViewCopiesPaneFolderKeep(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "meadow"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "orchard"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"lantern.txt",
		filepath.Join("meadow", "lantern.txt"),
		filepath.Join("meadow", "beacon.txt"),
		filepath.Join("orchard", "lantern.txt"),
	} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("dup"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	waitDedupDone(t, app)

	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.handleDedupViewKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))

	meadowLantern := filepath.Join(dir, "meadow", "lantern.txt")
	meadowBeacon := filepath.Join(dir, "meadow", "beacon.txt")
	orchardLantern := filepath.Join(dir, "orchard", "lantern.txt")
	rootLantern := filepath.Join(dir, "lantern.txt")

	app.handleDedupViewKey(dedupCtrlK())
	if !app.model.DedupView.Kept[meadowLantern] || !app.model.DedupView.Kept[meadowBeacon] {
		t.Fatalf("meadow files not kept: %v", app.model.DedupView.Kept)
	}
	if !app.model.DedupView.Marked[rootLantern] || !app.model.DedupView.Marked[orchardLantern] {
		t.Fatalf("non-meadow copies not marked: %v", app.model.DedupView.Marked)
	}
	if app.model.DedupView.Marked[meadowLantern] || app.model.DedupView.Marked[meadowBeacon] {
		t.Fatalf("meadow files must not be marked: %v", app.model.DedupView.Marked)
	}
}

func TestDedupProgressDialogMenuBarNotInteractive(t *testing.T) {
	m := ui.Model{
		DedupProgressDialog: dialog.DedupProgressDialogState{Open: true},
		DedupSnapshot:       comparepkg.DedupSnapshot{Phase: comparepkg.DedupHashing},
	}
	if m.MenuBarInteractive() {
		t.Fatal("dedup progress dialog must hide interactive menu bar")
	}
	if !m.ModalDialogOpen() {
		t.Fatal("dedup progress dialog must count as modal")
	}
}

func TestDedupProgressDialogCancelClosesScan(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openFindDuplicates()
	if !app.model.DedupProgressDialog.Open {
		t.Fatal("expected progress dialog open")
	}

	app.handleDedupProgressDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.DedupProgressDialog.Open {
		t.Fatal("progress dialog should close after cancel")
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want ViewBrowser after cancel", app.model.ViewMode)
	}
}
