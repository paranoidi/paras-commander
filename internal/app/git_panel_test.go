package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func swapGitStatusForListing(t *testing.T, fn func(*gitstatus.Cache, context.Context, string, string, []gitstatus.ListingPaths) (map[string]gitstatus.Cell, error)) {
	t.Helper()
	orig := gitStatusForListing
	gitStatusForListing = fn
	t.Cleanup(func() { gitStatusForListing = orig })
}

// TestIsWithinDir covers the guard applyGitStatusLoad uses to accept git-status results for a
// tree-mode expanded child directory (a descendant of the panel's current listing) while
// rejecting results left over from navigating to an unrelated directory.
func TestIsWithinDir(t *testing.T) {
	cases := []struct {
		child, parent string
		want          bool
	}{
		{"/repo/src/pkg", "/repo/src", true},
		{"/repo/src", "/repo/src", true},
		{"/repo/other", "/repo/src", false},
		{"/repo/src2", "/repo/src", false}, // sibling with parent as string-prefix, not path-prefix
		{"/repo", "/repo/src", false},
		{"/repo/src/pkg", "", false},
	}
	for _, c := range cases {
		if got := isWithinDir(c.child, c.parent); got != c.want {
			t.Errorf("isWithinDir(%q, %q) = %v, want %v", c.child, c.parent, got, c.want)
		}
	}
}

// TestGitStatusLoadSurvivesSaturatedEventQueue is the characterizing test for dropped cwd git
// completions: with tcell's queue full, the fetch still applies exactly once and GitPending clears.
func TestGitStatusLoadSurvivesSaturatedEventQueue(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	app := newApp(t, screen, root)
	drainScreenInterrupts(app, screen)

	marker := filepath.Join(root, "willow.txt")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := map[string]gitstatus.Cell{marker: {Unstaged: gitstatus.Modified}}

	block := make(chan struct{})
	started := make(chan struct{})
	var startOnce sync.Once
	var applies int
	swapGitStatusForListing(t, func(c *gitstatus.Cache, ctx context.Context, workRoot, listDir string, paths []gitstatus.ListingPaths) (map[string]gitstatus.Cell, error) {
		startOnce.Do(func() { close(started) })
		<-block
		applies++
		return want, nil
	})

	pan := app.panelByID(ui.PrimaryPanel)
	pan.GitPending = true
	schedule := app.gitStatusScheduler(ui.PrimaryPanel)
	if !schedule(panel.GitStatusRequest{WorkRoot: root, ListDir: pan.PathString(), Paths: []gitstatus.ListingPaths{{AbsPath: marker}}}) {
		t.Fatal("git status should be scheduled")
	}
	<-started
	if !pan.GitPending {
		t.Fatal("GitPending should stay true while the fetch is held")
	}

	app.screen = dropPostEventScreen{SimulationScreen: screen}
	saturateSimulationEventQueue(t, screen)
	close(block)

	drainInterruptEventsUntil(t, app, screen, 3*time.Second, func() bool { return !pan.GitPending })
	if pan.GitPending {
		t.Fatal("GitPending should clear once even when the tcell queue was full")
	}
	if pan.GitByPath[marker].Unstaged != gitstatus.Modified {
		t.Fatalf("GitByPath[%q] = %+v, want unstaged Modified", marker, pan.GitByPath[marker])
	}
	if applies != 1 {
		t.Fatalf("git fetch ran %d times, want 1", applies)
	}

	drainScreenInterrupts(app, screen)
	if applies != 1 {
		t.Fatalf("second drain re-ran the git fetch (%d)", applies)
	}
	if pan.GitPending {
		t.Fatal("GitPending should stay false after the single apply")
	}
}

// TestTreeChildGitStatusRejectsStaleListingEpoch holds a pre-refresh child result, applies a
// newer cwd status after the listing epoch moves, then releases the old child and asserts it
// cannot merge over the newer cells. A same-session sibling child still merges.
func TestTreeChildGitStatusRejectsStaleListingEpoch(t *testing.T) {
	screen := newScreen(t, 80, 24)
	root := t.TempDir()
	child := filepath.Join(root, "meadow")
	leaf := filepath.Join(child, "harbor.txt")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(leaf, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := newApp(t, screen, root)
	pan := app.panelByID(ui.PrimaryPanel)
	pan.GitColumnActive = true
	oldEpoch := pan.ListingEpoch

	pan.ListingEpoch++
	pan.GitByPath = map[string]gitstatus.Cell{
		leaf: {Unstaged: gitstatus.Modified},
	}

	if app.applyGitStatusLoad(gitStatusPayload{
		panelID:      ui.PrimaryPanel,
		cwdLevel:     false,
		listDir:      child,
		sessionEpoch: oldEpoch,
		byPath:       map[string]gitstatus.Cell{leaf: {Unstaged: gitstatus.New}},
	}) {
		t.Fatal("stale child git result should be rejected")
	}
	if pan.GitByPath[leaf].Unstaged != gitstatus.Modified {
		t.Fatalf("stale child merged over newer cwd cell: %+v", pan.GitByPath[leaf])
	}

	sibling := filepath.Join(root, "willow")
	sibFile := filepath.Join(sibling, "amber.txt")
	if !app.applyGitStatusLoad(gitStatusPayload{
		panelID:      ui.PrimaryPanel,
		cwdLevel:     false,
		listDir:      sibling,
		sessionEpoch: pan.ListingEpoch,
		byPath:       map[string]gitstatus.Cell{sibFile: {Unstaged: gitstatus.Modified}},
	}) {
		t.Fatal("same-session child git result should merge")
	}
	if pan.GitByPath[sibFile].Unstaged != gitstatus.Modified {
		t.Fatalf("same-session sibling was not merged: %+v", pan.GitByPath[sibFile])
	}
	if pan.GitByPath[leaf].Unstaged != gitstatus.Modified {
		t.Fatalf("same-session merge overwrote the newer cwd cell: %+v", pan.GitByPath[leaf])
	}
}
