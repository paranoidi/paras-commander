package rsync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// fakeReporter records what a run reports.
type fakeReporter struct {
	mu       sync.Mutex
	progress []jobs.TaskProgress
	warnings []string
}

func (f *fakeReporter) Progress(p jobs.TaskProgress) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress = append(f.progress, p)
}
func (f *fakeReporter) Warn(m string)    { f.mu.Lock(); f.warnings = append(f.warnings, m); f.mu.Unlock() }
func (f *fakeReporter) FinishSource(int) {}

func (f *fakeReporter) last() jobs.TaskProgress {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.progress) == 0 {
		return jobs.TaskProgress{}
	}
	return f.progress[len(f.progress)-1]
}

func TestReaderCountsProgress2Stream(t *testing.T) {
	r := &fakeReporter{}
	rd := &reader{rep: r}
	for _, l := range []string{
		"lantern.txt",
		"           3   0%    0.00kB/s    0:00:00  ",
		"      1,500   50%    1.00kB/s    0:00:00 (xfr#1, to-chk=1/4)",
		"orchard/willow.bin",
		"      3,000  100%    1.00kB/s    0:00:01 (xfr#2, to-chk=0/4)",
	} {
		rd.line(l)
	}
	rd.finish()
	got := r.last()
	if got.TotalFiles != 4 || got.DoneFiles != 4 || got.DoneBytes != 3000 || got.TotalBytes != 3000 || got.CurrentPath != "orchard/willow.bin" {
		t.Fatalf("last progress = %+v", got)
	}
}

func TestReaderOldRsyncCountsNames(t *testing.T) {
	r := &fakeReporter{}
	rd := &reader{rep: r}
	rd.line("lantern.txt")
	rd.line("orchard/willow.bin")
	rd.finish()
	got := r.last()
	if got.DoneFiles != 2 || got.TotalFiles != 0 || got.TotalBytes != 0 || got.CurrentPath != "orchard/willow.bin" {
		t.Fatalf("last progress = %+v, want 2 names counted and no totals", got)
	}
}

func TestReaderDryRunCollectsItemizedLines(t *testing.T) {
	r := &fakeReporter{}
	rd := &reader{rep: r, dry: true}
	for _, l := range []string{">f+++++++++ lantern.txt", "sending incremental file list", "*deleting   thistle.txt", "  0 0% 0.00kB/s 0:00:00 (xfr#0, to-chk=0/3)"} {
		rd.line(l)
	}
	if len(rd.changes) != 2 || rd.changes[1] != "*deleting   thistle.txt" {
		t.Fatalf("changes = %q", rd.changes)
	}
}

func TestFailureKeepsErrorShortAndStderrAsWarnings(t *testing.T) {
	for _, tc := range []struct {
		code int
		want string
	}{
		{23, "rsync: partial transfer due to error (exit 23)"},
		{12, "rsync: error in rsync protocol data stream (exit 12)"},
	} {
		r := &fakeReporter{}
		err := failure(r, tc.code, []byte("meadow: protocol error\n\n  \nrsync error: something (code 12)\n"))
		if err == nil || err.Error() != tc.want {
			t.Fatalf("code %d: err = %v, want %s", tc.code, err, tc.want)
		}
		if len(r.warnings) != 2 || r.warnings[0] != "meadow: protocol error" {
			t.Fatalf("code %d: warnings = %q, want the 2 non-blank stderr lines", tc.code, r.warnings)
		}
	}
}

