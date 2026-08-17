package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"golang.org/x/text/encoding/japanese"
)

func TestHandleKeyOpeningFileDialogRendersDialogFooterImmediately(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F7) quit = true, want false")
	}
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		t.Fatalf("file dialog = %+v, want open mkdir dialog", app.model.FileDialog)
	}

	footer := screenLine(screen, 19, 80)
	if strings.Contains(footer, "Mkdir") {
		t.Fatalf("footer = %q, should hide browser F7 hint immediately", footer)
	}
	if !strings.Contains(footer, "Esc") || !strings.Contains(footer, "Close") {
		t.Fatalf("footer = %q, want Esc Close first", footer)
	}
	if !strings.Contains(footer, "F10") || !strings.Contains(footer, "Quit") {
		t.Fatalf("footer = %q, want dialog footer with F10 Quit", footer)
	}
}

func TestFileMenuExitQuits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	// Open pulldown first, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModNone))

	if !quit {
		t.Fatal("handleKey() quit = false, want true")
	}
}

func TestFileMenuRenameOpensRenameDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'r')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogRename {
		t.Fatalf("dialog type = %d, want FileDialogRename", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.Fields) != 1 || app.model.FileDialog.Fields[0].Value != "test.txt" {
		t.Fatalf("expected prefilled rename field, got %+v", app.model.FileDialog.Fields)
	}
}

func TestFileMenuMkdirOpensMkdirDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'm')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		t.Fatalf("dialog type = %d, want FileDialogMkdir", app.model.FileDialog.DialogType)
	}
}

func TestQuestionMarkInsertsLiteralInFileDialogInsteadOfOpeningHelp(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone)); quit {
		t.Fatal("handleKey(F7) quit = true, want false")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone)); quit {
		t.Fatal("handleKey('?') quit = true, want false")
	}

	if app.model.HelpView.Open {
		t.Fatal("'?' should not open help while a file dialog is open")
	}
	if len(app.model.FileDialog.Fields) != 1 || app.model.FileDialog.Fields[0].Value != "?" {
		t.Fatalf("expected '?' typed into mkdir field, got %+v", app.model.FileDialog.Fields)
	}
}

func TestFileMenuDeleteOpensDeleteConfirmation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'd')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogDelete {
		t.Fatalf("dialog type = %d, want FileDialogDelete", app.model.FileDialog.DialogType)
	}
	if got := app.model.FileDialog.DeleteSummary; !strings.HasPrefix(got, "1 file (") {
		t.Fatalf("DeleteSummary = %q, want prefix %q", got, "1 file (")
	}
	if len(app.model.FileDialog.DeleteEntries) != 1 || app.model.FileDialog.DeleteEntries[0].Name != "test.txt" {
		t.Fatalf("DeleteEntries = %v, want [test.txt]", app.model.FileDialog.DeleteEntries)
	}
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (No)", app.model.FileDialog.FocusedField)
	}
}

func TestFileMenuExtractOpensExtractDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "archive.zip"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'x')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogExtract {
		t.Fatalf("dialog type = %d, want FileDialogExtract", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.ExtractSources) != 1 {
		t.Fatalf("ExtractSources len = %d, want 1", len(app.model.FileDialog.ExtractSources))
	}
}

func TestExtractDialogEnqueuesJob(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	srcDir := t.TempDir()
	destDir := t.TempDir()
	inner := filepath.Join(srcDir, "hello.txt")
	writeFile(t, inner)
	archivePath := filepath.Join(srcDir, "pack.tar.gz")
	cmd := exec.Command("tar", "-czf", archivePath, "-C", srcDir, "hello.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	app.model.Secondary.Path = pathloc.MustParse(destDir)
	_ = app.model.Secondary.Refresh(20)
	if !app.activePanel().SelectVisibleEntry("pack.tar.gz") {
		t.Fatal("pack.tar.gz not visible in panel")
	}

	app.dispatch(keymap.ActionFileExtract)
	if !app.model.FileDialog.Open {
		t.Fatal("extract dialog not open")
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	flushBackgroundJobs(t, app)

	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs len = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeExtract {
		t.Fatalf("job type = %v, want TypeExtract", all[0].Type)
	}
	if all[0].Destination.String() != destDir {
		t.Fatalf("destination = %q, want %q", all[0].Destination, destDir)
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
	if all[0].Status != jobs.StatusCompleted {
		t.Fatalf("status = %q, want completed (%s)", all[0].Status, all[0].Error)
	}
	if _, err := os.Stat(filepath.Join(destDir, "hello.txt")); err != nil {
		t.Fatalf("extracted file missing: %v", err)
	}
}

func TestFileMenuChmodOpensChmodDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'h')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogChmod {
		t.Fatalf("dialog type = %d, want FileDialogChmod", app.model.FileDialog.DialogType)
	}
}

func TestFileMenuChownOpensChownDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'o')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogChown {
		t.Fatalf("dialog type = %d, want FileDialogChown", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.Fields) != 2 {
		t.Fatalf("chown has %d fields, want 2", len(app.model.FileDialog.Fields))
	}
}

func TestFileMenuSymlinkOpensSymlinkDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 's')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogSymlink {
		t.Fatalf("dialog type = %d, want FileDialogSymlink", app.model.FileDialog.DialogType)
	}
}

func TestFileMenuHardlinkOpensHardlinkDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'l')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogHardlink {
		t.Fatalf("dialog type = %d, want FileDialogHardlink", app.model.FileDialog.DialogType)
	}
}

func TestFileDialogEscCancelsAndNoMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'r')
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Esc")
	}
}

func TestFileDialogEnterExecutesRename(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	writeFile(t, oldPath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Dispatch rename via keybinding (Shift+F6).
	action := keymap.ActionFileRename
	app.dispatch(action)

	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	// Clear prefill by typing a character, enter new name.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModNone))

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("renamed file not found: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old file should not exist")
	}
	if app.model.Message == "" {
		t.Fatal("expected success message after rename")
	}
}

func TestRenameDialogFocusCheckboxDefaultOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	app.dispatch(keymap.ActionFileRename)

	if !app.model.FileDialog.Open {
		t.Fatal("rename dialog should be open")
	}
	if app.model.FileDialog.RenameFocusAfter {
		t.Fatal("RenameFocusAfter = true, want false (default)")
	}
	if got, want := dialog.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 4; got != want {
		t.Fatalf("focus count = %d, want %d (field + checkbox + OK + Cancel)", got, want)
	}
}

