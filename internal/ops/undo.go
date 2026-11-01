package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// UndoKind is what a journaled step did, and so how to reverse it.
type UndoKind int

const (
	// UndoRemoveCreated: a copy created Path (a file, symlink or new directory).
	UndoRemoveCreated UndoKind = iota
	// UndoMoveBack: From was moved (or renamed) to Path.
	UndoMoveBack
)

// UndoStep is one reversible effect of a copy, move or rename. Size and ModTime describe the
// created file as it was right after the write; ExecuteUndo compares them before removing it.
type UndoStep struct {
	Kind    UndoKind
	Path    pathloc.Path
	From    pathloc.Path
	IsDir   bool
	Size    int64
	ModTime time.Time
	// Source is the index of the original job source the step belongs to.
	Source int32
}

// noteCreated reports a path a copy just created through opts.OnJournal and ctx's fschange sink.
// The one Lstat of the destination supplies the size and mtime the undo check needs; a failed
// stat drops the step.
func (o Options) noteCreated(ctx context.Context, dst pathloc.Path) {
	host := hostPathOf(dst)
	report := host != "" && fschange.Enabled(ctx)
	if o.OnJournal == nil && !report {
		return
	}
	ent, err := statEntry(ctx, dst)
	if err != nil {
		return
	}
	isDir := ent.Type == fsbackend.EntryDirectory
	if report {
		c := fschange.Change{Kind: fschange.Created, Path: host, IsDir: isDir}
		if !isDir {
			c.Bytes = ent.Size
		}
		fschange.Report(ctx, c)
	}
	if o.OnJournal != nil {
		o.OnJournal(UndoStep{Kind: UndoRemoveCreated, Path: dst, IsDir: isDir, Size: ent.Size, ModTime: ent.ModifiedAt, Source: o.source})
	}
}

// journalMoved reports that from now lives at to, through opts.OnJournal.
func (o Options) journalMoved(ctx context.Context, from, to pathloc.Path) {
	if o.OnJournal == nil {
		return
	}
	ent, err := statEntry(ctx, to)
	if err != nil {
		return
	}
	o.OnJournal(UndoStep{Kind: UndoMoveBack, Path: to, From: from, IsDir: ent.Type == fsbackend.EntryDirectory, Source: o.source})
}

// ExecuteUndo reverses steps in reverse order, checking each one first so it never touches
// something that changed since the original run. A step that fails its check is reported through
// opts.OnItemError and skipped (without the callback the first failure aborts). Moves back never
// overwrite. It returns the number of steps reversed.
func ExecuteUndo(ctx context.Context, steps []UndoStep, opts Options, progress ProgressCallback) (int, int64, error) {
	opts.OnJournal = nil
	fail := func(s UndoStep, err error) error {
		if opts.OnItemError == nil {
			return err
		}
		opts.OnItemError(PlanItem{Src: s.Path, Dst: s.From, IsDir: s.IsDir, Source: s.Source}, err)
		return nil
	}
	if rows, ok := undoRenameBatch(steps); ok {
		return undoMassRename(ctx, rows, steps, fail, progress, opts)
	}
	r := moveRun{opts: opts, progress: progress, resolver: func(context.Context, string, string, FileConflictFacts) (ConflictResolution, error) {
		return ConflictResolution{Action: ActionSkip}, nil
	}}
	var done int
	var bytes int64
	for i := len(steps) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return done, bytes, err
		}
		s := steps[i]
		opts.enterSource(s.Source)
		r.opts = opts
		var err error
		switch s.Kind {
		case UndoMoveBack:
			done, bytes, err = undoMoveBack(ctx, r, s, done, bytes)
		default:
			err = undoRemoveCreated(ctx, s)
			if err == nil {
				opts.itemDone(PlanItem{Src: s.Path, IsDir: s.IsDir}, ItemDone, "")
				done++
				if progress != nil {
					progress(s.Path.String(), "", done, bytes)
				}
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return done, bytes, ctx.Err()
			}
			if ferr := fail(s, err); ferr != nil {
				return done, bytes, ferr
			}
		}
	}
	return done, bytes, nil
}

// undoRemoveCreated removes a created item when it still looks like the one the copy wrote: same
// kind, and for non-directories the same size and mtime. Directories go only when empty.
func undoRemoveCreated(ctx context.Context, s UndoStep) error {
	ent, err := statEntry(ctx, s.Path)
	if err != nil {
		if isNotExist(err) {
			return fmt.Errorf("already gone")
		}
		return err
	}
	if isDir := ent.Type == fsbackend.EntryDirectory; isDir != s.IsDir {
		return fmt.Errorf("changed since the copy (different kind)")
	}
	if !s.IsDir && (ent.Size != s.Size || !ent.ModifiedAt.Equal(s.ModTime)) {
		return fmt.Errorf("changed since the copy")
	}
	be, err := backendFor(s.Path)
	if err != nil {
		return err
	}
	if s.IsDir {
		removed, err := removeDirIfEmpty(ctx, be, s.Path, "undo")
		if err != nil {
			return err
		}
		if !removed {
			return fmt.Errorf("directory is not empty, kept")
		}
		return nil
	}
	if err := be.Remove(ctx, s.Path); err != nil {
		return err
	}
	reportRemoved(ctx, s.Path, false)
	return nil
}