func TestCheckDest(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "lantern.txt")
	write(t, file, 1)
	for _, tc := range []struct {
		name  string
		dest  string
		extra []string
		want  string // error text, "" for none
	}{
		{"existing dir", root, nil, ""},
		{"missing leaf", filepath.Join(root, "willow") + "/", nil, ""},
		{"missing parent", filepath.Join(root, "willow", "thistle"), nil, "destination parent does not exist: " + filepath.Join(root, "willow")},
		{"missing parent with mkpath", filepath.Join(root, "willow", "thistle"), []string{"--mkpath"}, ""},
		{"regular file", file, nil, "destination is not a directory: " + file},
		{"file as parent", filepath.Join(file, "thistle"), nil, "destination parent is not a directory: " + file},
		{"remote", "harbor:/srv/meadow", nil, ""},
	} {
		err := checkDest(Options{Dest: tc.dest, Extra: tc.extra})
		if got := fmt.Sprint(err); (tc.want == "" && err != nil) || (tc.want != "" && got != tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}

// --- integration: a real rsync under the real job queue ---

func needRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed")
	}
}

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i*7 + size)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// runJob queues task as a TypeRsync job on a fresh job state and returns the state, the job id and
// a wait function that applies events until the job finishes.
func startJob(t *testing.T, task *jobs.Task, src, dest string) (*jobs.State, *jobs.Job, func() *jobs.Job) {
	t.Helper()
	s := jobs.NewState()
	s.SetScanConfig(jobs.ScanConfig{})
	stop := make(chan struct{})
	s.StartWorker(stop)
	t.Cleanup(func() { close(stop) })
	job := jobs.NewRsyncJob(task, pathloc.PathsForTest(src), pathloc.PathsForTest(dest)[0])
	s.AddJob(job)
	wait := func() *jobs.Job {
		deadline := time.After(20 * time.Second)
		for {
			select {
			case ev := <-s.Events():
				s.ApplyEvent(ev)
				if ev.JobID == job.ID && ev.Status.IsFinished() {
					return s.Job(job.ID)
				}
			case <-deadline:
				t.Fatal("job did not finish")
				return nil
			}
		}
	}
	return s, job, wait
}

func TestRsyncJobSyncsAndMirrors(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src, dest := filepath.Join(root, "orchard"), filepath.Join(root, "meadow")
	write(t, filepath.Join(src, "lantern.bin"), 200_000)
	write(t, filepath.Join(src, "willow", "thistle.txt"), 5_000)
	write(t, filepath.Join(dest, "stale.txt"), 100)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, ContentsOnly: true}, nil)
	_, _, wait := startJob(t, task, src, dest)
	got := wait()
	if got.Status != jobs.StatusCompleted || got.Type != jobs.TypeRsync {
		t.Fatalf("status = %s type = %s error = %q", got.Status, got.Type, got.Error)
	}
	if got.TotalBytes == 0 || got.DoneBytes != got.TotalBytes {
		t.Fatalf("bytes done %d of %d, want equal and non-zero", got.DoneBytes, got.TotalBytes)
	}
	if got.TotalFiles == 0 || got.DoneFiles != got.TotalFiles {
		t.Fatalf("files done %d of %d, want equal and non-zero", got.DoneFiles, got.TotalFiles)
	}
	if _, err := os.Stat(filepath.Join(dest, "willow", "thistle.txt")); err != nil {
		t.Fatalf("file not synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale.txt")); err != nil {
		t.Fatalf("update mode must keep the extra file: %v", err)
	}

	mirror := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, ContentsOnly: true, Mode: ModeMirror}, nil)
	_, _, wait = startJob(t, mirror, src, dest)
	if got := wait(); got.Status != jobs.StatusCompleted {
		t.Fatalf("mirror status = %s error = %q", got.Status, got.Error)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("mirror must delete the extra file, stat err = %v", err)
	}
}

func TestRsyncDryRunChangesNothingAndReportsChanges(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src, dest := filepath.Join(root, "orchard"), filepath.Join(root, "meadow")
	write(t, filepath.Join(src, "lantern.bin"), 1_000)
	write(t, filepath.Join(src, "willow.txt"), 10)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	var changes []string
	called := make(chan struct{})
	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, ContentsOnly: true, DryRun: true}, func(c []string) {
		changes = c
		close(called)
	})
	_, _, wait := startJob(t, task, src, dest)
	if got := wait(); got.Status != jobs.StatusCompleted {
		t.Fatalf("status = %s error = %q", got.Status, got.Error)
	}
	<-called
	if len(changes) != 2 {
		t.Fatalf("changes = %q, want the 2 new files", changes)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Fatalf("dry run wrote %d entries", len(entries))
	}
}