func TestRenameDialogConfigFocusAfterDefaultOn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old.txt"))

	screen := newScreen(t, 80, 20)
	cfg := config.Default()
	cfg.Operations.RenameFocusAfter = true
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	app.config.UI.KeyRepeatDebounceMS = 0

	app.dispatch(keymap.ActionFileRename)
	if !app.model.FileDialog.RenameFocusAfter {
		t.Fatal("RenameFocusAfter = false, want true from config")
	}
}

func TestRenameWithoutFocusAfterDoesNotCenterOnNewFile(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("%02d.txt", i)))
	}
	target := filepath.Join(dir, "10.txt")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Path == target {
			p.Cursor = i
			break
		}
	}

	newName := "99.txt"
	app.dispatch(keymap.ActionFileRename)
	for _, r := range newName {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Primary
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Secondary

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false after rename")
	}
	if entry.Name == newName {
		t.Fatalf("cursor entry = %q, want index fallback not focus-after selection", entry.Name)
	}
	if entry.Name != "11.txt" {
		t.Fatalf("cursor entry = %q, want 11.txt at prior index", entry.Name)
	}
}

func TestRenameWithFocusAfterSelectsAndCentersNewFile(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("%02d.txt", i)))
	}
	target := filepath.Join(dir, "10.txt")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Path == target {
			p.Cursor = i
			break
		}
	}

	newName := "99.txt"
	app.dispatch(keymap.ActionFileRename)
	for _, r := range newName {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModAlt))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Primary
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Secondary

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false after rename")
	}
	if entry.Name != newName {
		t.Fatalf("cursor entry = %q, want %s", entry.Name, newName)
	}
	vp := app.activeViewportRows()
	wantScroll := p.Cursor - vp/2
	if wantScroll < 0 {
		wantScroll = 0
	}
	maxOffset := p.VisibleEntryCount() - vp
	if maxOffset < 0 {
		maxOffset = 0
	}
	if wantScroll > maxOffset {
		wantScroll = maxOffset
	}
	if p.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d (centered on renamed entry)", p.ScrollOffset, wantScroll)
	}
}

func TestMassRenameTwoSelectedFiles(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "foo_a.txt")
	bPath := filepath.Join(dir, "foo_b.txt")
	writeFile(t, aPath)
	writeFile(t, bPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true, bPath: true}

	app.dispatch(keymap.ActionFileRename)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog, open=%v type=%v", app.model.FileDialog.Open, app.model.FileDialog.DialogType)
	}
	d := &app.model.FileDialog
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("FocusedField = %d, want %d (Find)", d.FocusedField, dialog.MassRenameFindFieldFocus)
	}
	for _, r := range "foo_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameFindFieldFocus + 1 // Replace field (0-3 = radios, 4 = Find, 5 = Replace)
	for _, r := range "bar_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "bar_a.txt")); err != nil {
		t.Fatalf("bar_a.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bar_b.txt")); err != nil {
		t.Fatalf("bar_b.txt: %v", err)
	}
	if _, err := os.Stat(aPath); !os.IsNotExist(err) {
		t.Fatal("old path foo_a.txt should not exist")
	}
}

func TestMassRenameApplyKeepsDialogOpenAndReselects(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "foo_a.txt")
	bPath := filepath.Join(dir, "foo_b.txt")
	writeFile(t, aPath)
	writeFile(t, bPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true, bPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog, open=%v type=%v", d.Open, d.DialogType)
	}

	// First pass: foo_ -> bar_, applied via the Apply button.
	for _, r := range "foo_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameFindFieldFocus + 1 // Replace field
	for _, r := range "bar_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameApplyFocusIndex(*d)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if !d.Open {
		t.Fatal("dialog should stay open after Apply")
	}
	bar1Path := filepath.Join(dir, "bar_a.txt")
	bar2Path := filepath.Join(dir, "bar_b.txt")
	if _, err := os.Stat(bar1Path); err != nil {
		t.Fatalf("bar_a.txt: %v", err)
	}
	if _, err := os.Stat(bar2Path); err != nil {
		t.Fatalf("bar_b.txt: %v", err)
	}
	if d.Fields[0].Value != "" || d.Fields[1].Value != "" {
		t.Fatalf("expected fields cleared after Apply, got %+v / %+v", d.Fields[0], d.Fields[1])
	}
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("FocusedField after Apply = %d, want %d (Find)", d.FocusedField, dialog.MassRenameFindFieldFocus)
	}
	p = app.activePanel()
	if !p.SelectedPaths[bar1Path] || !p.SelectedPaths[bar2Path] {
		t.Fatalf("expected renamed batch reselected, got %+v", p.SelectedPaths)
	}
	if p.SelectedPaths[aPath] || p.SelectedPaths[bPath] {
		t.Fatalf("old paths should not remain selected, got %+v", p.SelectedPaths)
	}

	// Second pass: bar_ -> baz_, applied via OK to confirm the dialog closes normally.
	for _, r := range "bar_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameFindFieldFocus + 1
	for _, r := range "baz_" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.FileDialogOKFocusIndex(*d)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter on OK")
	}
	if _, err := os.Stat(filepath.Join(dir, "baz_a.txt")); err != nil {
		t.Fatalf("baz_a.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "baz_b.txt")); err != nil {
		t.Fatalf("baz_b.txt: %v", err)
	}
}

func TestMassRenameNoMatchMarksFindInvalid(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "foo_a.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	for _, r := range "nomatch" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if !d.Fields[0].InputInvalid {
		t.Fatal("expected Find input invalid when nothing matches")
	}
}

func TestMassRenameModeShortcutKeepsFindFocus(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("FocusedField = %d, want %d (Find)", d.FocusedField, dialog.MassRenameFindFieldFocus)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want regex", d.MassRenameMode)
	}
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("after Alt+R: focus = %d, want %d", d.FocusedField, dialog.MassRenameFindFieldFocus)
	}
	if d.Fields[0].Label != "Pattern" {
		t.Fatalf("label = %q, want Pattern", d.Fields[0].Label)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUISimple {
		t.Fatalf("mode = %v, want simple", d.MassRenameMode)
	}
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("after Alt+S: focus = %d, want %d", d.FocusedField, dialog.MassRenameFindFieldFocus)
	}
	if d.Fields[0].Label != "Find" {
		t.Fatalf("label = %q, want Find", d.Fields[0].Label)
	}
}

