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
