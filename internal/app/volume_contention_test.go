package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// addPausedJobOn enqueues a paused (so the worker never picks it up, but it still counts as
// unfinished work) copy job whose source and destination live under dir, giving the job real
// cached VolumeDevs for dir's device.
func addPausedJobOn(t *testing.T, app *App, dir string) {
	t.Helper()
	src := filepath.Join(dir, "contend-source.dat")
	dst := filepath.Join(dir, "contend-target")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	job := &jobs.Job{
		ID:          "contend",
		Type:        jobs.TypeCopy,
		Status:      jobs.StatusPaused,
		Sources:     pathloc.PathsForTest(src),
		Destination: pathloc.MustParse(dst),
	}
	app.jobState.AddJob(job)
	if len(job.VolumeDevs) == 0 {
		t.Fatal("job cached no volume devs; contention test cannot run")
	}
	if !app.pathVolumeContendsWithActiveJob(dir) {
		t.Fatal("dir should contend with the paused job")
	}
}

func TestFilterJobContendedPaths(t *testing.T) {
	app := testAppMinimal(t)
	dir := t.TempDir()
	// A path that cannot be stat'ed has no device to contend over, so it is always kept.
	ghost := filepath.Join(dir, "no-such-directory")
	in := []string{dir, ghost}

	if got := app.filterJobContendedPaths(in); len(got) != 2 {
		t.Fatalf("no unfinished jobs must pass paths through unchanged, got %v", got)
	}

	addPausedJobOn(t, app, dir)

	got := app.filterJobContendedPaths(in)
	if len(got) != 1 || got[0] != ghost {
		t.Fatalf("contended dir should be dropped and %q kept, got %v", ghost, got)
	}
	if in[0] != dir {
		t.Fatalf("input slice must not be overwritten, got %v", in)
	}
}

func TestGitStatusSchedulerSkipsJobContendedVolume(t *testing.T) {
	app := testAppMinimal(t)
	dir := t.TempDir()
	schedule := app.gitStatusScheduler(ui.PrimaryPanel)
	req := panel.GitStatusRequest{WorkRoot: dir, ListDir: dir}

	if !schedule(req) {
		t.Fatal("git status should be scheduled with no unfinished jobs")
	}

	addPausedJobOn(t, app, dir)

	if schedule(req) {
		t.Fatal("git status must not be scheduled for a job-contended volume")
	}
}

func TestSchedulePanelListingRefreshSkipsJobContendedVolume(t *testing.T) {
	app := testAppMinimal(t)
	p := app.panelByID(ui.PrimaryPanel)
	addPausedJobOn(t, app, p.PathString())

	app.schedulePanelListingRefresh(ui.PrimaryPanel)

	if app.panelRefreshInFlight[ui.PrimaryPanel].Load() {
		t.Fatal("contended panel refresh must not flip the in-flight flag")
	}
}
