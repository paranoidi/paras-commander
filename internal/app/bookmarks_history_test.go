package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestBookmarkDialogOpensAndNavigates(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "deep", "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("markone : %s\n", target)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if !app.model.PathPicker.Open {
		t.Fatal("expected path picker open")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.PathPicker.Open {
		t.Fatal("expected dialog closed")
	}
	if got := app.activePanel().Path.String(); got != filepath.Clean(target) {
		t.Fatalf("panel path = %q want %q", got, filepath.Clean(target))
	}
}

func TestBookmarkDialogF8DeletesFZFMark(t *testing.T) {
	root := t.TempDir()
	xdg := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(xdg, "gtk-3.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	target := filepath.Join(root, "marked")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("markone : %s\n", target)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD:          func() (string, error) { return root, nil },
		Config:       cfg,
		KeymapBundle: bundle,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if len(app.model.PathPicker.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(app.model.PathPicker.Items))
	}
	if !app.bookmarkDialogDeleteFooterEligible() {
		t.Fatal("expected delete footer for fzf-marks row")
	}
	if !app.tryBookmarkDialogShortcut(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)) {
		t.Fatal("F8 should delete selected fzf-marks bookmark")
	}
	if len(app.model.PathPicker.Items) != 0 {
		t.Fatalf("expected empty picker items, got %d", len(app.model.PathPicker.Items))
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("marks file = %q, want empty", string(data))
	}
	if !strings.Contains(app.model.Message, "Bookmark removed") {
		t.Fatalf("message = %q, want removal confirmation", app.model.Message)
	}
}

func TestBookmarkDialogDeleteFooterSkippedForGnomeMark(t *testing.T) {
	root := t.TempDir()
	xdg := filepath.Join(root, "xdg")
	gtkDir := filepath.Join(xdg, "gtk-3.0")
	if err := os.MkdirAll(gtkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gnomePath := filepath.Join(root, "gnome-only")
	if err := os.MkdirAll(gnomePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	gtkMarks := filepath.Join(gtkDir, "bookmarks")
	if err := os.WriteFile(gtkMarks, []byte("file://"+gnomePath+" gnomeproj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD:          func() (string, error) { return root, nil },
		Config:       cfg,
		KeymapBundle: bundle,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if len(app.model.PathPicker.Items) != 1 {
		t.Fatalf("items = %d, want 1 gnome bookmark", len(app.model.PathPicker.Items))
	}
	if app.model.PathPicker.Items[0].Source != "gnome" {
		t.Fatalf("source = %q, want gnome", app.model.PathPicker.Items[0].Source)
	}
	if app.bookmarkDialogDeleteFooterEligible() {
		t.Fatal("delete footer should not show for gnome bookmark")
	}
	before, err := os.ReadFile(gtkMarks)
	if err != nil {
		t.Fatal(err)
	}
	if app.tryBookmarkDialogShortcut(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)) {
		t.Fatal("F8 should not delete gnome bookmark")
	}
	after, err := os.ReadFile(gtkMarks)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("gtk bookmarks changed: before %q after %q", before, after)
	}
}

func TestHistoryDialogAltHUsesActivePanel(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.model.ActivePanel = ui.SecondaryPanel
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	if app.model.HistoryDialog.PanelID != ui.SecondaryPanel {
		t.Fatalf("History panel = %d want right (%d)", app.model.HistoryDialog.PanelID, ui.SecondaryPanel)
	}
}

func TestHistoryDialogFilterNavigatesToMatch(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p := app.panelByID(ui.PrimaryPanel)
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			p.Cursor = i
			break
		}
	}
	app.dispatch(keymap.ActionNavOpen)
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "beta" {
			p.Cursor = i
			break
		}
	}
	app.dispatch(keymap.ActionNavOpen)

	app.openHistoryDialog(ui.PrimaryPanel)
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	for _, r := range "alpha" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("unexpected quit")
		}
	}
	if len(app.model.HistoryDialog.Ranked) == 0 {
		t.Fatal("expected fuzzy matches for alpha")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.HistoryDialog.Open {
		t.Fatal("expected dialog closed")
	}
	want := filepath.Clean(alpha)
	if got := filepath.Clean(app.activePanel().Path.String()); got != want {
		t.Fatalf("panel path = %q want %q", got, want)
	}
}