func TestMassRenameModeShortcutKeepsReplaceFocus(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	const replaceFocus = dialog.MassRenameFindFieldFocus + 1
	d.FocusedField = replaceFocus
	for _, r := range "y" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("setup: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want regex", d.MassRenameMode)
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("after Alt+R: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUISimple {
		t.Fatalf("mode = %v, want simple", d.MassRenameMode)
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("after Alt+S: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}
}

func TestMassRenameRadioFocusAppliesRegexMode(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if d.MassRenameMode != dialog.MassRenameModeUISimple {
		t.Fatalf("initial mode = %v, want simple", d.MassRenameMode)
	}
	// Up from Find (4) → showModified (6), Up → Capitalize radio (3), Up → External (2),
	// Up again → Regex (1)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if d.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (Regex radio)", d.FocusedField)
	}
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want regex after focusing radio", d.MassRenameMode)
	}
	if d.Fields[0].Label != "Pattern" {
		t.Fatalf("label = %q, want Pattern", d.Fields[0].Label)
	}
}

func TestMassRenameTabIntoModeRadiosKeepsCurrentMode(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("setup mode = %v, want regex", d.MassRenameMode)
	}
	if d.FocusedField != dialog.MassRenameFindFieldFocus {
		t.Fatalf("setup focus = %d, want Find", d.FocusedField)
	}

	// Backtab from Find should land on the Regex radio, not force Simple.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if d.FocusedField != 1 {
		t.Fatalf("after Backtab: focus = %d, want 1 (Regex radio)", d.FocusedField)
	}
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("after Backtab: mode = %v, want regex", d.MassRenameMode)
	}

	// Tab to buttons, then Tab again into mode radios — still Regex.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // Find
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // OK
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)) // mode radios
	if d.FocusedField != 1 {
		t.Fatalf("after Tab cycle: focus = %d, want 1 (Regex radio)", d.FocusedField)
	}
	if d.MassRenameMode != dialog.MassRenameModeUIRegex {
		t.Fatalf("after Tab cycle: mode = %v, want regex", d.MassRenameMode)
	}
}

