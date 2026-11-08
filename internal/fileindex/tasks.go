package fileindex

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/paranoidi/fileindex/scan"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
)

// TaskLease serializes fileindex task jobs: one scan or prune at a time.
const TaskLease = "fileindex"

// DifferencesLease serializes the differences reports: one at a time, beside scans and prunes (a
// report only reads, so it does not wait behind a long scan).
const DifferencesLease = "fileindex.differences"

// ScanTask returns the task job scanning paths (local host paths) into the index: hashing new
// and changed files, then pruning what is gone under them (fileindex's scan.Run with its
// config.ini rules). Each path is reported done (jobs.TaskReporter.FinishSource) once its files
// are committed, while the others are still hashed, so its file-list mark clears.
func (x *Index) ScanTask(paths []string) *jobs.Task {
	return x.scanTask(paths, false, "fileindex scan", "Hash new files", nil)
}

// ScanTagTask is ScanTask adding tags to each path (recursive as in EditTags) once the scan has
// committed that path's files, so the tags do not wait for hashing and survive a cancel during
// it. A path the scan did not record (its rules skip it) gets a warning.
func (x *Index) ScanTagTask(paths, tags []string, recursive bool) *jobs.Task {
	return x.scanTask(paths, false, "fileindex scan", "Hash new files", func(path string, warn func(string)) {
		res, err := x.EditTags([]string{path}, tags, true, recursive)
		switch {
		case err != nil:
			warn(fmt.Sprintf("Not tagged: %s: %v", path, err))
		case len(res.Unknown) > 0:
			warn("Not tagged, the scan did not index it: " + path)
		}
		x.changed() // the tags show now, not when hashing ends
	})
}

// ScanFastTask is ScanTask hashing only the first 8 KB of files larger than that (scan --fast):
// they stay unhashed until a plain scan. Asked for explicitly, it hashes even with hash off.
func (x *Index) ScanFastTask(paths []string) *jobs.Task {
	return x.scanTask(paths, true, "fileindex fast scan", "Fast-hash new files", nil)
}

// scanTask builds a scan job; rootDone (optional) runs for each path once its files are committed.
func (x *Index) scanTask(paths []string, fast bool, title, hashStep string, rootDone func(path string, warn func(string))) *jobs.Task {
	skipHash := !x.cfg.Hash && !fast
	steps := []string{"Walk filesystem", "Pruning", hashStep}
	if skipHash {
		steps = steps[:2]
	}
	return &jobs.Task{
		Title: title,
		Lease: TaskLease,
		Steps: steps,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			defer x.changed()
			d, err := x.DB()
			if err != nil {
				return err
			}
			rules, workers, err := x.scanSettings()
			if err != nil {
				return err
			}
			rep := newScanReporter(r, skipHash)
			rep.onCommit = x.OnScanCommit
			if rootDone != nil {
				rep.rootDone = func(i int) { rootDone(paths[i], rep.Log) }
			}
			stats, err := scan.Run(ctx, d, paths, scan.Options{
				Rules:         rules,
				Workers:       workers,
				MissingOnly:   true,
				SkipHash:      skipHash,
				Fast:          fast,
				BatchInterval: x.cfg.ScanCommitInterval(),
			}, rep)
			for _, e := range stats.Errors {
				rep.Log(e)
			}
			rep.finish(stats)
			if err != nil {
				return pcSchemaError(err)
			}
			if stats.Interrupted {
				return context.Canceled
			}
			return nil
		},
	}
}

