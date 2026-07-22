package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestQuickViewDirOverlayLoadsGitStatus is a regression test: populateQuickViewDirOverlay's
// fresh-snapshot path (preview.go) used to build the overlay via panel.State.Load without ever
// wiring ScheduleGitStatus, so GitColumnActive/GitPending were set but the async fetch never
// dispatched and GitByPath stayed nil forever (state.go prepareGitColumn no-ops when
// ScheduleGitStatus is nil).
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
	ov := &app.model.QuickViewDirOverlay
	if !ov.GitColumnActive {
		t.Fatal("overlay GitColumnActive = false, want true inside Git work tree")
	}
	if !ov.GitPending {
		t.Fatal("overlay GitPending = false, want true before async status completes")
	}

	// The primary panel's own cwd (root, also a Git work tree) schedules its own async git
	// status fetch alongside the overlay's, so drain interrupts until the overlay settles
	// instead of assuming the overlay's event arrives first.
	for i := 0; i < 10 && app.model.QuickViewDirOverlay.GitPending; i++ {
		applyNextInterruptEvent(t, app, screen)
	}

	ov = &app.model.QuickViewDirOverlay
	if ov.GitPending {
		t.Fatal("overlay GitPending still true after draining async git status events")
	}
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
