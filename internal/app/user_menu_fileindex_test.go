package app

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// indexScanSettle is how long a no-scan test keeps pumping events, past the wake that would have
// queued the scan.
const indexScanSettle = 150 * time.Millisecond

// indexScanApp is fileIndexApp with body as its global menu.toml; the other panel sits in a second
// directory, returned as dest.
func indexScanApp(t *testing.T, body string) (a *App, screen tcell.SimulationScreen, dir, dest string) {
	t.Helper()
	cfgDir := t.TempDir()
	writeUserMenuFile(t, filepath.Join(cfgDir, config.DefaultUserMenuFileName), body)
	a, screen, dir, _ = fileIndexAppWith(t, func(o *Options) {
		o.Config.UserMenu = config.UserMenuConfig{LocalNames: []string{"menu.toml"}}
		o.Paths = config.Paths{ConfigDir: cfgDir}
	})
	dest, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	moveOtherPanel(t, a, screen, dest)
	return a, screen, dir, dest
}

// moveOtherPanel loads dir in the other panel and waits for it to land (Load is async).
func moveOtherPanel(t *testing.T, a *App, screen tcell.SimulationScreen, dir string) {
	t.Helper()
	if err := a.inactivePanel().Load(dir); err != nil {
		t.Fatal(err)
	}
	drainInterruptEventsUntil(t, a, screen, 3*time.Second, func() bool {
		return a.inactivePanel().PathString() == dir
	})
}

func runMenuKey(a *App, key rune) {
	a.openUserMenu()
	a.handleLeaderMenuKey(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone))
}

// scanSources lists the source of every queued fileindex scan job.
func scanSources(a *App) []string {
	var out []string
	for _, j := range a.jobState.AllJobs() {
		if j.Task != nil && j.Task.Title == "fileindex scan" {
			for _, src := range j.Sources {
				out = append(out, src.String())
			}
		}
	}
	return out
}

func waitScanSources(t *testing.T, a *App, screen tcell.SimulationScreen, want ...string) {
	t.Helper()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool { return len(scanSources(a)) >= len(want) })
	if got := scanSources(a); !slices.Equal(got, want) {
		t.Fatalf("scan sources = %v, want %v", got, want)
	}
}

// waitLastJobFinished applies job events (PollEvents, which is also what shows a job's toasts)
// until the newest job has finished.
func waitLastJobFinished(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		a.jobsCtrl.PollEvents()
		j := a.jobState.AllJobs()
		return len(j) > 0 && j[len(j)-1].Status.IsFinished()
	})
}

func assertNoScan(t *testing.T, a *App, screen tcell.SimulationScreen) {
	t.Helper()
	deadline := time.Now().Add(indexScanSettle)
	drainInterruptEventsUntil(t, a, screen, 2*indexScanSettle+time.Second, func() bool { return time.Now().After(deadline) })
	if got := scanSources(a); len(got) != 0 {
		t.Fatalf("scan sources = %v, want none", got)
	}
}

func TestUserMenuFileIndexScanRunsAfterCommand(t *testing.T) {
	for _, tc := range []struct{ name, mode, command, macro string }{
		{"background %D", "background = true", "true", "%D"},
		{"background %d", "background = true", "true", "%d"},
		{"background non-zero exit", "background = true", "false", "%D"},
		{"foreground", "", "true", "%D"},
		{"foreground non-zero exit", "", "false", "%D"},
		{"dialog", "dialog = true", "true", "%D"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, screen, dir, dest := indexScanApp(t, `[scan]
key = "s"
title = "Scan"
command = "`+tc.command+`"
fileindex_scan = "`+tc.macro+`"
`+tc.mode+"\n")
			runMenuKey(a, 's')
			want := dest
			if tc.macro == "%d" {
				want = dir
			}
			waitScanSources(t, a, screen, want)
		})
	}
}

func TestUserMenuFileIndexScanOnlyWhenAsked(t *testing.T) {
	a, screen, _, _ := indexScanApp(t, `[plain]
key = "p"
title = "Plain"
command = "true"
background = true
`)
	runMenuKey(a, 'p')
	waitCommandsDone(t, a)
	assertNoScan(t, a, screen)
}

func TestUserMenuFileIndexScanSkipsLaunchFailure(t *testing.T) {
	a, screen, _, _ := indexScanApp(t, `[broken]
key = "b"
title = "Broken"
command = "/nonexistent/lantern-orchard"
background = true
fileindex_scan = "%D"
`)
	runMenuKey(a, 'b')
	waitCommandsDone(t, a)
	assertNoScan(t, a, screen)
}

func TestUserMenuFileIndexScanSkipsKilledCommand(t *testing.T) {
	a, screen, _, _ := indexScanApp(t, `[killed]
key = "k"
title = "Killed"
command = "kill -KILL $$"
shell = true
background = true
fileindex_scan = "%D"
`)
	runMenuKey(a, 'k')
	waitCommandsDone(t, a)
	assertNoScan(t, a, screen)
}

