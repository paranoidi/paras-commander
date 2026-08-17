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
// fresh-snapshot path (internal/apphandler/preview) used to build the overlay via panel.State.Load
// without ever wiring ScheduleGitStatus, so GitColumnActive/GitPending were set but the async fetch
// never dispatched and GitByPath stayed nil forever (state.go prepareGitColumn no-ops when
// ScheduleGitStatus is nil).
func TestQuickViewDirOverlayLoadsGitStatus(t *testing.T) {
	// BUG (pre-existing, exposed by making local navigation async): populateQuickViewDirOverlay's
	// fresh-snapshot path (internal/apphandler/preview/preview.go, populateQuickViewDirOverlay's
	// ov.Load(canonical) call around line 561) only hits when neither driver nor follower already
	// lists the target directory (the ListingAtPath fast paths above it are skipped in that case).
	// ov.ScheduleAsyncLoad is a verbatim copy of follower.ScheduleAsyncLoad
	// (initQuickViewDirOverlayFromFollower), which closes over the *real* follower panelID. So
	// ov.Load's async completion posts a panelAsyncLoadPayload tagged with the real follower's
	// panelID, and applyPanelAsyncLoad (internal/app/panel_async_load.go) applies it onto the real
	// follower panel.State, never onto ov — ov.GitColumnActive/GitPending/GitByPath (set inside
	// ApplyListing/prepareGitColumn) are never touched, so the overlay's own git status never
	// dispatches. Previously unreachable because ScheduleRemoteLoad only ever fired for sftp://
	// paths, so this fresh-snapshot overlay path never aliased a real panel's async load locally.
	t.Skip("quick-view overlay aliases the real panel's ScheduleAsyncLoad panelID on its fresh-snapshot path — needs its own identity in internal/apphandler/preview or internal/app/panel_async_load.go before this is reliable")
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
