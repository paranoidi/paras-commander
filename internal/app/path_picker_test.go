package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestApplyPathPickerPathValidation(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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
	st := &app.model.PathPicker

	st.Query = "fuzzyonly"
	st.QueryPathCheckPending = false
	app.dialogCtrl.ApplyPathPickerPathValidation()
	if st.QueryPathInvalid {
		t.Fatal("plain fuzzy filter must not set invalid")
	}

	st.Query = filepath.Join(root, "nope", "missing")
	st.QueryPathCheckPending = false
	app.dialogCtrl.ApplyPathPickerPathValidation()
	if !st.QueryPathInvalid {
		t.Fatal("expected invalid for missing path-shaped query")
	}

	st.Query = real
	st.QueryPathCheckPending = false
	app.dialogCtrl.ApplyPathPickerPathValidation()
	if st.QueryPathInvalid {
		t.Fatal("existing path must clear invalid")
	}
}

func TestPathPickerCloseStopsValidateTimer(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(80, 24)

	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	cfg.UI.PathPickerValidateDelayMS = 5000
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	app.dialogCtrl.ArmPathPickerValidateTimer()
	if !app.dialogCtrl.PathPickerValidateArmed() {
		t.Fatal("expected timer armed")
	}
	app.dialogCtrl.ClosePathPicker()
	if app.dialogCtrl.PathPickerValidateArmed() {
		t.Fatal("closePathPicker should stop validate timer")
	}
	if app.model.PathPicker.Open {
		t.Fatal("dialog should be closed")
	}
}

func TestPathPickerQueryInsertAdvancesCursorAndScroll(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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

	const longPath = "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/value"
	for _, r := range longPath {
		app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	st := &app.model.PathPicker
	if st.Query != longPath {
		t.Fatalf("Query mismatch: got %q want %q", st.Query, longPath)
	}
	wantCursor := len([]rune(longPath))
	if st.QueryCursor != wantCursor {
		t.Fatalf("QueryCursor = %d want %d", st.QueryCursor, wantCursor)
	}
	if st.QueryScroll == 0 {
		t.Fatalf("expected QueryScroll > 0 for long input, got 0")
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor-1 {
		t.Fatalf("Left arrow: QueryCursor = %d want %d", st.QueryCursor, wantCursor-1)
	}

	for i := 0; i < 200; i++ {
		app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if st.QueryCursor != 0 {
		t.Fatalf("after many Left: QueryCursor = %d want 0", st.QueryCursor)
	}
	if st.QueryScroll != 0 {
		t.Fatalf("after cursor home: QueryScroll = %d want 0", st.QueryScroll)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl))
	if st.QueryCursor != wantCursor {
		t.Fatalf("Ctrl+E: QueryCursor = %d want %d", st.QueryCursor, wantCursor)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor-1 {
		t.Fatalf("Backspace: QueryCursor = %d want %d", st.QueryCursor, wantCursor-1)
	}
	if got := st.Query; got != longPath[:len(longPath)-1] {
		t.Fatalf("after Backspace Query = %q want %q", got, longPath[:len(longPath)-1])
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if st.QueryCursor != 0 || st.QueryScroll != 0 {
		t.Fatalf("Ctrl+A: cursor/scroll = (%d,%d) want (0,0)", st.QueryCursor, st.QueryScroll)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
	if got := st.Query; got != longPath[1:len(longPath)-1] {
		t.Fatalf("after Delete Query = %q want %q", got, longPath[1:len(longPath)-1])
	}
	if st.QueryCursor != 0 {
		t.Fatalf("after Delete cursor = %d want 0", st.QueryCursor)
	}
}

func TestPathPickerQueryCtrlWAndAltBWordNav(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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
	st := &app.model.PathPicker
	st.Query = "/foo/bar"
	st.QueryCursor = len([]rune(st.Query))
	st.Focus = 0

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if st.Query != "/foo/" || st.QueryCursor != 5 {
		t.Fatalf("after Ctrl+W: query=%q cursor=%d", st.Query, st.QueryCursor)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if st.QueryCursor != 1 {
		t.Fatalf("after Alt+b: cursor=%d want 1", st.QueryCursor)
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if st.Query != "" || st.QueryCursor != 0 || st.QueryScroll != 0 {
		t.Fatalf("after Ctrl+L: query=%q cursor=%d scroll=%d", st.Query, st.QueryCursor, st.QueryScroll)
	}
}

func TestPathPickerTabAcceptsFilesystemCompletion(t *testing.T) {
	root := t.TempDir()
	fooDir := filepath.Join(root, "foo")
	if err := os.MkdirAll(fooDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+fooDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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
	st := &app.model.PathPicker

	prefix := filepath.Join(root, "f")
	for _, r := range prefix {
		app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if st.QueryCompletionSuffix != "oo" {
		t.Fatalf("suffix = %q want oo", st.QueryCompletionSuffix)
	}
	if !st.QueryCompletionIsDir {
		t.Fatal("expected directory completion")
	}

	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	want := filepath.Join(root, "foo") + "/"
	if st.Query != want {
		t.Fatalf("Query = %q want %q", st.Query, want)
	}
	if st.QueryCursor != len([]rune(want)) {
		t.Fatalf("QueryCursor = %d want %d", st.QueryCursor, len([]rune(want)))
	}
	if st.QueryCompletionSuffix != "" {
		t.Fatalf("suffix should be cleared after accept, got %q", st.QueryCompletionSuffix)
	}
}

func TestPathPickerBackspaceRevealScrollOnLastVisible(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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
	st := &app.model.PathPicker

	query := "/very/long/path/with/many/segments/that/exceeds/the/visible/picker/input/width/~/projects/paras-commander/"
	for _, r := range query {
		app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.dialogCtrl.SyncPathPickerScroll()
	width := app.dialogCtrl.PathPickerQueryWidth()
	runes := []rune(st.Query)
	if len(runes) <= width {
		t.Fatalf("test path must exceed input width %d", width)
	}
	if st.QueryScroll == 0 {
		t.Fatal("expected scrolled query before backspace")
	}
	scrollBefore := st.QueryScroll
	if st.QueryCursor != len(runes) {
		t.Fatalf("cursor = %d want %d", st.QueryCursor, len(runes))
	}
	app.dialogCtrl.HandlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if st.QueryScroll >= scrollBefore {
		t.Fatalf("QueryScroll = %d want < %d after backspace on last visible char", st.QueryScroll, scrollBefore)
	}
	wantQuery := string(runes[:len(runes)-1])
	if st.Query != wantQuery {
		t.Fatalf("Query = %q want %q", st.Query, wantQuery)
	}
}

func TestPathPickerValidateArmIncrementsGeneration(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "exists")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("m : "+real+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(screen.Fini)
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
	before := app.dialogCtrl.PathPickerValidateGeneration()
	app.dialogCtrl.ArmPathPickerValidateTimer()
	afterArm := app.dialogCtrl.PathPickerValidateGeneration()
	if afterArm != before+1 {
		t.Fatalf("pathPickerValidate generation after arm = %d want %d", afterArm, before+1)
	}
	app.dialogCtrl.ClosePathPicker()
	if app.dialogCtrl.PathPickerValidateGeneration() <= afterArm {
		t.Fatalf("pathPickerValidate generation after close should exceed post-arm value; got %d want > %d",
			app.dialogCtrl.PathPickerValidateGeneration(), afterArm)
	}
}