// HashPotentialTask returns the task job fully hashing the only fast-hashed files among paths
// (local host paths) that are in a potential duplicate set, which confirms or rules out the match.
// done runs when it ends, however it ends.
func (x *Index) HashPotentialTask(paths []string, done func()) *jobs.Task {
	return &jobs.Task{
		Title: "fileindex hash potential duplicates",
		Lease: TaskLease,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			defer done()
			defer x.changed()
			d, err := x.DB()
			if err != nil {
				return err
			}
			_, workers, err := x.scanSettings()
			if err != nil {
				return err
			}
			groups, err := d.PotentialDuplicates(nil, false)
			if err != nil {
				return err
			}
			asked := make(map[string]bool, len(paths))
			for _, p := range paths {
				asked[p] = true
			}
			var targets []string
			for _, g := range groups {
				for _, p := range g.Unverified {
					if asked[p] {
						targets = append(targets, p)
					}
				}
			}
			rep := newScanReporter(r, false)
			rep.hashing, rep.p.TotalFiles = true, len(targets)
			stats, err := scan.HashPaths(ctx, d, targets, workers, rep.Worker, rep.Speed, rep.FileDone)
			for _, e := range stats.Errors {
				rep.Log(e)
			}
			rep.finish(stats)
			if err != nil {
				return pcSchemaError(err)
			}
			if stats.Interrupted {
				return context.Canceled
			}
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
			defer x.changed()
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

// scanReporter maps scan.Reporter (and scan.PhaseReporter) onto task progress, one stage after
// the other. Walking: DoneFiles counts the files committed (searchable, unhashed) and TotalFiles
// stays unknown. Pruning: TotalFiles is still unknown and CurrentPath is the directory being
// checked. Hashing: TotalFiles is the number of files to hash, DoneFiles counts the files whose
// hashing finished or failed (FileDone, scan.FileReporter), Speed samples add the bytes hashed
// since the last sample. A root whose files are committed finishes its job source. Its methods are called from scan's goroutines concurrently.
type scanReporter struct {
	r jobs.TaskReporter
	w warnings

	mu       sync.Mutex
	p        jobs.TaskProgress
	skipHash bool                // the scan stops after pruning
	rootDone func(i int)         // optional, runs before the root's job source finishes
	onCommit func(dirs []string) // optional, Index.OnScanCommit
	hashing  bool                // Pruned was reached and hashing follows
	bytes    float64
	lastTick time.Time
}

func newScanReporter(r jobs.TaskReporter, skipHash bool) *scanReporter {
	return &scanReporter{r: r, w: warnings{r: r}, skipHash: skipHash}
}

// report sends the current sample; caller holds s.mu.
func (s *scanReporter) report() {
	s.p.DoneBytes = int64(s.bytes)
	s.r.Progress(s.p)
}

func (s *scanReporter) Worker(id int, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.CurrentPath = path
	s.report()
}

func (s *scanReporter) FileDone() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.DoneFiles++
	s.report()
}

func (s *scanReporter) Speed(workers int, bps float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.Workers = workers
	now := time.Now()
	if !s.lastTick.IsZero() {
		s.bytes += bps * now.Sub(s.lastTick).Seconds()
	}
	s.lastTick = now
	s.report()
}

func (s *scanReporter) Scan(dir string, _, _, _ int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if dir != "" {
		s.p.CurrentPath = dir
	}
	s.report()
}

// Indexed counts the files committed by the walk.
func (s *scanReporter) Indexed(committed int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.DoneFiles = committed
	s.report()
}

// Discovered ends the walk: the committed count is final and the current path is stale.
func (s *scanReporter) Discovered(res scan.Stats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.DoneFiles = res.Discovered
	s.p.Step = 1
	s.p.CurrentPath = ""
	s.report()
}

// Pruned starts the hashing stage: the progress restarts at 0 of toHash.
func (s *scanReporter) Pruned(_ scan.Stats, toHash int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.skipHash {
		return
	}
	s.hashing = true
	s.p.Step = 2
	s.p.TotalFiles = toHash
	s.p.DoneFiles = 0
	s.p.CurrentPath = ""
	s.report()
}

// Hashed replaces the hashing estimates with what was committed.
func (s *scanReporter) Hashed(res scan.Stats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.Workers = 0
	if !s.hashing { // cut short before hashing, or SkipHash: keep the walk's count
		s.report()
		return
	}
	s.p.DoneFiles = res.Processed + res.Errored
	s.p.TotalFiles = max(s.p.TotalFiles, s.p.DoneFiles)
	s.bytes = float64(res.TotalSize)
	s.report()
}

func (s *scanReporter) Prune(dir string, _ int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p.CurrentPath = dir
	s.report()
}

func (s *scanReporter) Log(msg string) { s.w.warn(msg) }

// Committed passes the directories of a committed batch on (scan.CommitReporter).
func (s *scanReporter) Committed(_ scan.Stage, dirs []string) {
	if s.onCommit != nil {
		s.onCommit(dirs)
	}
}

// RootDone finishes source i: scan.Run's roots are ScanTask's paths, which are the job's sources.
func (s *scanReporter) RootDone(i int) {
	if s.rootDone != nil {
		s.rootDone(i)
	}
	s.r.FinishSource(i)
}

// finish flushes the dropped-warning count and, for a scan that completed its hashing, the
// committed counts.
func (s *scanReporter) finish(stats scan.Stats) {
	s.w.flush()
	if stats.Interrupted {
		return
	}
	s.mu.Lock()
	s.p.CurrentPath = ""
	s.mu.Unlock()
	s.Hashed(stats)
}

// compile-time check: scanReporter is a scan.Reporter, a scan.PhaseReporter, a scan.RootReporter, a
// scan.FileReporter and a scan.CommitReporter.
var (
	_ scan.Reporter       = (*scanReporter)(nil)
	_ scan.PhaseReporter  = (*scanReporter)(nil)
	_ scan.RootReporter   = (*scanReporter)(nil)
	_ scan.FileReporter   = (*scanReporter)(nil)
	_ scan.CommitReporter = (*scanReporter)(nil)
)

// DifferencesTask returns the task job listing where the files under root (a local host path,
// resolved like the index records it) differ from the index: new files the scan rules include,
// changed ones (size or mtime) and recorded ones gone from disk, over the whole subtree (the same
// driftWalk as the directory state icon). The differences come sorted by path. done runs when the
// task ends, however it ends: with the differences, or with the error (ErrNotIndexed when root is
// not in the index, which does not fail the job; context.Canceled when it was canceled).
func (x *Index) DifferencesTask(root string, done func([]Difference, error)) *jobs.Task {
	return &jobs.Task{
		Title:             "fileindex differences",
		Lease:             DifferencesLease,
		ReportsOwnOutcome: true,
		Run: func(ctx context.Context, r jobs.TaskReporter) error {
			out, err := x.differences(ctx, r, root)
			done(out, err)
			if errors.Is(err, ErrNotIndexed) {
				return nil
			}
			return err
		},
	}
}

// differences is DifferencesTask's work.
func (x *Index) differences(ctx context.Context, r jobs.TaskReporter, root string) ([]Difference, error) {
	d, err := x.DB()
	if err != nil {
		return nil, err
	}
	rules, _, err := x.scanSettings()
	if err != nil {
		return nil, err
	}
	host, err := IndexPath(root)
	if err != nil {
		return nil, err
	}
	var out []Difference
	walked := 0
	_, err = driftWalk(ctx, d, rules, host, 0, func(p string) {
		walked++
		r.Progress(jobs.TaskProgress{DoneFiles: walked, CurrentPath: filepath.Dir(p)})
	}, func(df Difference) bool {
		out = append(out, df)
		return true
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b Difference) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}
