package ops

import (
	"context"
	"fmt"
	"sort"

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

// CollectFlattenSources lists move sources for flatten into dest.
// When recursive is false, immediate children of each root are returned; child directories
// whose resolved destination equals a flatten root are expanded to their immediate children
// (recursively while the collision persists) so same-name nesting does not move onto the root.
// When recursive is true, every file and symlink under each root is returned (directories are not move roots).
func CollectFlattenSources(ctx context.Context, roots []pathloc.Path, dest pathloc.Path, recursive bool) ([]string, error) {
	if len(roots) == 0 {
		return nil, &Error{Op: "flatten", Text: "no directories to flatten"}
	}
	if dest.IsZero() {
		return nil, &Error{Op: "flatten", Text: "invalid destination"}
	}
	for _, root := range roots {
		if destStrictlyUnderRoot(dest, root) {
			return nil, &Error{Op: "flatten", Text: "destination cannot be inside a selected directory"}
		}
	}
	out := make([]string, 0)
	for _, root := range roots {
		var part []string
		var err error
		if recursive {
			part, err = collectRecursiveFlattenFiles(ctx, root)
		} else {
			part, err = collectNonRecursiveFlattenSources(ctx, root, dest, roots)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	if len(out) == 0 {
		return nil, nil
	}
	sort.Strings(out)
	return dedupeSortedStrings(out), nil
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

func collectNonRecursiveFlattenSources(ctx context.Context, dir, dest pathloc.Path, roots []pathloc.Path) ([]string, error) {
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
		if e.Type == fsbackend.EntryDirectory {
			collides, err := flattenDestEqualsRoot(ctx, e.Loc, dest, roots)
			if err != nil {
				return nil, err
			}
			if collides {
				part, err := collectNonRecursiveFlattenSources(ctx, e.Loc, dest, roots)
				if err != nil {
					return nil, err
				}
				out = append(out, part...)
				continue
			}
		}
		out = append(out, e.Loc.String())
	}
	return out, nil
}

func collectRecursiveFlattenFiles(ctx context.Context, dir pathloc.Path) ([]string, error) {
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
			part, err := collectRecursiveFlattenFiles(ctx, e.Loc)
			if err != nil {
				return nil, err
			}
			out = append(out, part...)
		case fsbackend.EntryFile, fsbackend.EntrySymlink:
			out = append(out, e.Loc.String())
		default:
			out = append(out, e.Loc.String())
		}
	}
	return out, nil
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
