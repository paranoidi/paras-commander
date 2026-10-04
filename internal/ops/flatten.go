package ops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

var (
	collectFlattenTestHookMu sync.Mutex
	collectFlattenTestHook   func(context.Context)
)

// SetCollectFlattenTestHook installs a test-only callback invoked at the start of
// CollectFlattenSources so tests can block or cancel while the walk is planned.
func SetCollectFlattenTestHook(fn func(context.Context)) {
	collectFlattenTestHookMu.Lock()
	collectFlattenTestHook = fn
	collectFlattenTestHookMu.Unlock()
}

func runCollectFlattenTestHook(ctx context.Context) {
	collectFlattenTestHookMu.Lock()
	fn := collectFlattenTestHook
	collectFlattenTestHookMu.Unlock()
	if fn != nil {
		fn(ctx)
	}
}

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

// CollectFlattenSources lists move sources for flatten into dest.
// When recursive is false, immediate children of each root are returned; child directories
// whose resolved destination equals a flatten root are expanded to their immediate children
// (recursively while the collision persists) so same-name nesting does not move onto the root.
// When recursive is true, every file and symlink under each root is returned (directories are not move roots).
// Files and symlinks whose destination equals a flatten root (e.g. root/root) cannot be moved while
// the root exists; they are returned in deferred (not in sources) for FinishFlattenDeferred.
func CollectFlattenSources(ctx context.Context, roots []pathloc.Path, dest pathloc.Path, recursive bool) (sources, deferred []string, err error) {
	if len(roots) == 0 {
		return nil, nil, &Error{Op: "flatten", Text: "no directories to flatten"}
	}
	if dest.IsZero() {
		return nil, nil, &Error{Op: "flatten", Text: "invalid destination"}
	}
	runCollectFlattenTestHook(ctx)
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, root := range roots {
		if destStrictlyUnderRoot(dest, root) {
			return nil, nil, &Error{Op: "flatten", Text: "destination cannot be inside a selected directory"}
		}
	}
	out := make([]string, 0)
	var def []string
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var part []string
		var err error
		if recursive {
			part, err = collectRecursiveFlattenFiles(ctx, root, dest, roots, &def)
		} else {
			part, err = collectNonRecursiveFlattenSources(ctx, root, dest, roots, &def)
		}
		if err != nil {
			return nil, nil, err
		}
		out = append(out, part...)
	}
	sort.Strings(def)
	def = dedupeSortedStrings(def)
	if len(out) == 0 {
		return nil, def, nil
	}
	sort.Strings(out)
	return dedupeSortedStrings(out), def, nil
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

func flattenDestEqualsRoot(ctx context.Context, child, dest pathloc.Path, roots []pathloc.Path) (bool, error) {
	resolved, err := ResolveDestinationCtx(ctx, child, dest)
	if err != nil {
		return false, err
	}
	for _, root := range roots {
		if resolved.Equal(root) {
			return true, nil
		}
	}
	return false, nil
}

func collectNonRecursiveFlattenSources(ctx context.Context, dir, dest pathloc.Path, roots []pathloc.Path, deferred *[]string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	be, err := backendFor(dir)
	if err != nil {
		return nil, err
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		return nil, &Error{Op: "flatten", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		collides, err := flattenDestEqualsRoot(ctx, e.Loc, dest, roots)
		if err != nil {
			return nil, err
		}
		if collides {
			if e.Type != fsbackend.EntryDirectory {
				*deferred = append(*deferred, e.Loc.String())
				continue
			}
			part, err := collectNonRecursiveFlattenSources(ctx, e.Loc, dest, roots, deferred)
			if err != nil {
				return nil, err
			}
			out = append(out, part...)
			continue
		}
		out = append(out, e.Loc.String())
	}
	return out, nil
}

func collectRecursiveFlattenFiles(ctx context.Context, dir, dest pathloc.Path, roots []pathloc.Path, deferred *[]string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	be, err := backendFor(dir)
	if err != nil {
		return nil, err
	}
	entries, err := be.List(ctx, dir)
	if err != nil {
		return nil, &Error{Op: "flatten", Text: fmt.Sprintf("list %q: %v", dir, err), Err: err}
	}
	out := make([]string, 0)
	for _, e := range entries {
		if e.Name == "." || e.Name == ".." {
			continue
		}
		switch e.Type {
		case fsbackend.EntryDirectory:
			part, err := collectRecursiveFlattenFiles(ctx, e.Loc, dest, roots, deferred)
			if err != nil {
				return nil, err
			}
			out = append(out, part...)
		default:
			collides, err := flattenDestEqualsRoot(ctx, e.Loc, dest, roots)
			if err != nil {
				return nil, err
			}
			if collides {
				*deferred = append(*deferred, e.Loc.String())
			} else {
				out = append(out, e.Loc.String())
			}
		}
	}
	return out, nil
}

// FinishFlattenDeferred completes a flatten after the main move. Each deferred item (whose final
// name equals a flatten root) is parked as dest/<name>.flatten (or <name>.1.flatten, <name>.2.flatten,
// ... when taken; first free name wins), empty directories under
// roots are removed when removeEmpty is set, and the parked items are renamed to their final names.
// If a final path still exists (its root was not removed) the parked item is left in place and an
// error names it. With no deferred items it only removes empty directories.
func FinishFlattenDeferred(ctx context.Context, deferred []pathloc.Path, dest pathloc.Path, roots []pathloc.Path, removeEmpty bool) error {
	type parked struct{ temp, final pathloc.Path }
	var items []parked
	for _, src := range deferred {
		be, err := backendFor(src)
		if err != nil {
			return err
		}
		final, err := ResolveDestinationCtx(ctx, src, dest)
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
	if removeEmpty {
		if err := RemoveEmptyDirsUnder(ctx, roots); err != nil {
			return err
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
	}
	return nil
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
	entries, err = be.List(ctx, dir)
	if err != nil {
		return err
	}
	if dirHasOnlyDotEntries(entries) {
		if err := be.Remove(ctx, dir); err != nil {
			return &Error{Op: "flatten", Text: fmt.Sprintf("remove empty directory %q: %v", dir, err), Err: err}
		}
	}
	return nil
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

func dedupeSortedStrings(sorted []string) []string {
	if len(sorted) == 0 {
		return nil
	}
	out := sorted[:1]
	for i := 1; i < len(sorted); i++ {
		if sorted[i] != out[len(out)-1] {
			out = append(out, sorted[i])
		}
	}
	return out
}