func TestMassRenameCapitalizeTabCyclesThroughCheckboxes(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "wandering-elephant.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUICapitalize {
		t.Fatalf("mode = %v, want capitalize after Alt+Z", d.MassRenameMode)
	}

	okIdx := dialog.FileDialogOKFocusIndex(*d)
	if okIdx != 8 {
		t.Fatalf("okIdx = %d, want 8", okIdx)
	}

	// Down steps through the two capitalize checkbox rows individually (mirrors Find/Replace
	// in Simple/Regex mode), then reaches OK.
	d.FocusedField = 3 // Capitalize radio
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if want := dialog.MassRenameShowModifiedFocusIdx(*d); d.FocusedField != want || want != 4 {
		t.Fatalf("Down from radio: focus = %d, want 4 (Show only modified)", d.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if want := dialog.MassRenameCapEachWordFocusIdx(*d); d.FocusedField != want || want != 6 {
		t.Fatalf("Down from options row: focus = %d, want 6 (Capitalize each word)", d.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if want := dialog.MassRenameCapPunctFocusIdx(*d); d.FocusedField != want || want != 7 {
		t.Fatalf("Down from Capitalize each word: focus = %d, want 7 (Treat punctuation as separators)", d.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if d.FocusedField != okIdx {
		t.Fatalf("Down from Treat punctuation: focus = %d, want %d (OK)", d.FocusedField, okIdx)
	}

	// Up retraces the same path in reverse.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if want := dialog.MassRenameCapPunctFocusIdx(*d); d.FocusedField != want {
		t.Fatalf("Up from OK: focus = %d, want %d (Treat punctuation as separators)", d.FocusedField, want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if want := dialog.MassRenameCapEachWordFocusIdx(*d); d.FocusedField != want {
		t.Fatalf("Up: focus = %d, want %d (Capitalize each word)", d.FocusedField, want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if want := dialog.MassRenameShowModifiedFocusIdx(*d); d.FocusedField != want {
		t.Fatalf("Up: focus = %d, want %d (Show only modified)", d.FocusedField, want)
	}

	// Tab jumps by segment (radio+options row -> checkbox rows -> buttons -> radio), same
	// pattern as Find/Replace in Simple/Regex mode.
	d.FocusedField = 3 // Capitalize radio
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if want := dialog.MassRenameCapEachWordFocusIdx(*d); d.FocusedField != want {
		t.Fatalf("Tab from radio: focus = %d, want %d (Capitalize each word)", d.FocusedField, want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if d.FocusedField != okIdx {
		t.Fatalf("Tab from checkbox segment: focus = %d, want %d (OK)", d.FocusedField, okIdx)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if d.FocusedField != 3 {
		t.Fatalf("Tab from OK: focus = %d, want 3 (Capitalize radio)", d.FocusedField)
	}
	if d.MassRenameMode != dialog.MassRenameModeUICapitalize {
		t.Fatalf("mode changed unexpectedly to %v after wrapping Tab", d.MassRenameMode)
	}

	// Backtab retraces the same path in reverse.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if d.FocusedField != okIdx {
		t.Fatalf("Backtab from radio: focus = %d, want %d (OK)", d.FocusedField, okIdx)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if d.FocusedField != 3 {
		t.Fatalf("Backtab from OK: focus = %d, want 3 (Capitalize radio)", d.FocusedField)
	}
}

func TestMassRenameClampFocusIntoAndOutOfCapitalize(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "wandering-elephant.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog

	altSwitch := func(r rune) {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModAlt))
	}

	// Simple -> Capitalize: Show only modified / Strip carry over to Capitalize's matching
	// checkbox; Case and Find/Replace (which Capitalize doesn't have) fall back to Capitalize's
	// "Show only modified".
	fromSimpleCases := []struct {
		label string
		idx   func() int
		want  int
	}{
		{"ShowModified", func() int { return dialog.MassRenameShowModifiedFocusIdx(*d) }, 4},
		{"Strip", func() int { return dialog.MassRenameStripFocusIdx(*d) }, 5},
		{"Case", func() int { return dialog.MassRenameCaseFocusIdx(*d) }, 4},
		{"Find", func() int { return dialog.MassRenameFindFieldFocus }, 4},
		{"Replace", func() int { return dialog.MassRenameFindFieldFocus + 1 }, 4},
	}
	for _, tc := range fromSimpleCases {
		altSwitch('s') // back to Simple
		d.FocusedField = tc.idx()
		altSwitch('z') // Simple -> Capitalize
		if d.MassRenameMode != dialog.MassRenameModeUICapitalize {
			t.Fatalf("%s: mode = %v, want capitalize", tc.label, d.MassRenameMode)
		}
		if d.FocusedField != tc.want {
			t.Fatalf("Simple(%s) -> Capitalize: focus = %d, want %d", tc.label, d.FocusedField, tc.want)
		}
	}

	// Capitalize -> Simple/Regex: the capitalize-only checkbox rows have no counterpart, so
	// focus falls back to the new mode's "Show only modified" (the default-branch case).
	altSwitch('z')
	d.FocusedField = dialog.MassRenameCapEachWordFocusIdx(*d)
	altSwitch('s')
	if want := dialog.MassRenameShowModifiedFocusIdx(*d); d.FocusedField != want || want != 6 {
		t.Fatalf("Capitalize(CapEachWord) -> Simple: focus = %d, want 6", d.FocusedField)
	}

	altSwitch('z')
	d.FocusedField = dialog.MassRenameCapPunctFocusIdx(*d)
	altSwitch('r')
	if want := dialog.MassRenameShowModifiedFocusIdx(*d); d.FocusedField != want || want != 6 {
		t.Fatalf("Capitalize(CapPunct) -> Regex: focus = %d, want 6", d.FocusedField)
	}

	// External <-> Capitalize share identical Show-only-modified(4)/Strip(5) indices, so focus
	// on either checkbox is preserved exactly across the switch, in both directions.
	altSwitch('e')
	d.FocusedField = dialog.MassRenameStripFocusIdx(*d)
	altSwitch('z')
	if d.FocusedField != 5 {
		t.Fatalf("External(Strip) -> Capitalize: focus = %d, want 5 (preserved)", d.FocusedField)
	}
	altSwitch('e')
	if d.FocusedField != 5 {
		t.Fatalf("Capitalize(Strip) -> External: focus = %d, want 5 (preserved)", d.FocusedField)
	}
}

func TestMassRenameCapPunctSepToggleEnablesCapEachWord(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "small-package-name.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog

	// Switch to Capitalize mode
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModAlt))
	if d.MassRenameMode != dialog.MassRenameModeUICapitalize {
		t.Fatalf("mode = %v, want capitalize after Alt+Z", d.MassRenameMode)
	}

	// Both checkboxes should start off
	if d.MassRenameCapEachWord {
		t.Fatal("MassRenameCapEachWord should start as false")
	}
	if d.MassRenameCapPunctSep {
		t.Fatal("MassRenameCapPunctSep should start as false")
	}

	// Toggle punctuation separator via Alt+P
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModAlt))

	// Both should now be on
	if !d.MassRenameCapPunctSep {
		t.Fatal("MassRenameCapPunctSep should be true after Alt+P")
	}
	if !d.MassRenameCapEachWord {
		t.Fatal("MassRenameCapEachWord should be automatically enabled when CapPunctSep is turned on")
	}

	// Toggle capitalize-each-word independently and verify it doesn't affect punctSep
	d.FocusedField = dialog.MassRenameCapEachWordFocusIdx(*d)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))

	if d.MassRenameCapEachWord {
		t.Fatal("MassRenameCapEachWord should be false after space toggle")
	}
	if !d.MassRenameCapPunctSep {
		t.Fatal("MassRenameCapPunctSep should remain true (not affected by CapEachWord toggle)")
	}
}

func TestMassRenameRegexCaptureGroupPreviewWithShiftDollar(t *testing.T) {
	dir := t.TempDir()
	season1 := filepath.Join(dir, "Season 1")
	if err := os.Mkdir(season1, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{season1: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	for _, r := range `(\d)$` {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameFindFieldFocus + 1 // Replacement field (0-3 = radios, 4 = Pattern, 5 = Replacement)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, '$', tcell.ModShift))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))

	if d.Fields[1].Value != "0$1" {
		t.Fatalf("replace = %q, want 0$1", d.Fields[1].Value)
	}
	if len(d.MassRenamePreviewAfter) != 1 || d.MassRenamePreviewAfter[0] != "Season 01" {
		t.Fatalf("preview after = %v, want Season 01", d.MassRenamePreviewAfter)
	}
	if d.MassRenameReplacementSyntaxHint == "" {
		t.Fatal("expected replacement syntax hint for capture group pattern")
	}
}

func TestMassRenameReplacementSyntaxHintHiddenWithoutGroups(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	for _, r := range `\.txt$` {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if d.MassRenameReplacementSyntaxHint != "" {
		t.Fatalf("hint = %q, want empty for pattern without capture groups", d.MassRenameReplacementSyntaxHint)
	}
}

func TestMassRenameRegexpCompileHintForBackslashPattern(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	d.FocusedField = dialog.MassRenameFindFieldFocus // Pattern field (0-3 = radios, 4 = Pattern)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, '\\', tcell.ModNone))

	if !d.Fields[0].InputInvalid {
		t.Fatal("expected invalid pattern field")
	}
	hint := strings.TrimSpace(d.MassRenamePatternCompileHint)
	if hint == "" {
		t.Fatal("expected regexp compile hint under pattern field")
	}
	if !strings.Contains(hint, "backslash") {
		t.Fatalf("hint = %q, want backslash detail", hint)
	}
}

func TestMassRenameEnterCancelClosesWithInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	// Regex mode + invalid pattern
	d.FocusedField = 1
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	d.FocusedField = dialog.MassRenameFindFieldFocus // Pattern field (0-3 = radios, 4 = Pattern)
	for _, r := range "a++" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if !d.Fields[0].InputInvalid {
		t.Fatal("expected invalid pattern")
	}
	d.FocusedField = dialog.FileDialogCancelFocusIndex(*d)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Enter on Cancel should close dialog even when regexp is invalid")
	}
}

func TestMassRenameConflictBlocksOKWithCriticalToast(t *testing.T) {
	dir := t.TempDir()
	names := []string{"Season1", "Season2", "Season3", "Season4"}
	paths := make([]string, len(names))
	for i, name := range names {
		p := filepath.Join(dir, name)
		paths[i] = p
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = make(map[string]bool, len(paths))
	for _, path := range paths {
		p.SelectedPaths[path] = true
	}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	for _, r := range "1" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = dialog.MassRenameFindFieldFocus + 1 // Replace field (0-3 = radios, 4 = Find, 5 = Replace)
	for _, r := range "2" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if len(d.MassRenamePreviewBefore) != len(paths) {
		t.Fatalf("preview rows = %d, want %d (no banner row)", len(d.MassRenamePreviewBefore), len(paths))
	}
	for _, lb := range d.MassRenamePreviewBefore {
		if strings.HasPrefix(lb, "!") {
			t.Fatalf("unexpected banner row %q", lb)
		}
	}
	conflictIdx := -1
	for i, lb := range d.MassRenamePreviewBefore {
		if lb == "Season1" {
			conflictIdx = i
			break
		}
	}
	if conflictIdx < 0 {
		t.Fatal("Season1 preview row missing")
	}
	if len(d.MassRenamePreviewAfterError) != len(paths) || !d.MassRenamePreviewAfterError[conflictIdx] {
		t.Fatalf("after error flags = %v, want index %d set", d.MassRenamePreviewAfterError, conflictIdx)
	}
	if dialog.FileDialogMassRenameOKEnabled(*d) {
		t.Fatal("OK action should be blocked when preview has conflicts")
	}
	okIdx := dialog.FileDialogOKFocusIndex(*d)
	// Navigate directly to OK to test the blocked-OK path.
	d.FocusedField = okIdx
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.model.FileDialog.Open {
		t.Fatal("Enter on OK with conflicts should not close dialog")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want MessageUrgencyCritical", app.model.MessageUrgency)
	}
	if strings.TrimSpace(app.model.Message) == "" {
		t.Fatal("expected critical toast explaining the conflict")
	}
	if !strings.Contains(app.model.Message, "Season2") {
		t.Fatalf("message = %q, want conflict detail mentioning Season2", app.model.Message)
	}
}

func TestFileDialogMkdirCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	for _, r := range "newdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	info, err := os.Stat(filepath.Join(dir, "newdir"))
	if err != nil {
		t.Fatalf("created directory not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
	}
}

func TestFileDialogInsertRune(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))

	field := &app.model.FileDialog.Fields[0]
	if field.Value != "n" {
		t.Fatalf("field value = %q, want n", field.Value)
	}
	if field.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", field.Cursor)
	}
}

func TestFileDialogBackspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "hello" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))

	field := &app.model.FileDialog.Fields[0]
	if field.Value != "hell" {
		t.Fatalf("field value = %q, want hell", field.Value)
	}
}

func TestFileDialogLeftRightMoveCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "abc" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	// cursor should be at 3 (end)
	field := &app.model.FileDialog.Fields[0]
	expectedCursor := 3
	if field.Cursor != expectedCursor {
		t.Fatalf("cursor = %d, want %d", field.Cursor, expectedCursor)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if field.Cursor != 2 {
		t.Fatalf("cursor after left = %d, want 2", field.Cursor)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if field.Cursor != 3 {
		t.Fatalf("cursor after right = %d, want 3", field.Cursor)
	}
}

func TestFileDialogCtrlWKillWord(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "/foo/bar" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	field := &app.model.FileDialog.Fields[0]
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if field.Value != "/foo/" || field.Cursor != 5 {
		t.Fatalf("after Ctrl+W: value=%q cursor=%d", field.Value, field.Cursor)
	}
}

func TestFileDialogHomeEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "abcdef" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	// cursor should be at 3 now (moved left 3 from 6)

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	field := &app.model.FileDialog.Fields[0]
	if field.Cursor != 6 {
		t.Fatalf("cursor after End = %d, want 6", field.Cursor)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if field.Cursor != 0 {
		t.Fatalf("cursor after Home = %d, want 0", field.Cursor)
	}
}

func TestRenameDialogFooterListsRestoreDefaultShortcut(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	keys := app.activeFooterKeys()
	if len(keys) != 5 {
		t.Fatalf("footer len = %d, want Esc + Sanitize + Slugify + Default + F10", len(keys))
	}
	if keys[1].Hint != "Sanitize" || keys[1].Key != tcell.KeyF2 {
		t.Fatalf("footer sanitize = %+v, want F2 Sanitize", keys[1])
	}
	if keys[2].Hint != "Slugify" || keys[2].Key != tcell.KeyF3 {
		t.Fatalf("footer slugify = %+v, want F3 Slugify", keys[2])
	}
	if keys[3].Hint != "Default" || keys[3].KeyLabel != "C-r" {
		t.Fatalf("footer restore = %+v, want C-r Default", keys[3])
	}
	if keys[4].Key != tcell.KeyF10 {
		t.Fatalf("last footer = %+v, want F10 Quit", keys[4])
	}

	// On OK button the restore hint is hidden (no prefill field focused).
	app.model.FileDialog.FocusedField = 1
	keys = app.activeFooterKeys()
	if len(keys) != 4 {
		t.Fatalf("on OK footer len = %d, want Esc + Sanitize + Slugify + F10", len(keys))
	}
}

func TestRenameDialogSanitizeF2ApplyTransformsName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.y_z"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseSanitize {
		t.Fatalf("phase = %v, want Sanitize", app.model.FileDialog.RenamePhase)
	}
	if got, want := app.model.FileDialog.FocusedField, dialog.FileDialogOKFocusIndex(app.model.FileDialog); got != want {
		t.Fatalf("sanitize open focus = %d, want OK (%d)", got, want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
		t.Fatalf("want back on main rename, got phase %v", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "x y z" {
		t.Fatalf("name = %q, want %q", got, "x y z")
	}
}

func TestRenameDialogSlugifyF3ApplyTransformsName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "my file"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseSlugify {
		t.Fatalf("phase = %v, want Slugify", app.model.FileDialog.RenamePhase)
	}
	if got, want := app.model.FileDialog.FocusedField, dialog.FileDialogOKFocusIndex(app.model.FileDialog); got != want {
		t.Fatalf("slugify open focus = %d, want OK (%d)", got, want)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
		t.Fatalf("want back on main rename, got phase %v", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "my.file" {
		t.Fatalf("name = %q, want %q", got, "my.file")
	}
}

func TestRenameDialogEncodingF4ApplyConvertsLegacyName(t *testing.T) {
	dir := t.TempDir()
	want := "日本語"
	sjis, err := japanese.ShiftJIS.NewEncoder().String(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if utf8.ValidString(sjis) {
		t.Fatal("want invalid UTF-8 test name")
	}
	if err := os.Mkdir(filepath.Join(dir, sjis), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	_ = app.activePanel().Refresh(app.activeViewportRows())
	if !app.activePanel().SelectVisibleEntry(sjis) {
		t.Fatal("SelectVisibleEntry(sjis) failed")
	}

	app.dispatch(keymap.ActionFileRename)
	if len(app.model.FileDialog.RenameEncodingCandidates) == 0 {
		t.Fatal("want encoding candidates")
	}
	keys := app.activeFooterKeys()
	foundEncoding := false
	for _, k := range keys {
		if k.Hint == "Encoding" && k.Key == tcell.KeyF4 {
			foundEncoding = true
			break
		}
	}
	if !foundEncoding {
		t.Fatalf("footer missing F4 Encoding: %+v", keys)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyF4, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseEncoding {
		t.Fatalf("phase = %v, want Encoding", app.model.FileDialog.RenamePhase)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
		t.Fatalf("want back on main rename, got phase %v", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}

func TestRenameDialogSanitizeEscReturnsWithoutApply(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.b"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != dialog.RenamePhaseMain {
		t.Fatalf("phase = %v, want Main", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "a.b" {
		t.Fatalf("name = %q, want unchanged a.b", got)
	}
}

func TestRenameDialogPrefillClearsOnType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]
	if field.Value != "existing.txt" {
		t.Fatalf("prefill value = %q, want existing.txt", field.Value)
	}

	// First printable should clear prefill.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if field.Value != "n" {
		t.Fatalf("after first printable: value = %q, want n", field.Value)
	}
	if field.PrefillPending {
		t.Fatal("PrefillPending should be false after typing")
	}
}

func TestMkdirScrollsToCreatedDirectory(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 25; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("%02d.dat", i)))
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.ScrollMode = panel.ScrollModeMinimal
	vr := app.activeViewportRows()
	p.Cursor = 20
	p.ScrollOffset = 15

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "zz-new" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Primary
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Secondary

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name != "zz-new" {
		name := ""
		if ok {
			name = entry.Name
		}
		t.Fatalf("cursor entry = %q ok=%v, want zz-new", name, ok)
	}
	row := p.Cursor - p.ScrollOffset
	if row < 0 || row >= vr {
		t.Fatalf("cursor row %d outside viewport [0,%d), scroll=%d cursor=%d", row, vr, p.ScrollOffset, p.Cursor)
	}
}

func TestMkdirDialogPrefillsFromCursorEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cursor-name.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	f := &app.model.FileDialog.Fields[0]
	if f.Value != "cursor-name.txt" || f.Prefill != "cursor-name.txt" {
		t.Fatalf("field = %+v, want Value and Prefill cursor-name.txt", f)
	}
	if !f.PrefillPending {
		t.Fatal("PrefillPending should be true with a non-empty suggestion")
	}
}

func TestRenameDialogPrefillBackspaceCommitsBeforeDelete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if field.PrefillPending {
		t.Fatal("PrefillPending should be false after backspace")
	}
	if field.Value != "existing.tx" {
		t.Fatalf("value = %q, want existing.tx", field.Value)
	}
}

func TestRenameDialogCtrlRRestoresClearedPrefill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if field.Value != "" || field.PrefillPending {
		t.Fatalf("after Ctrl+L: value=%q pending=%v, want empty/false", field.Value, field.PrefillPending)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone))
	if field.Value != "existing.txt" {
		t.Fatalf("after Ctrl+R: value = %q, want existing.txt", field.Value)
	}
	if field.Cursor != len([]rune("existing.txt")) {
		t.Fatalf("after Ctrl+R: cursor = %d, want %d", field.Cursor, len([]rune("existing.txt")))
	}
	if !field.PrefillPending {
		t.Fatal("after Ctrl+R: PrefillPending should be true")
	}
}

