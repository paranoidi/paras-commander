package ops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ValidateFlattenSource requires a non-empty directory-only source (selection or cursor).
// Mixed files and directories return a dedicated error message.
func ValidateFlattenSource(p *panel.State) ([]pathloc.Path, error) {
	source, err := ResolveSource(p)
	if err != nil {
		return nil, err
	}
	if len(source.Entries) == 0 {
		return nil, &Error{Op: "flatten", Text: "no entries to flatten"}
	}
	dirCount := CountDirectories(source.Entries)
	fileCount := len(source.Entries) - dirCount
	if fileCount > 0 && dirCount > 0 {
		return nil, &Error{Op: "flatten", Text: "cannot mix files and directories in selection"}
	}
	if dirCount == 0 {
		return nil, &Error{Op: "flatten", Text: "no directories selected"}
	}
	paths := SourcePaths(source)
	pruned := panel.PruneNestedPaths(paths)
	roots := make([]pathloc.Path, 0, len(pruned))
	for _, s := range pruned {
		loc, perr := pathloc.Parse(s)
		if perr != nil {
			return nil, &Error{Op: "flatten", Text: fmt.Sprintf("invalid path %q: %v", s, perr), Err: perr}
		}
		roots = append(roots, loc)
	}
	return roots, nil
}

// ValidateFlattenTarget rejects an empty root list, an invalid destination, and a destination
// inside one of the roots.
func ValidateFlattenTarget(roots []pathloc.Path, dest pathloc.Path) error {
	if len(roots) == 0 {
		return &Error{Op: "flatten", Text: "no directories to flatten"}
	}
	if dest.IsZero() {
		return &Error{Op: "flatten", Text: "invalid destination"}
	}
	for _, root := range roots {
		if destStrictlyUnderRoot(dest, root) {
			return &Error{Op: "flatten", Text: "destination cannot be inside a selected directory"}
		}
	}
	return nil
}

// ExecuteFlatten moves every file and symlink under roots (recursive) or every immediate child of
// each root (non-recursive) to dest/<name> in a single streaming pass: each entry is listed and
// moved via moveOne, so nothing is enumerated up front. Conflicts go through resolver; an entry whose
// destination is the entry itself is left alone. Non-recursive children that are directories whose
// destination equals a flatten root are expanded to their children (recursively while the collision
// persists). Files and symlinks whose destination equals a root (e.g. root/root) cannot be moved
// while the root exists; they are parked as dest/<name>.flatten (or <name>.1.flatten, ... when
// taken), the roots are removed when removeEmpty leaves them empty, and the parked items are renamed
// to their final names. If a final path still exists the parked item is left in place and an error
// names it. With removeEmpty, each directory is removed right after its children are processed when
// nothing remains in it, roots included.
func ExecuteFlatten(ctx context.Context, roots []pathloc.Path, dest pathloc.Path, recursive, removeEmpty bool, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	if err := ValidateFlattenTarget(roots, dest); err != nil {
		return 0, 0, err
	}
	destIsDir, err := destinationIsDir(ctx, dest)
	if err != nil {
		return 0, 0, err
	}
	w := &flattenWalk{
		roots: roots, dest: dest, destIsDir: destIsDir, recursive: recursive, removeEmpty: removeEmpty,
		moveRun: moveRun{opts: opts, throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait},
	}
	for _, root := range roots {
		if err := w.walk(ctx, root); err != nil {
			return w.doneFiles, w.doneBytes, err
		}
	}
	err = w.finishDeferred(ctx)
	return w.doneFiles, w.doneBytes, err
}

type flattenWalk struct {
	roots                  []pathloc.Path
	dest                   pathloc.Path
	destIsDir              bool
	recursive, removeEmpty bool
	moveRun
	doneFiles int
	doneBytes int64
	deferred  []pathloc.Path
}

// dstFor is the flatten destination of child; dest was resolved once, so no per-entry Stat.
func (w *flattenWalk) dstFor(child pathloc.Path) (pathloc.Path, error) {
	if !w.destIsDir {
		return w.dest, nil
	}
	return w.dest.Join(child.Base())
}

func (w *flattenWalk) isRoot(p pathloc.Path) bool {
	for _, root := range w.roots {
		if p.Equal(root) {
			return true
		}
	}
	return false
}