// undoMoveBack moves s.Path back to s.From through the move path (copy fallback across devices
// included) when the item is still there and the original location is free.
func undoMoveBack(ctx context.Context, r moveRun, s UndoStep, done int, bytes int64) (int, int64, error) {
	ent, err := statEntry(ctx, s.Path)
	if err != nil {
		if isNotExist(err) {
			return done, bytes, fmt.Errorf("no longer at %s", s.Path)
		}
		return done, bytes, err
	}
	if (ent.Type == fsbackend.EntryDirectory) != s.IsDir {
		return done, bytes, fmt.Errorf("changed since the move (different kind)")
	}
	if _, err := statEntry(ctx, s.From); err == nil {
		return done, bytes, fmt.Errorf("original location %s is occupied", s.From)
	} else if !isNotExist(err) {
		return done, bytes, err
	}
	if err := ensureParentDirs(ctx, s.From); err != nil {
		return done, bytes, err
	}
	// destination == dst makes a copy fallback land exactly on From, whatever its name.
	return r.moveAndCount(ctx, s.Path, s.From, s.From, done, bytes)
}

// undoRenameBatch reports whether steps are two or more local renames within their directories,
// the shape of a mass rename, and returns the reverse rename rows.
func undoRenameBatch(steps []UndoStep) ([]MassRenameRow, bool) {
	if len(steps) < 2 {
		return nil, false
	}
	rows := make([]MassRenameRow, 0, len(steps))
	for _, s := range steps {
		if s.Kind != UndoMoveBack || !s.Path.IsLocal() || !s.From.IsLocal() {
			return nil, false
		}
		from, err1 := s.Path.FilePath()
		to, err2 := s.From.FilePath()
		if err1 != nil || err2 != nil || filepath.Dir(from) != filepath.Dir(to) {
			return nil, false
		}
		rows = append(rows, MassRenameRow{SourcePath: from, OldBase: filepath.Base(from), NewBase: filepath.Base(to), IsDir: s.IsDir})
	}
	return rows, true
}

// undoMassRename reverses a rename batch in one two-phase ExecuteMassRename so swaps and chains
// work. A row is dropped (and reported) when its item is gone or changed kind, or when its target
// name is held by something the batch does not itself vacate; dropping a row can vacate less, so
// the check repeats until it settles.
func undoMassRename(ctx context.Context, rows []MassRenameRow, steps []UndoStep, fail func(UndoStep, error) error, progress ProgressCallback, opts Options) (int, int64, error) {
	keep := make([]bool, len(rows))
	reasons := make([]error, len(rows))
	for i, row := range rows {
		keep[i] = true
		fi, err := lstatKind(row.SourcePath)
		switch {
		case err != nil:
			keep[i], reasons[i] = false, fmt.Errorf("no longer at %s", row.SourcePath)
		case fi != row.IsDir:
			keep[i], reasons[i] = false, fmt.Errorf("changed since the rename (different kind)")
		}
	}
	for changed := true; changed; {
		changed = false
		vacated := map[string]bool{}
		for i, row := range rows {
			if keep[i] {
				vacated[row.SourcePath] = true
			}
		}
		for i, row := range rows {
			if !keep[i] || vacated[row.NewPath()] {
				continue
			}
			if _, err := lstatKind(row.NewPath()); err == nil {
				keep[i], reasons[i], changed = false, fmt.Errorf("original location %s is occupied", row.NewPath()), true
			}
		}
	}
	var work []MassRenameRow
	var workSources []int32
	for i, row := range rows {
		if keep[i] {
			work = append(work, row)
			workSources = append(workSources, steps[i].Source)
			continue
		}
		if err := fail(steps[i], reasons[i]); err != nil {
			return 0, 0, err
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}
	if err := ExecuteMassRename(ctx, work); err != nil {
		return 0, 0, err
	}
	for i, row := range work {
		src, serr := pathloc.Parse(row.SourcePath)
		dst, derr := pathloc.Parse(row.NewPath())
		if serr == nil && derr == nil {
			opts.enterSource(workSources[i])
			opts.itemDone(PlanItem{Src: src, Dst: dst, IsDir: row.IsDir}, ItemDone, "")
		}
	}
	if progress != nil && len(work) > 0 {
		progress(work[len(work)-1].SourcePath, work[len(work)-1].NewPath(), len(work), 0)
	}
	return len(work), 0, nil
}

// lstatKind Lstats a local path and reports whether it is a directory.
func lstatKind(path string) (isDir bool, err error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}