func TestRsyncJobFailsWithExitMeaning(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	missing := filepath.Join(root, "nowhere")
	task := NewTask(Options{Sources: []string{missing}, Dest: filepath.Join(root, "meadow"), Archive: true}, nil)
	_, _, wait := startJob(t, task, missing, filepath.Join(root, "meadow"))
	got := wait()
	if got.Status != jobs.StatusFailed || got.Error == "" {
		t.Fatalf("status = %s error = %q, want failed with a message", got.Status, got.Error)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("a partial transfer keeps the stderr lines as warnings")
	}
}

func TestRsyncDryRunFailsForMissingDestinationParent(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src := filepath.Join(root, "orchard")
	write(t, filepath.Join(src, "lantern.bin"), 100)
	dest := filepath.Join(root, "missing1", "missing2")
	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, DryRun: true}, nil)
	_, _, wait := startJob(t, task, src, dest)
	got := wait()
	want := "destination parent does not exist: " + filepath.Join(root, "missing1")
	if got.Status != jobs.StatusFailed || got.Error != want {
		t.Fatalf("status = %s error = %q, want failed with %q", got.Status, got.Error, want)
	}
}

func TestRsyncDryRunCompletesWhenOnlyTheLeafIsMissing(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src := filepath.Join(root, "orchard")
	write(t, filepath.Join(src, "lantern.bin"), 100)
	dest := filepath.Join(root, "missing1")
	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, DryRun: true}, nil)
	_, _, wait := startJob(t, task, src, dest)
	if got := wait(); got.Status != jobs.StatusCompleted {
		t.Fatalf("status = %s error = %q", got.Status, got.Error)
	}
}

func TestRsyncJobFailureListsStderrAsWarnings(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src, dest := filepath.Join(root, "orchard"), filepath.Join(root, "meadow")
	write(t, filepath.Join(src, "lantern.bin"), 100)
	// An unknown option is a usage error (exit 1), neither partial nor a destination problem.
	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, Extra: []string{"--no-such-thistle"}}, nil)
	_, _, wait := startJob(t, task, src, dest)
	got := wait()
	if got.Status != jobs.StatusFailed || got.Error != "rsync: syntax or usage error (exit 1)" {
		t.Fatalf("status = %s error = %q, want failed with the short exit meaning", got.Status, got.Error)
	}
	if !strings.Contains(strings.Join(got.Warnings, "\n"), "no-such-thistle") {
		t.Fatalf("warnings = %q, want rsync's stderr lines", got.Warnings)
	}
}

func TestRsyncJobCancelEndsCanceled(t *testing.T) {
	needRsync(t)
	root := t.TempDir()
	src, dest := filepath.Join(root, "orchard"), filepath.Join(root, "meadow")
	write(t, filepath.Join(src, "lantern.bin"), 2_000_000)
	// 1 KiB/s keeps the single file in flight far longer than the test.
	task := NewTask(Options{Sources: []string{src}, Dest: dest, Archive: true, ContentsOnly: true, BwLimit: 1}, nil)
	s, job, wait := startJob(t, task, src, dest)

	deadline := time.After(10 * time.Second)
	for running := false; !running; {
		select {
		case ev := <-s.Events():
			s.ApplyEvent(ev)
			running = ev.JobID == job.ID && ev.Type == jobs.EventStarted
		case <-deadline:
			t.Fatal("job never started")
		}
	}
	time.Sleep(300 * time.Millisecond) // let rsync start
	start := time.Now()
	if !s.CancelJob(job.ID) {
		t.Fatal("CancelJob refused")
	}
	if got := wait(); got.Status != jobs.StatusCanceled {
		t.Fatalf("status = %s error = %q, want canceled", got.Status, got.Error)
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Fatalf("cancel took %s", d)
	}
}