func TestRenameDialogCtrlDRestoresEditedPrefill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if field.Value == "existing.txt" {
		t.Fatalf("expected prefill to be replaced after typing 'x'; value=%q", field.Value)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	if field.Value != "existing.txt" {
		t.Fatalf("after Ctrl+D: value = %q, want existing.txt", field.Value)
	}
	if !field.PrefillPending {
		t.Fatal("after Ctrl+D: PrefillPending should be true")
	}
}

func TestFileDialogExecutesDelete(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	writeFile(t, filePath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	if !app.model.FileDialog.Open {
		t.Fatal("delete dialog should be open")
	}

	// Default focus is No; move to Yes then Enter confirms delete.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after confirm")
	}
	flushBackgroundJobs(t, app)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
	if app.model.Message == "" {
		t.Fatal("expected success message after delete")
	}
}

func TestKeybindingDispatcherOpensFileDialogs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Test F7 for mkdir.
	app.handleKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		t.Fatal("F7 should open mkdir dialog")
	}
	app.dialogCtrl.CloseFileDialog()

	// Test F8 for delete (default global binding).
	app.handleKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogDelete {
		t.Fatal("F8 should open delete dialog")
	}
	app.dialogCtrl.CloseFileDialog()
}

func TestEmptyPanelShowsErrorForFileOperations(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Empty dir - just . and possibly no files.
	// Make it truly empty by using panel with no entries (the only entry might be parent reference).
	// Actually panel doesn't show ".." in v1, so empty means empty.

	app.dispatch(keymap.ActionFileRename)
	if app.model.FileDialog.Open {
		t.Fatal("dialog should not open on empty panel")
	}
	if app.model.Message == "" {
		t.Fatal("expected error message for operation on empty panel")
	}
}

func TestFileDialogEnterOnDeleteWithNoItems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	// Esc should work.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Esc should close delete dialog")
	}
}

func TestDeleteDialogEnterDefaultCancelsWithoutDelete(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "keep.txt")
	writeFile(t, filePath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (No)", app.model.FileDialog.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Enter on No should close dialog")
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}

