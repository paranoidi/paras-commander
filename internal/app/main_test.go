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
	externalEditorRunner = func(context.Context, string) error { return nil }
	userMenuInteractiveRunner = func(context.Context, []string, string) error { return nil }
	userMenuDetachRunner = func([]string, string) error { return nil }
	runDetachedXDGOpen = func(string) error { return nil }
	// Keep dedup's persistent hash cache out of the real user cache dir.
	dir, err := os.MkdirTemp("", "pc-app-test-cache")
	if err == nil {
		_ = os.Setenv("XDG_CACHE_HOME", dir)
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(dir)
	}
	os.Exit(code)
}
