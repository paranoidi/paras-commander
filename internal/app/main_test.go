package app

import (
	"context"
	"os"
	"testing"
)

// TestMain neuters every subprocess launcher that would otherwise pop a real editor, terminal
// program, or desktop application on the developer's machine. Tests that need to observe a
// launch install their own stub on top of these (and restore them in t.Cleanup).
func TestMain(m *testing.M) {
	externalEditorRunner = func(context.Context, string, bool) error { return nil }
	userMenuInteractiveRunner = func(context.Context, []string, string) error { return nil }
	userMenuDetachRunner = func([]string, string) error { return nil }
	runDetachedXDGOpen = func(string) error { return nil }
	defaultAppForPath = func(string) (string, error) { return "stub.desktop", nil }
	runDetachedLaunch = func(string, []string) error { return nil }
	// Keep dedup's persistent hash cache, the user's zoxide database and their fileindex database
	// (found through $HOME when a test's config names none) out of tests.
	dir, err := os.MkdirTemp("", "pc-app-test-cache")
	if err == nil {
		_ = os.Setenv("XDG_CACHE_HOME", dir)
		_ = os.Setenv("_ZO_DATA_DIR", dir)
		_ = os.Setenv("HOME", dir)
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}