func TestUserMenuFileIndexScanKeepsFailureNotification(t *testing.T) {
	a, screen, _, dest := indexScanApp(t, `[fail]
key = "f"
title = "Fail"
command = "false"
background = true
fileindex_scan = "%D"
`)
	runMenuKey(a, 'f')
	waitScanSources(t, a, screen, dest)
	// Past the job's end: its started and completed toasts must not replace the banner either.
	waitLastJobFinished(t, a, screen)
	if a.model.MessageUrgency != ui.MessageUrgencyError || !strings.Contains(a.model.Message, "Fail") {
		t.Fatalf("message = %q (%v), want the failure banner", a.model.Message, a.model.MessageUrgency)
	}
}

// The directory is the one %D named at launch, wherever the panel went while the command ran.
func TestUserMenuFileIndexScanDirFixedAtLaunch(t *testing.T) {
	a, screen, _, dest := indexScanApp(t, `[slow]
key = "s"
title = "Slow"
command = "sleep 0.5"
background = true
fileindex_scan = "%D"
`)
	runMenuKey(a, 's')
	moveOtherPanel(t, a, screen, t.TempDir())
	waitScanSources(t, a, screen, dest)
}

func TestUserMenuFileIndexScanInteractive(t *testing.T) {
	exitErr := exec.Command("sh", "-c", "exit 3").Run()
	if exitErr == nil {
		t.Fatal("want a non-zero exit")
	}
	const body = `[edit]
key = "e"
title = "Edit"
command = "true"
interactive = true
fileindex_scan = "%D"
`
	stub := func(t *testing.T, err error) {
		prev := userMenuInteractiveRunner
		userMenuInteractiveRunner = func(context.Context, []string, string) error { return err }
		t.Cleanup(func() { userMenuInteractiveRunner = prev })
	}

	for name, err := range map[string]error{"success": nil, "non-zero exit": exitErr} {
		t.Run(name, func(t *testing.T) {
			stub(t, err)
			a, screen, _, dest := indexScanApp(t, body)
			runMenuKey(a, 'e')
			waitScanSources(t, a, screen, dest)
		})
	}
	t.Run("launch failure", func(t *testing.T) {
		stub(t, errors.New("run command: lantern not found"))
		a, screen, _, _ := indexScanApp(t, body)
		runMenuKey(a, 'e')
		assertNoScan(t, a, screen)
	})
}

func TestUserMenuFileIndexScanRunForEach(t *testing.T) {
	a, screen, dir, dest := indexScanApp(t, `[each]
key = "e"
title = "Each"
command = "sh -c 'test %f = nothing'"
run_for_each = ["files"]
background = true
fileindex_scan = "%D"
`)
	a.activePanel().SelectedPaths = map[string]bool{
		filepath.Join(dir, "harbor.txt"): true,
		filepath.Join(dir, "meadow.txt"): true,
	}
	runMenuKey(a, 'e')
	// Both items fail, and the one batch still scans once.
	waitScanSources(t, a, screen, dest)
	assertScanCountStays(t, a, screen, 1)
}

func assertScanCountStays(t *testing.T, a *App, screen tcell.SimulationScreen, want int) {
	t.Helper()
	deadline := time.Now().Add(indexScanSettle)
	drainInterruptEventsUntil(t, a, screen, 2*indexScanSettle+time.Second, func() bool { return time.Now().After(deadline) })
	if got := scanSources(a); len(got) != want {
		t.Fatalf("scan sources = %v, want %d", got, want)
	}
}

func TestUserMenuFileIndexScanRunForEachNothingRan(t *testing.T) {
	a, screen, _, _ := indexScanApp(t, `[dirs]
key = "d"
title = "Dirs"
command = "echo %f"
run_for_each = ["dirs"]
background = true
fileindex_scan = "%D"
`)
	cursorTo(t, a, "harbor.txt") // a file, so the dirs-only entry skips its one row
	runMenuKey(a, 'd')
	waitCommandsDone(t, a)
	assertNoScan(t, a, screen)
}

// A fileindex:// directory scans the host path it stands for.
func TestScanAfterCommandIndexPathScansHostPath(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	a.fileindexCtrl.ScanAfterCommand(pathloc.MustParse(pathloc.FileIndexPrefix + dir))
	waitScanSources(t, a, screen, dir)
}

// The index picks up what the command wrote, in the directory fileindex_scan named.
func TestUserMenuFileIndexScanIndexesNewFile(t *testing.T) {
	a, screen, _, dest := indexScanApp(t, `[write]
key = "w"
title = "Write"
command = "echo beacon > %D/lantern.txt"
shell = true
background = true
fileindex_scan = "%D"
`)
	runMenuKey(a, 'w')
	waitScanSources(t, a, screen, dest)
	waitLastJobFinished(t, a, screen)
	if _, err := fsbackend.Default().Stat(context.Background(), pathloc.MustParse(pathloc.FileIndexPrefix+filepath.Join(dest, "lantern.txt"))); err != nil {
		t.Fatalf("lantern.txt not indexed: %v", err)
	}
}