func TestHistoryDialogF5TogglesBothPanels(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	gamma := filepath.Join(root, "gamma")
	for _, p := range []string{alpha, beta, gamma} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	navigatePanelIntoDir(t, app, ui.PrimaryPanel, "alpha")
	app.model.ActivePanel = ui.SecondaryPanel
	navigatePanelIntoDir(t, app, ui.SecondaryPanel, "beta")

	app.openHistoryDialog(ui.PrimaryPanel)
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	singleCount := len(app.model.HistoryDialog.Paths)
	if singleCount == 0 {
		t.Fatal("expected left panel history")
	}
	if app.model.HistoryDialog.BothPanels {
		t.Fatal("expected single-panel mode on open")
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.HistoryDialog.BothPanels {
		t.Fatal("expected both-panels mode after F5")
	}
	if len(app.model.HistoryDialog.Paths) <= singleCount {
		t.Fatalf("merged paths = %d want > single %d", len(app.model.HistoryDialog.Paths), singleCount)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.HistoryDialog.BothPanels {
		t.Fatal("expected single-panel mode after second F5")
	}
	if len(app.model.HistoryDialog.Paths) != singleCount {
		t.Fatalf("paths = %d want restored single count %d", len(app.model.HistoryDialog.Paths), singleCount)
	}
}

func TestHistoryDialogBothPanelsOKNavigatesPanelID(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	navigatePanelIntoDir(t, app, ui.PrimaryPanel, "alpha")
	app.model.ActivePanel = ui.SecondaryPanel
	navigatePanelIntoDir(t, app, ui.SecondaryPanel, "beta")

	app.openHistoryDialog(ui.PrimaryPanel)
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	wantBeta := filepath.Clean(beta)
	for i, p := range app.model.HistoryDialog.Paths {
		if filepath.Clean(p) == wantBeta {
			for _, r := range app.model.HistoryDialog.Ranked {
				if r == i {
					app.model.HistoryDialog.Selected = 0
					for si, idx := range app.model.HistoryDialog.Ranked {
						if idx == i {
							app.model.HistoryDialog.Selected = si
							break
						}
					}
					break
				}
			}
			break
		}
	}
	app.activateHistorySelection()
	if got := filepath.Clean(app.panelByID(ui.PrimaryPanel).Path.String()); got != wantBeta {
		t.Fatalf("left panel path = %q want %q", got, wantBeta)
	}
}

func TestHistoryDialogBothEmptyInfoNoDialog(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.panelByID(ui.PrimaryPanel).History = nil
	app.panelByID(ui.SecondaryPanel).History = nil

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.HistoryDialog.Open {
		t.Fatal("expected dialog closed")
	}
	if app.model.Message != "No directory history yet" {
		t.Fatalf("message = %q", app.model.Message)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("urgency = %v want info", app.model.MessageUrgency)
	}
}

func TestHistoryDialogFooterF5Hints(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	navigatePanelIntoDir(t, app, ui.PrimaryPanel, "alpha")

	app.openHistoryDialog(ui.PrimaryPanel)
	keys := app.activeFooterKeys()
	var foundBoth bool
	for _, fk := range keys {
		if fk.KeyLabel == "F5" && fk.Hint == "Both Panels" {
			foundBoth = true
			break
		}
	}
	if !foundBoth {
		t.Fatalf("footer keys missing F5 Both Panels: %+v", keys)
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	keys = app.activeFooterKeys()
	var foundThis bool
	for _, fk := range keys {
		if fk.KeyLabel == "F5" && fk.Hint == "Active panel" {
			foundThis = true
			break
		}
	}
	if !foundThis {
		t.Fatalf("footer keys missing F5 Active panel: %+v", keys)
	}
}

func TestBookmarkDialogFilterSelectsRankedFirst(t *testing.T) {
	root := t.TempDir()
	tAlpha := filepath.Join(root, "alpha")
	tBeta := filepath.Join(root, "beta")
	if err := os.MkdirAll(tAlpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tBeta, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	content := fmt.Sprintf("aaa : %s\nbbb : %s\n", tAlpha, tBeta)
	if err := os.WriteFile(marksPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	for _, r := range "b" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("unexpected quit")
		}
	}
	if len(app.model.PathPicker.Ranked) == 0 {
		t.Fatal("expected at least one fuzzy match for query b")
	}
	if app.model.PathPicker.Ranked[0] != 1 {
		t.Fatalf("first ranked index = %d want 1 (bbb line), ranked=%v", app.model.PathPicker.Ranked[0], app.model.PathPicker.Ranked)
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().Path.String(); got != filepath.Clean(tBeta) {
		t.Fatalf("panel path = %q want %q", got, filepath.Clean(tBeta))
	}
}

func TestBookmarkDialogTypingODoesNotActivateWithoutEnter(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "t")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("x : "+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	startPath := app.activePanel().Path
	app.openBookmarkDialog()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.PathPicker.Open {
		t.Fatal("typing o in filter must not close or navigate")
	}
	if app.model.PathPicker.Query != "o" {
		t.Fatalf("query = %q want o", app.model.PathPicker.Query)
	}
	if app.activePanel().Path != startPath {
		t.Fatalf("panel path changed without Enter: %q", app.activePanel().Path.String())
	}
}

func TestAddBookmarkDialogOpenPrefillsBasename(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionBookmarkAdd)

	if !app.model.FileDialog.Open {
		t.Fatal("FileDialog should be open after ActionBookmarkAdd")
	}
	if app.model.FileDialog.DialogType != dialog.FileDialogAddBookmark {
		t.Fatalf("dialog type = %d, want FileDialogAddBookmark", app.model.FileDialog.DialogType)
	}
	if got, want := len(app.model.FileDialog.Fields), 1; got != want {
		t.Fatalf("Fields length = %d, want %d", got, want)
	}
	wantName := filepath.Base(dir)
	if app.model.FileDialog.Fields[0].Value != wantName {
		t.Fatalf("Fields[0].Value = %q, want %q", app.model.FileDialog.Fields[0].Value, wantName)
	}
	if !app.model.FileDialog.Fields[0].PrefillPending {
		t.Fatal("Fields[0].PrefillPending should be true on open")
	}
	if app.model.FileDialog.Message != dir {
		t.Fatalf("Message = %q, want %q", app.model.FileDialog.Message, dir)
	}
}

func TestAddBookmarkExecuteAppendsToMarksFile(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	// Replace prefilled name by typing a fresh value (first printable clears prefill).
	for _, r := range "myproject" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", marksPath, err)
	}
	wantLine := "myproject : " + dir + "\n"
	if string(data) != wantLine {
		t.Fatalf("marks file contents = %q, want %q", string(data), wantLine)
	}
	if !strings.Contains(app.model.Message, "Bookmark added") {
		t.Fatalf("transient message = %q, want it to mention bookmark added", app.model.Message)
	}
	var logText strings.Builder
	for _, e := range app.model.MessageLog {
		logText.WriteString(e.Text)
	}
	if !strings.Contains(logText.String(), marksPath) {
		t.Fatalf("message log should include marks file path %q; log=%#v", marksPath, app.model.MessageLog)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("message urgency = %v, want info", app.model.MessageUrgency)
	}
}

func TestAddBookmarkConfirmFromOKButtonWritesFile(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	for _, r := range "okbtn" {
		app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Move focus from name field to OK, then confirm (Enter must still append).
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (OK)", app.model.FileDialog.FocusedField)
	}
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter on OK")
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", marksPath, err)
	}
	wantLine := "okbtn : " + dir + "\n"
	if string(data) != wantLine {
		t.Fatalf("marks file contents = %q, want %q", string(data), wantLine)
	}
}

func TestAddBookmarkEmptyNameClosesWithError(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	// Wipe the prefilled value so name is empty.
	app.model.FileDialog.Fields[0].Value = ""
	app.model.FileDialog.Fields[0].Cursor = 0
	app.model.FileDialog.Fields[0].PrefillPending = false

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after rejected confirm")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyError {
		t.Fatalf("message urgency = %v, want error", app.model.MessageUrgency)
	}
	if _, err := os.Stat(marksPath); !os.IsNotExist(err) {
		t.Fatalf("marks file should not exist; stat err = %v", err)
	}
}

func TestAddBookmarkDefaultName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "root"},
		{path: "/home/user/projects", want: "projects"},
		{path: ".", want: "root"},
		{path: "", want: "root"},
	}
	for _, tt := range tests {
		if got := defaultBookmarkName(tt.path); got != tt.want {
			t.Fatalf("defaultBookmarkName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
