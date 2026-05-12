package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestPathPickerQueryLooksPathlike(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		q    string
		want bool
	}{
		{"", false},
		{"  ", false},
		{"foo", false},
		{"foo bar", false},
		{"/abs", true},
		{"./here", true},
		{"../up", true},
		{".hidden", true},
		{"rel/sub", true},
		{"~", true},
		{"~/x", true},
	} {
		if got := pathPickerQueryLooksPathlike(tc.q); got != tc.want {
			t.Fatalf("pathPickerQueryLooksPathlike(%q) = %v want %v", tc.q, got, tc.want)
		}
	}
}

func TestResolvePathPickerQuery(t *testing.T) {
	t.Parallel()
	panel := "/tmp/panel"
	home := "/home/u"
	if got := resolvePathPickerQuery(panel, home, "~/doc"); got != filepath.Join(home, "doc") {
		t.Fatalf("resolve ~/ = %q want %q", got, filepath.Join(home, "doc"))
	}
	if got := resolvePathPickerQuery(panel, home, "~"); got != home {
		t.Fatalf("resolve ~ = %q want %q", got, home)
	}
	if got := resolvePathPickerQuery(panel, home, "/etc"); got != "/etc" {
		t.Fatalf("resolve abs = %q", got)
	}
	if got := resolvePathPickerQuery(panel, home, "sub"); got != filepath.Join(panel, "sub") {
		t.Fatalf("resolve rel = %q want %q", got, filepath.Join(panel, "sub"))
	}
}

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
	app.applyPathPickerPathValidation()
	if st.QueryPathInvalid {
		t.Fatal("plain fuzzy filter must not set invalid")
	}

	st.Query = filepath.Join(root, "nope", "missing")
	st.QueryPathCheckPending = false
	app.applyPathPickerPathValidation()
	if !st.QueryPathInvalid {
		t.Fatal("expected invalid for missing path-shaped query")
	}

	st.Query = real
	st.QueryPathCheckPending = false
	app.applyPathPickerPathValidation()
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
	app.armPathPickerValidateTimer()
	if app.pathPickerValidateTimer == nil {
		t.Fatal("expected timer armed")
	}
	app.closePathPicker()
	if app.pathPickerValidateTimer != nil {
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
		app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
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

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor-1 {
		t.Fatalf("Left arrow: QueryCursor = %d want %d", st.QueryCursor, wantCursor-1)
	}

	for i := 0; i < 200; i++ {
		app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	}
	if st.QueryCursor != 0 {
		t.Fatalf("after many Left: QueryCursor = %d want 0", st.QueryCursor)
	}
	if st.QueryScroll != 0 {
		t.Fatalf("after cursor home: QueryScroll = %d want 0", st.QueryScroll)
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlE, 0, tcell.ModCtrl))
	if st.QueryCursor != wantCursor {
		t.Fatalf("Ctrl+E: QueryCursor = %d want %d", st.QueryCursor, wantCursor)
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if st.QueryCursor != wantCursor-1 {
		t.Fatalf("Backspace: QueryCursor = %d want %d", st.QueryCursor, wantCursor-1)
	}
	if got := st.Query; got != longPath[:len(longPath)-1] {
		t.Fatalf("after Backspace Query = %q want %q", got, longPath[:len(longPath)-1])
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, tcell.ModCtrl))
	if st.QueryCursor != 0 || st.QueryScroll != 0 {
		t.Fatalf("Ctrl+A: cursor/scroll = (%d,%d) want (0,0)", st.QueryCursor, st.QueryScroll)
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone))
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

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if st.Query != "/foo/" || st.QueryCursor != 5 {
		t.Fatalf("after Ctrl+W: query=%q cursor=%d", st.Query, st.QueryCursor)
	}

	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if st.QueryCursor != 1 {
		t.Fatalf("after Alt+b: cursor=%d want 1", st.QueryCursor)
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
	before := app.pathPickerValidateGen.Load()
	app.armPathPickerValidateTimer()
	afterArm := app.pathPickerValidateGen.Load()
	if afterArm != before+1 {
		t.Fatalf("pathPickerValidateGen after arm = %d want %d", afterArm, before+1)
	}
	app.closePathPicker()
	if app.pathPickerValidateGen.Load() <= afterArm {
		t.Fatalf("pathPickerValidateGen after close should exceed post-arm value; got %d want > %d",
			app.pathPickerValidateGen.Load(), afterArm)
	}
}
