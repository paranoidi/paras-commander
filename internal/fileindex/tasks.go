package fileindex

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
)

// TaskLease serializes fileindex task jobs: one scan or prune at a time.
const TaskLease = "fileindex"

// ScanTask returns the task job scanning paths (local host paths) into the index: hashing new
// and changed files, then pruning what is gone under them (fileindex's scan.Run with its
// config.ini rules). Paths are scanned one at a time (jobs.TaskReporter.StartSource), so each
// one's file-list mark clears when it is done.
// ponytail: one scan.Run per path loses hashing parallelism across many small selected files;
// batch paths into one run if that gets slow.
func (x *Index) ScanTask(paths []string) *jobs.Task {
	return &jobs.Task{
		Title: "fileindex scan",
		Lease: TaskLease,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			d, err := x.DB()
			if err != nil {
				return err
			}
			rules, workers, err := x.scanSettings()
			if err != nil {
				return err
			}
			rep := newScanReporter(r)
			var total scan.Stats
			for i, p := range paths {
				r.StartSource(i)
				rep.next(total)
				stats, err := scan.Run(ctx, d, []string{p}, scan.Options{
					Rules:       rules,
					Workers:     workers,
					MissingOnly: true,
					BatchFiles:  config.FileIndexScanBatchFiles,
				}, rep)
				for _, e := range stats.Errors {
					rep.warn(e)
				}
				total.Processed += stats.Processed
				total.Errored += stats.Errored
				total.TotalSize += stats.TotalSize
				total.Interrupted = stats.Interrupted
				if err != nil || stats.Interrupted {
					rep.finish(total)
					if err != nil {
						return err
					}
					return context.Canceled
				}
			}
			rep.finish(total)
			return nil
		},
	}
}

// PruneTask returns the task job removing from the index whatever under path (a local host path,
// resolved like the index records it) is gone from disk, then the orphaned rows.
func (x *Index) PruneTask(path string) *jobs.Task {
	return &jobs.Task{
		Title: "fileindex prune",
		Lease: TaskLease,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			d, err := x.DB()
			if err != nil {
				return err
			}
			host, err := IndexPath(path)
			if err != nil {
				return err
			}
			w := &warnings{r: r}
			_, err = d.PrunePath(ctx, host, func(dir string, checked int) {
				r.Progress(jobs.TaskProgress{DoneFiles: checked, CurrentPath: dir})
			}, w.warn)
			w.flush()
			return err
		},
	}
}

// warnings forwards warning lines to the job up to config.FileIndexMaxWarnings and counts the rest.
type warnings struct {
	r       jobs.TaskReporter
	mu      sync.Mutex
	n, more int
}

func (w *warnings) warn(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.n >= config.FileIndexMaxWarnings {
		w.more++
		return
	}
	w.n++
	w.r.Warn(msg)
}

// flush adds the count of dropped lines.
func (w *warnings) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.more > 0 {
		w.r.Warn(fmt.Sprintf("… %d more", w.more))
	}
}

// scanReporter maps scan.Reporter onto task progress: discovery sets the total, a hash worker
// starting its next file counts the previous one done, Speed samples add the bytes hashed since
// the last sample, and pruning shows the directory being checked. Each scan.Run's counts add to
// those of the runs before it (next). Its methods are called from scan's goroutines
// concurrently.
type scanReporter struct {
	r jobs.TaskReporter
	w warnings

	mu       sync.Mutex
	p        jobs.TaskProgress
	busy     map[int]bool
	bytes    float64
	lastTick time.Time
	base     scan.Stats // committed counts of the finished runs
	baseNew  int        // files to hash found by the finished runs
}

func newScanReporter(r jobs.TaskReporter) *scanReporter {
	return &scanReporter{r: r, w: warnings{r: r}, busy: map[int]bool{}}
}

// next starts the next scan.Run on top of the finished runs' committed counts done.
func (s *scanReporter) next(done scan.Stats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base, s.baseNew = done, s.p.TotalFiles
	s.p.DoneFiles = done.Processed + done.Errored
	s.bytes = float64(done.TotalSize)
	s.busy = map[int]bool{}
	s.lastTick = time.Time{}
}

// report sends the current sample; caller holds s.mu.
func (s *scanReporter) report() {
	s.p.DoneBytes = int64(s.bytes)
	s.r.Progress(s.p)
}

func (s *scanReporter) Worker(id int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy[id] {
		s.p.DoneFiles++
	}
	s.busy[id] = true
	s.p.CurrentPath = path
	s.report()
}

func (s *scanReporter) Speed(_ int, bps float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.lastTick.IsZero() {
		s.bytes += bps * now.Sub(s.lastTick).Seconds()
	}
	s.lastTick = now
	s.report()
}

func (s *scanReporter) Scan(dir string, _, newFiles, _ int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.TotalFiles = max(s.p.TotalFiles, s.baseNew+newFiles)
	if dir != "" && len(s.busy) == 0 {
		s.p.CurrentPath = dir
	}
	s.report()
}

// Hashed replaces the estimates with what was committed.
func (s *scanReporter) Hashed(res scan.Stats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.DoneFiles = s.base.Processed + s.base.Errored + res.Processed + res.Errored
	s.p.TotalFiles = max(s.p.TotalFiles, s.p.DoneFiles)
	s.bytes = float64(s.base.TotalSize + res.TotalSize)
	s.report()
}

func (s *scanReporter) Prune(dir string, _ int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.CurrentPath = dir
	s.report()
}

func (s *scanReporter) Log(msg string) { s.w.warn(msg) }

func (s *scanReporter) warn(msg string) { s.w.warn(msg) }

// finish flushes the dropped-warning count and, for a scan that completed its hashing, the
// committed counts of every run (stats).
func (s *scanReporter) finish(stats scan.Stats) {
	s.w.flush()
	if stats.Interrupted {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.DoneFiles = stats.Processed + stats.Errored
	s.p.TotalFiles = max(s.p.TotalFiles, s.p.DoneFiles)
	s.bytes = float64(stats.TotalSize)
	s.p.CurrentPath = ""
	s.report()
}

// compile-time check: scanReporter is a scan.Reporter.
var _ scan.Reporter = (*scanReporter)(nil)
