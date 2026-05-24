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
// When recursive is false, immediate children of each root are returned.
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
			part, err = collectImmediateFlattenChildren(ctx, root)
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

func collectImmediateFlattenChildren(ctx context.Context, dir pathloc.Path) ([]string, error) {
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
