package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestQuickViewDirOverlayLoadsGitStatus checks that a Quick View directory overlay
// on a path neither panel currently lists schedules its own git-status fetch through
// ui.QuickViewOverlayPanel and paints the overlay's GitByPath, not a real panel's.
func TestQuickViewDirOverlayLoadsGitStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	runQuickViewGit(t, root, "init")
	runQuickViewGit(t, root, "config", "user.email", "t@example.com")
	runQuickViewGit(t, root, "config", "user.name", "test")

	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	tracked := filepath.Join(alpha, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	runQuickViewGit(t, root, "add", "alpha/tracked.txt")
	runQuickViewGit(t, root, "commit", "-m", "init")
	fresh := filepath.Join(alpha, "fresh.txt")
	if err := os.WriteFile(fresh, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.PrimaryPanel
	selectPanelEntryByName(t, app.panelByID(ui.PrimaryPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	if !app.model.QuickViewDirOverlayActive {
		t.Fatal("quick view dir overlay should be active")
	}
	// Overlay listing and git status are both async under ui.QuickViewOverlayPanel; the
	// primary panel's own cwd (also a Git work tree) schedules its own fetch alongside.
	drainInterruptEventsUntil(t, app, screen, 2*time.Second, func() bool {
		ov := app.model.QuickViewDirOverlay
		return ov.GitColumnActive && !ov.GitPending
	})

	ov := &app.model.QuickViewDirOverlay
	cell, ok := ov.GitByPath[fresh]
	if !ok {
		t.Fatalf("overlay GitByPath missing entry for %q, got %v", fresh, ov.GitByPath)
	}
	if cell.Unstaged != gitstatus.New {
		t.Fatalf("fresh.txt unstaged cell = %v, want New", cell.Unstaged)
	}
}

func runQuickViewGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
