package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/keymap"
)

type multiOpenFixture struct {
	app      *App
	launches [][]string
}

// setupMultiOpen selects the named files, parks the cursor on the first, and stubs the open seams.
func setupMultiOpen(t *testing.T, appFor func(name string) string, names ...string) *multiOpenFixture {
	t.Helper()
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := newApp(t, screen, root)
	app.config.Panels.OpenFilesExternally = true
	app.config.Panels.RunExecutablesOnEnter = false
	f := &multiOpenFixture{app: app}
	oldApp, oldLaunch := defaultAppForPath, runDetachedLaunch
	defaultAppForPath = func(p string) (string, error) { return appFor(filepath.Base(p)), nil }
	runDetachedLaunch = func(id string, paths []string) error {
		f.launches = append(f.launches, append([]string{id}, paths...))
		return nil
	}
	t.Cleanup(func() { defaultAppForPath, runDetachedLaunch = oldApp, oldLaunch })
	for _, n := range names {
		app.activePanel().AddSelection(filepath.Join(root, n))
	}
	selectEntryByName(t, app, names[0])
	app.dispatch(keymap.ActionNavOpen)
	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return app.model.Message != "" })
	return f
}

func TestNavOpenSelectionLaunchesSharedApplicationOnce(t *testing.T) {
	f := setupMultiOpen(t, func(string) string { return "mesh.desktop" }, "copper-anvil.stl", "velvet-orchid.stl")
	if len(f.launches) != 1 || f.launches[0][0] != "mesh.desktop" || len(f.launches[0]) != 3 {
		t.Fatalf("launches = %v, want one mesh.desktop launch with both files", f.launches)
	}
	if !strings.Contains(f.app.model.Message, "Opened 2 files") {
		t.Fatalf("message = %q", f.app.model.Message)
	}
}

func TestNavOpenSelectionWithMixedApplicationsLaunchesNothing(t *testing.T) {
	f := setupMultiOpen(t, func(name string) string {
		if strings.HasSuffix(name, ".stl") {
			return "mesh.desktop"
		}
		return "viewer.desktop"
	}, "copper-anvil.stl", "velvet-orchid.jpg")
	if len(f.launches) != 0 {
		t.Fatalf("launches = %v, want none", f.launches)
	}
	if !strings.Contains(f.app.model.Message, "different applications") {
		t.Fatalf("message = %q", f.app.model.Message)
	}
}

func TestLaunchSharedAppErrorListsApplicationsSorted(t *testing.T) {
	old := defaultAppForPath
	defaultAppForPath = func(p string) (string, error) {
		return map[string]string{"a": "b.desktop", "b": "a.desktop", "c": "b.desktop"}[p], nil
	}
	t.Cleanup(func() { defaultAppForPath = old })
	err := launchSharedApp([]string{"a", "b", "c"})
	want := "selected files open with different applications: a.desktop (1), b.desktop (2)"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}
