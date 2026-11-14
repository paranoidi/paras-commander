package rsync

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/jobs"
)

const (
	// Lease serializes rsync jobs: one runs at a time, beside the transfer lease's copies.
	Lease = "rsync"

	// terminateGrace is how long a canceled rsync gets to stop after SIGTERM before its process
	// group is killed.
	terminateGrace = 5 * time.Second
	// stderrCap bounds the stderr kept as the job's warnings: the cap is the only limit on how
	// many lines they are, so a run with many failed files keeps the last few hundred.
	stderrCap = 64 * 1024
	// maxDryRunLines bounds the itemized changes a dry run keeps.
	maxDryRunLines = 200_000
)

// ErrNotInstalled is returned by a run when rsync is not on PATH.
var ErrNotInstalled = errors.New("rsync not found in PATH")

// NewTask returns the job task that runs rsync with o. A dry run reports its itemized changes to
// onDryRun (called once, on the worker goroutine, with nil when nothing would change) when the
// run succeeds; onDryRun may be nil. The task may run again (a retried job).
func NewTask(o Options, onDryRun func(changes []string)) *jobs.Task {
	return &jobs.Task{
		Title: "rsync",
		Lease: Lease,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			return run(ctx, r, o, onDryRun)
		},
	}
}

func run(ctx context.Context, r jobs.TaskReporter, o Options, onDryRun func([]string)) error {
	bin, err := exec.LookPath("rsync")
	if err != nil {
		return ErrNotInstalled
	}
	if err := checkDest(o); err != nil {
		return err
	}
	o.Report, o.Progress2 = true, progress2Supported()
	argv := BuildArgv(o)

	cmd := exec.CommandContext(ctx, bin, argv[1:]...)
	cmdrun.DetachProcessGroup(cmd)
	var killTimer *time.Timer
	var mu sync.Mutex
	// A canceled run gets SIGTERM (rsync cleans up its temporary files), then SIGKILL.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := cmdrun.SignalGroup(cmd.Process, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		mu.Lock()
		killTimer = time.AfterFunc(terminateGrace, func() { _ = cmdrun.SignalGroup(cmd.Process, syscall.SIGKILL) })
		mu.Unlock()
		return err
	}
	cmd.WaitDelay = terminateGrace + 2*time.Second
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		if killTimer != nil {
			killTimer.Stop()
		}
	}()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr cmdrun.CappedWriter
	stderr.SetMaxIfZero(stderrCap)
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	rd := &reader{rep: r, dry: o.DryRun}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sc.Split(splitLines)
	for sc.Scan() {
		rd.line(sc.Text())
	}
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	// The exit status is the process's own: a grandchild holding the pipes open past WaitDelay
	// (ssh) only makes Wait report exec.ErrWaitDelay.
	if cmd.ProcessState == nil {
		return waitErr
	}
	if code := cmd.ProcessState.ExitCode(); code != 0 {
		return failure(r, code, stderr.Bytes())
	}
	warnStderr(r, stderr.Bytes()) // rsync's warnings of a run that still succeeded
	rd.finish()
	if o.DryRun && onDryRun != nil {
		onDryRun(rd.changes)
	}
	return nil
}

// reader turns rsync's stdout lines into task progress.
type reader struct {
	rep jobs.TaskReporter
	dry bool

	prog    jobs.TaskProgress
	changes []string // itemized lines of a dry run
}

func (rd *reader) line(s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	if !rd.dry {
		if p, ok := parseProgress(s); ok {
			rd.prog.DoneBytes = p.Bytes
			rd.prog.TotalBytes = max(rd.prog.TotalBytes, p.TotalBytes())
			if p.TotalFiles > 0 {
				rd.prog.TotalFiles, rd.prog.DoneFiles = p.TotalFiles, p.DoneFiles
			}
			rd.rep.Progress(rd.prog)
			return
		}
		rd.prog.CurrentPath = s
		if rd.prog.TotalFiles == 0 { // no progress2: the names are all there is to count
			rd.prog.DoneFiles++
		}
		rd.rep.Progress(rd.prog)
		return
	}
	if name, ok := itemizedName(s); ok {
		if len(rd.changes) < maxDryRunLines {
			rd.changes = append(rd.changes, s)
		}
		rd.prog.CurrentPath = name
		rd.prog.DoneFiles++
		rd.rep.Progress(rd.prog)
	}
}

// finish reports the run as complete: the last progress line can lag behind the end of the run
// (an unchanged tree prints none that reaches 100%).
func (rd *reader) finish() {
	if rd.dry {
		return
	}
	rd.prog.DoneFiles = max(rd.prog.DoneFiles, rd.prog.TotalFiles)
	if rd.prog.TotalBytes > 0 {
		rd.prog.DoneBytes = max(rd.prog.DoneBytes, rd.prog.TotalBytes)
	}
	rd.rep.Progress(rd.prog)
}

// checkDest fails a run whose local destination rsync could not create, before rsync starts: rsync
// creates only the last component of the destination, and its dry run does not notice a missing
// parent that the real run fails on. A remote destination ([user@]host:path, not absolute) is
// left to rsync, and so is any destination when --mkpath makes it create the whole path.
func checkDest(o Options) error {
	if !filepath.IsAbs(o.Dest) || slices.Contains(o.Extra, "--mkpath") {
		return nil
	}
	dest := filepath.Clean(o.Dest)
	fi, err := os.Stat(dest)
	if err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("destination is not a directory: %s", dest)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) { // ENOTDIR: a file is in the path
		return nil // rsync reports it (permission denied)
	}
	parent := filepath.Dir(dest)
	pi, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("destination parent does not exist: %s", parent)
	}
	if !pi.IsDir() {
		return fmt.Errorf("destination parent is not a directory: %s", parent)
	}
	return nil
}

// failure is the error of an rsync that exited with code: the exit meaning only. rsync's own
// messages (stderr) are the job's warnings, which its Messages pane shows in full.
func failure(r jobs.TaskReporter, code int, stderr []byte) error {
	warnStderr(r, stderr)
	return errors.New(exitMessage(code))
}

// warnStderr adds each non-blank stderr line as a job warning.
func warnStderr(r jobs.TaskReporter, stderr []byte) {
	for _, l := range bytes.Split(stderr, []byte("\n")) {
		if s := strings.TrimSpace(string(l)); s != "" {
			r.Warn(s)
		}
	}
}