func (w *flattenWalk) walk(ctx context.Context, dir pathloc.Path) error {
	be, err := backendFor(dir)
	if err != nil {
		return err
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		return &Error{Op: "flatten", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		dst, err := w.dstFor(e.Loc)
		if err != nil {
			return err
		}
		collides := w.isRoot(dst)
		switch {
		case e.Type == fsbackend.EntryDirectory && (w.recursive || collides):
			if err := w.walk(ctx, e.Loc); err != nil {
				return err
			}
		case collides:
			w.deferred = append(w.deferred, e.Loc)
		default:
			w.doneFiles, w.doneBytes, err = w.moveAndCount(ctx, e.Loc, dst, w.dest, w.doneFiles, w.doneBytes)
			if err != nil {
				return err
			}
		}
	}
	if w.removeEmpty {
		_, err = removeDirIfEmpty(ctx, be, dir, "flatten")
	}
	return err
}

// removeDirIfEmpty removes dir when it holds nothing but dot entries. A directory that no longer
// exists counts as removed. op labels returned errors. Callers have usually just emptied dir, so it
// tries the (non-recursive) Remove first and lists dir only when that fails, to tell "not empty"
// (false, nil) from a real error without parsing backend-specific error codes.
func removeDirIfEmpty(ctx context.Context, be fsbackend.Backend, dir pathloc.Path, op string) (bool, error) {
	removeErr := be.Remove(ctx, dir)
	if removeErr == nil || isNotExist(removeErr) {
		return true, nil
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		if isNotExist(err) {
			return true, nil
		}
		return false, &Error{Op: op, Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	if !dirHasOnlyDotEntries(entries) {
		return false, nil
	}
	return false, &Error{Op: op, Text: fmt.Sprintf("remove empty directory %q: %v", dir, removeErr), Err: removeErr}
}

// finishDeferred parks each deferred item as dest/<name>.flatten, removes the directories that
// held them when they are now empty (up to and including the root), and renames the parked items to
// their final names.
func (w *flattenWalk) finishDeferred(ctx context.Context) error {
	type parked struct{ temp, final pathloc.Path }
	items := make([]parked, 0, len(w.deferred))
	for _, src := range w.deferred {
		be, err := backendFor(src)
		if err != nil {
			return err
		}
		final, err := ResolveDestinationCtx(ctx, src, w.dest)
		if err != nil {
			return err
		}
		temp, err := freeFlattenTemp(ctx, be, final)
		if err != nil {
			return err
		}
		if err := be.Rename(ctx, src, temp); err != nil {
			return &Error{Op: "flatten", Text: fmt.Sprintf("move %q aside: %v", src, err), Err: err}
		}
		items = append(items, parked{temp, final})
	}
	if w.removeEmpty {
		for _, src := range w.deferred {
			for dir := src.Parent(); ; dir = dir.Parent() {
				be, err := backendFor(dir)
				if err != nil {
					return err
				}
				removed, err := removeDirIfEmpty(ctx, be, dir, "flatten")
				if err != nil {
					return err
				}
				if !removed || w.isRoot(dir) {
					break
				}
			}
		}
	}
	for _, it := range items {
		be, err := backendFor(it.temp)
		if err != nil {
			return err
		}
		if _, err := be.Stat(ctx, it.final); err == nil {
			return &Error{Op: "flatten", Text: fmt.Sprintf("%q still exists; item left at %q", it.final, it.temp)}
		}
		if err := be.Rename(ctx, it.temp, it.final); err != nil {
			return &Error{Op: "flatten", Text: fmt.Sprintf("rename %q: %v", it.temp, err), Err: err}
		}
		w.doneFiles++
	}
	return nil
}

func destStrictlyUnderRoot(dest, root pathloc.Path) bool {
	if dest.Scheme() != root.Scheme() {
		return false
	}
	if dest.Equal(root) {
		return false
	}
	return dest.HasPrefix(root)
}

// freeFlattenTemp returns the first of <name>.flatten, <name>.1.flatten, ... that does not exist.
func freeFlattenTemp(ctx context.Context, be fsbackend.Backend, final pathloc.Path) (pathloc.Path, error) {
	for i := 0; ; i++ {
		name := final.Base() + ".flatten"
		if i > 0 {
			name = fmt.Sprintf("%s.%d.flatten", final.Base(), i)
		}
		temp, err := final.Parent().Join(name)
		if err != nil {
			return temp, err
		}
		if _, err := be.Stat(ctx, temp); err != nil {
			return temp, nil
		}
	}
}

// RemoveEmptyDirsUnder removes empty directories under each root (depth-first), including roots when empty.
func RemoveEmptyDirsUnder(ctx context.Context, roots []pathloc.Path) error {
	for _, root := range roots {
		if err := removeEmptyDirsPostOrder(ctx, root); err != nil {
			return err
		}
	}
	return nil
}

func removeEmptyDirsPostOrder(ctx context.Context, dir pathloc.Path) error {
	be, err := backendFor(dir)
	if err != nil {
		return err
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		return &Error{Op: "flatten", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		if e.Type != fsbackend.EntryDirectory {
			continue
		}
		if err := removeEmptyDirsPostOrder(ctx, e.Loc); err != nil {
			return err
		}
	}
	_, err = removeDirIfEmpty(ctx, be, dir, "flatten")
	return err
}

// PreviewEmptyDirsUnder dry-runs RemoveEmptyDirsUnder: it reports which
// directories under each root would be removed if the paths in removed were
// deleted first, without touching the filesystem. removed keys are
// pathloc.Path.String() values of files about to be deleted.
func PreviewEmptyDirsUnder(ctx context.Context, roots []pathloc.Path, removed map[string]bool) ([]pathloc.Path, error) {
	var out []pathloc.Path
	for _, root := range roots {
		if _, err := previewEmptyDirsPostOrder(ctx, root, removed, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func previewEmptyDirsPostOrder(ctx context.Context, dir pathloc.Path, removed map[string]bool, out *[]pathloc.Path) (bool, error) {
	be, err := backendFor(dir)
	if err != nil {
		return false, err
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		return false, &Error{Op: "flatten", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	empty := true
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		if e.Type == fsbackend.EntryDirectory {
			childEmpty, err := previewEmptyDirsPostOrder(ctx, e.Loc, removed, out)
			if err != nil {
				return false, err
			}
			if !childEmpty {
				empty = false
			}
			continue
		}
		if !removed[e.Loc.String()] {
			empty = false
		}
	}
	if empty {
		*out = append(*out, dir)
	}
	return empty, nil
}

func dirHasOnlyDotEntries(entries []fsbackend.Entry) bool {
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		return false
	}
	return true
}

// DanglingDirsAfter reports directories left empty by removing sources (e.g. after a
// move or delete job completes): starting from each source's parent, it climbs upward
// while every remaining entry in the directory is itself a qualifying (already-empty
// or fully-emptied-below) directory, stopping at the first ancestor with any other
// content or at the filesystem root. Only the topmost directory of each emptied chain
// is returned (removing it recursively covers the rest and avoids re-prompting for the
// children next time). Nonexistent parents (already removed as part of the operation)
// are skipped rather than treated as an error; unexpected listing failures on parents
// that do exist are still returned, matching RemoveEmptyDirsUnder's error style.
func DanglingDirsAfter(ctx context.Context, sources []pathloc.Path) ([]pathloc.Path, error) {
	candidates := make(map[string]bool)
	var order []pathloc.Path
	for _, parent := range uniqueParentDirs(sources) {
		if err := climbDanglingChain(ctx, parent, candidates, &order); err != nil {
			return nil, err
		}
	}
	var out []pathloc.Path
	for _, c := range order {
		if !candidates[c.Parent().String()] {
			out = append(out, c)
		}
	}
	return out, nil
}

// uniqueParentDirs returns each path's parent directory, first-seen order, deduplicated.
func uniqueParentDirs(paths []pathloc.Path) []pathloc.Path {
	seen := make(map[string]bool, len(paths))
	out := make([]pathloc.Path, 0, len(paths))
	for _, p := range paths {
		parent := p.Parent()
		key := parent.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, parent)
	}
	return out
}

// climbDanglingChain walks dir upward, marking it and each qualifying ancestor as a
// candidate, until an ancestor has non-candidate content, the chain merges into an
// already-processed candidate, or dir is the filesystem root (Parent() == dir).
func climbDanglingChain(ctx context.Context, dir pathloc.Path, candidates map[string]bool, order *[]pathloc.Path) error {
	for {
		if candidates[dir.String()] {
			return nil
		}
		be, err := backendFor(dir)
		if err != nil {
			return nil
		}
		entries, err := be.List(ctx, dir)
		if err != nil {
			// errors.Is, not os.IsNotExist: localfs.ListDir wraps the cause with %w.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return &Error{Op: "dangling-dirs", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
		}
		for _, e := range entries {
			if e.Name == "." || e.Name == ".." {
				continue
			}
			if e.Type != fsbackend.EntryDirectory || !candidates[e.Loc.String()] {
				return nil
			}
		}
		candidates[dir.String()] = true
		*order = append(*order, dir)
		parent := dir.Parent()
		if parent.Equal(dir) {
			return nil
		}
		dir = parent
	}
}