func TestDeleteDialogWarningPluralDirectories(t *testing.T) {
	dir := t.TempDir()
	d1 := filepath.Join(dir, "a")
	d2 := filepath.Join(dir, "b")
	if err := os.Mkdir(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d2, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{d1: true, d2: true}
	if err := p.Refresh(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}

	app.dispatch(keymap.ActionFileDelete)
	if got := app.model.FileDialog.DeleteSummary; !strings.HasPrefix(got, "0 files (0 B)") {
		t.Fatalf("DeleteSummary = %q, want prefix %q", got, "0 files (0 B)")
	}
	if app.model.FileDialog.DeleteWarning != "" {
		t.Fatalf("DeleteWarning = %q, want empty", app.model.FileDialog.DeleteWarning)
	}
	if len(app.model.FileDialog.DeleteEntries) != 2 || app.model.FileDialog.DeleteEntries[0].Name != "a" || app.model.FileDialog.DeleteEntries[1].Name != "b" {
		t.Fatalf("DeleteEntries = %v, want [a b]", app.model.FileDialog.DeleteEntries)
	}
	if app.dialogCtrl.DeleteDialogScanFP() == "" {
		t.Fatal("expected delete dialog scan fingerprint after opening")
	}
}

func TestDeleteDialogShowsContextPathWhenSelectionOffPanel(t *testing.T) {
	dir := t.TempDir()
	series := filepath.Join(dir, "Some Series")
	season := filepath.Join(series, "Season 01")
	other := filepath.Join(dir, "Other")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	if err := p.Load(other); err != nil {
		t.Fatal(err)
	}
	p.SelectedPaths = map[string]bool{season: true}
	p.SelectionsStripOrder = []string{season}

	app.dispatch(keymap.ActionFileDelete)
	if !app.model.FileDialog.Open {
		t.Fatal("delete dialog should be open")
	}
	want := filepath.Join("Some Series", "Season 01")
	if len(app.model.FileDialog.DeleteEntries) != 1 || app.model.FileDialog.DeleteEntries[0].Name != want {
		t.Fatalf("DeleteEntries = %#v, want Name %q", app.model.FileDialog.DeleteEntries, want)
	}
}

func TestDeleteDialogListScrollsWithPageDown(t *testing.T) {
	dir := t.TempDir()
	for i := range 25 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("selection-%02d.txt", i)))
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = make(map[string]bool)
	for i := range 25 {
		p.SelectedPaths[filepath.Join(dir, fmt.Sprintf("selection-%02d.txt", i))] = true
	}
	if err := p.Refresh(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}

	app.dispatch(keymap.ActionFileDelete)
	if app.model.FileDialog.DeleteListScroll != 0 {
		t.Fatalf("DeleteListScroll = %d, want 0 on open", app.model.FileDialog.DeleteListScroll)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if app.model.FileDialog.DeleteListScroll == 0 {
		t.Fatal("PgDn should advance DeleteListScroll when list is clipped")
	}
}

func TestMkdirDialogExtractFooterHiddenWithSingleSelection(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	for _, k := range app.activeFooterKeys() {
		if k.Hint == "Extract common name" {
			t.Fatalf("footer should not list Extract common name with one selection: %+v", k)
		}
	}
}

func TestMkdirDialogExtractFooterShownWithMultipleSelections(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	writeFile(t, a)
	writeFile(t, b)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{a: true, b: true}

	app.dispatch(keymap.ActionFileMkdir)
	var found bool
	var defaultIdx, extractIdx = -1, -1
	for i, k := range app.activeFooterKeys() {
		switch k.Hint {
		case "Extract common name":
			found = true
			extractIdx = i
			if k.Key != tcell.KeyF7 || k.KeyLabel != "F7" {
				t.Fatalf("extract footer = %+v, want F7 Extract common name", k)
			}
		case "Default":
			defaultIdx = i
		}
	}
	if !found {
		t.Fatal("footer should list F7 Extract common name with two or more selections")
	}
	if defaultIdx < 0 {
		t.Fatal("footer should list Default when mkdir dialog has a prefill")
	}
	if defaultIdx >= extractIdx {
		t.Fatalf("footer order = Default@%d before Extract@%d, want Default left of F7", defaultIdx, extractIdx)
	}
}

func TestMkdirDialogExtractCommonNameF7(t *testing.T) {
	dir := t.TempDir()
	names := []string{
		"[aaa] some common name - 01 - asdf asdf",
		"[bbb] some common name - 02 - asdf asdf",
		"[acc] some common name - 03 - asdf asdf",
	}
	selected := make(map[string]bool, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		writeFile(t, path)
		selected[path] = true
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	app.activePanel().SelectedPaths = selected

	app.dispatch(keymap.ActionFileMkdir)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if got := app.model.FileDialog.Fields[0].Value; got != "some common name" {
		t.Fatalf("directory name = %q, want %q", got, "some common name")
	}
	if !app.model.FileDialog.Fields[0].PrefillPending {
		t.Fatal("expected PrefillPending after extract")
	}
}

func TestMkdirDialogWithoutSelectionHidesActionRadios(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogMkdir {
		t.Fatal("F7 should open mkdir dialog")
	}
	if app.model.FileDialog.MkdirShowActions {
		t.Fatal("MkdirShowActions should be false without selections")
	}
	if got, want := dialog.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 3; got != want {
		t.Fatalf("focus count = %d, want %d (field + OK + Cancel)", got, want)
	}
}

func TestMkdirDialogWithSelectionShowsActionRadiosAndNav(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should open")
	}
	if !app.model.FileDialog.MkdirShowActions {
		t.Fatal("MkdirShowActions should be true with selections")
	}
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreate {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreate (default)", app.model.FileDialog.MkdirAction)
	}
	if got, want := dialog.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 6; got != want {
		t.Fatalf("focus count = %d, want %d (field + 3 radios + OK + Cancel)", got, want)
	}

	// Down navigates: field(0) -> radio0(1) -> radio1(2) -> radio2(3) -> OK(4)
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("Down from field: focus = %d, want 1 (first radio)", app.model.FileDialog.FocusedField)
	}
	// Space on first radio selects MkdirActionCreate (already default).
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreate {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreate after Space on first radio", app.model.FileDialog.MkdirAction)
	}
	// Move to second radio and select copy.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreateCopySelect {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreateCopySelect after Space on second radio", app.model.FileDialog.MkdirAction)
	}
	// Down past last radio reaches OK button.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 4 {
		t.Fatalf("Down to OK: focus = %d, want 4", app.model.FileDialog.FocusedField)
	}
	// Right moves OK -> Cancel.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 5 {
		t.Fatalf("Right OK->Cancel: focus = %d, want 5", app.model.FileDialog.FocusedField)
	}
}

func TestMkdirDialogAltShortcutsSelectActionFromInputField(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	if app.model.FileDialog.FocusedField != 0 {
		t.Fatalf("initial focus = %d, want 0 (directory name field)", app.model.FileDialog.FocusedField)
	}

	alt := tcell.ModAlt
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'y', alt))
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreateCopySelect {
		t.Fatalf("Alt+y: MkdirAction = %v, want copy", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 2 {
		t.Fatalf("Alt+y: focus = %d, want 2 (copy radio)", app.model.FileDialog.FocusedField)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'm', alt))
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreateMoveSelect {
		t.Fatalf("Alt+m: MkdirAction = %v, want move", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 3 {
		t.Fatalf("Alt+m: focus = %d, want 3 (move radio)", app.model.FileDialog.FocusedField)
	}

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', alt))
	if app.model.FileDialog.MkdirAction != dialog.MkdirActionCreate {
		t.Fatalf("Alt+r: MkdirAction = %v, want create", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("Alt+r: focus = %d, want 1 (create radio)", app.model.FileDialog.FocusedField)
	}

	// Alt+C cancels the dialog; it must not select the copy radio.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'c', alt))
	if app.model.FileDialog.Open {
		t.Fatal("dialog still open after Alt+c; want cancel")
	}
}

func TestMkdirDialogRadioRowsRejectTextInput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	// Type a fresh value then move down to a radio row.
	for _, r := range "newdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if !app.dialogCtrl.FileDialogOnRadio() {
		t.Fatalf("expected focus on a radio row, focus = %d", app.model.FileDialog.FocusedField)
	}
	// Typing must not alter the text field while on a radio row.
	beforeValue := app.model.FileDialog.Fields[0].Value
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	if app.model.FileDialog.Fields[0].Value != beforeValue {
		t.Fatalf("text field changed while on radio: %q -> %q", beforeValue, app.model.FileDialog.Fields[0].Value)
	}
	// Backspace also must not alter the text field on a radio row.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if app.model.FileDialog.Fields[0].Value != beforeValue {
		t.Fatalf("text field changed via Backspace on radio: %q -> %q", beforeValue, app.model.FileDialog.Fields[0].Value)
	}
}

func TestMkdirOpenInInactiveOpensOtherPanelAfterCreate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "keep-cursor.txt"))
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	left := app.panelByID(ui.PrimaryPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "keep-cursor.txt" {
			left.Cursor = i
			break
		}
	}

	app.model.ActivePanel = ui.PrimaryPanel
	app.dispatch(keymap.ActionFileMkdirOpenInOther)
	if !app.model.FileDialog.Open {
		t.Fatal("expected mkdir dialog open")
	}
	if !app.model.FileDialog.MkdirOpenInInactive {
		t.Fatal("MkdirOpenInInactive = false, want true for file.mkdir-open-in-other")
	}
	for _, r := range "otherdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Primary
	applyNextInterruptEvent(t, app, screen) // async reload, RefreshBothPanels: Secondary (superseded below)
	applyNextInterruptEvent(t, app, screen) // async load, Secondary opens otherdir

	wantOther := filepath.Join(dir, "otherdir")
	if got := filepath.Clean(app.panelByID(ui.SecondaryPanel).Path.String()); got != filepath.Clean(wantOther) {
		t.Fatalf("inactive panel path = %q want %q", got, wantOther)
	}
	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != filepath.Clean(dir) {
		t.Fatalf("active panel path = %q want %q", got, dir)
	}
	entry, ok := left.CurrentEntry()
	if !ok || entry.Name != "keep-cursor.txt" {
		t.Fatalf("active panel cursor = %q, want keep-cursor.txt", entry.Name)
	}
}

func TestMkdirActionCreateOnlyDoesNotQueueJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	// Type fresh name (first printable clears prefill).
	for _, r := range "newdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// MkdirActionCreate is the default; confirm without changing the radio.
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "newdir")); err != nil {
		t.Fatalf("expected new directory: %v", err)
	}
	if got := len(app.jobState.AllJobs()); got != 0 {
		t.Fatalf("expected 0 jobs after plain Create, got %d", got)
	}
	if !p.SelectedPaths[src] {
		t.Fatal("plain Create must preserve selection")
	}
}

func TestMkdirActionCreateAndCopyQueuesCopyJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "newdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Set MkdirActionCreateCopySelect via the model (focus-independent path).
	app.model.FileDialog.MkdirAction = dialog.MkdirActionCreateCopySelect
	app.dialogCtrl.ExecuteFileDialog()

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after execute")
	}
	created := filepath.Join(dir, "newdir")
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("expected new directory at %q (err=%v)", created, err)
	}
	jobsList := app.jobState.AllJobs()
	if len(jobsList) != 1 {
		t.Fatalf("expected 1 job after Create+Copy, got %d", len(jobsList))
	}
	j := jobsList[0]
	if j.Type != jobs.TypeCopy {
		t.Fatalf("job type = %v, want TypeCopy", j.Type)
	}
	if filepath.Clean(j.Destination.String()) != filepath.Clean(created) {
		t.Fatalf("job destination = %q, want %q", j.Destination, created)
	}
	if len(j.Sources) != 1 || filepath.Clean(j.Sources[0].String()) != filepath.Clean(src) {
		t.Fatalf("job sources = %v, want [%q]", j.Sources, src)
	}
	if len(p.SelectedPaths) != 0 {
		t.Fatalf("selection should be cleared after queueing copy job, got %d", len(p.SelectedPaths))
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
}

func TestMkdirActionCreateAndMoveQueuesMoveJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "newdir" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.model.FileDialog.MkdirAction = dialog.MkdirActionCreateMoveSelect
	app.dialogCtrl.ExecuteFileDialog()

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after execute")
	}
	created := filepath.Join(dir, "newdir")
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("expected new directory at %q (err=%v)", created, err)
	}
	jobsList := app.jobState.AllJobs()
	if len(jobsList) != 1 {
		t.Fatalf("expected 1 job after Create+Move, got %d", len(jobsList))
	}
	j := jobsList[0]
	if j.Type != jobs.TypeMove {
		t.Fatalf("job type = %v, want TypeMove", j.Type)
	}
	if filepath.Clean(j.Destination.String()) != filepath.Clean(created) {
		t.Fatalf("job destination = %q, want %q", j.Destination, created)
	}
	if len(p.SelectedPaths) != 0 {
		t.Fatalf("selection should be cleared after queueing move job, got %d", len(p.SelectedPaths))
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
}
