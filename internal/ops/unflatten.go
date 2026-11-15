package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// UnflattenStem returns base without its last extension, and false when base has none: no dot,
// or only a leading dot (a dotfile such as ".bashrc"). "photo.tar.gz" gives "photo.tar".
func UnflattenStem(base string) (string, bool) {
	i := strings.LastIndexByte(base, '.')
	if i <= 0 {
		return "", false
	}
	return base[:i], true
}

// UnflattenDir is the directory an unflatten moves p into: parent/<stem>. False when p's name has
// no extension (the directory would take the file's own name).
func UnflattenDir(p pathloc.Path) (pathloc.Path, bool) {
	stem, ok := UnflattenStem(p.Base())
	if !ok {
		return pathloc.Path{}, false
	}
	dir, err := p.Parent().Join(stem)
	if err != nil {
		return pathloc.Path{}, false
	}
	return dir, true
}

// ValidateUnflattenSource requires a non-empty files-only source (selection or cursor) where every
// file has an extension. Any directory in the source, a fileindex:// panel, or a file without an
// extension is refused.
func ValidateUnflattenSource(p *panel.State) ([]pathloc.Path, error) {
	source, err := ResolveSource(p)
	if err != nil {
		return nil, err
	}
	if len(source.Entries) == 0 {
		return nil, &Error{Op: "unflatten", Text: "no files to unflatten"}
	}
	dirCount := CountDirectories(source.Entries)
	if dirCount == len(source.Entries) {
		return nil, &Error{Op: "unflatten", Text: "unflatten works on files only"}
	}
	if dirCount > 0 {
		return nil, &Error{Op: "unflatten", Text: "cannot mix files and directories in selection"}
	}
	paths := SourcePaths(source)
	out := make([]pathloc.Path, 0, len(paths))
	for _, s := range paths {
		loc, perr := pathloc.Parse(s)
		if perr != nil {
			return nil, &Error{Op: "unflatten", Text: fmt.Sprintf("invalid path %q: %v", s, perr), Err: perr}
		}
		if loc.Scheme() == pathloc.SchemeFileIndex {
			return nil, &Error{Op: "unflatten", Text: UnflattenIndexRefusal}
		}
		if _, ok := UnflattenDir(loc); !ok {
			return nil, &Error{Op: "unflatten", Text: fmt.Sprintf("%s has no extension", loc.Base())}
		}
		out = append(out, loc)
	}
	return out, nil
}

// ExecuteUnflatten moves each source file into parent/<stem> (see UnflattenDir), creating that
// directory when missing and reusing it when it already is a directory. A name clash inside it goes
// through resolver; a non-directory already at parent/<stem> fails the job naming it.
func ExecuteUnflatten(ctx context.Context, sources []pathloc.Path, opts Options, throttle ProgressEmitThrottle, progress ProgressCallback, resolver ConflictResolver, diskWait DiskWaitFunc) (int, int64, error) {
	r := moveRun{opts: opts, throttle: throttle, progress: progress, resolver: resolver, diskWait: diskWait}
	doneFiles, doneBytes := 0, int64(0)
	for i, src := range sources {
		if err := itemBoundaryErr(ctx); err != nil {
			return doneFiles, doneBytes, err
		}
		r.opts.enterSource(int32(i))
		if src.Scheme() == pathloc.SchemeFileIndex {
			return doneFiles, doneBytes, &Error{Op: "unflatten", Text: UnflattenIndexRefusal}
		}
		dir, ok := UnflattenDir(src)
		if !ok {
			return doneFiles, doneBytes, &Error{Op: "unflatten", Text: fmt.Sprintf("%s has no extension", src.Base())}
		}
		if err := ensureUnflattenDir(ctx, dir); err != nil {
			return doneFiles, doneBytes, err
		}
		dst, err := dir.Join(src.Base())
		if err != nil {
			return doneFiles, doneBytes, err
		}
		doneFiles, doneBytes, err = r.moveAndCount(ctx, src, dst, dir, doneFiles, doneBytes)
		if err != nil {
			return doneFiles, doneBytes, err
		}
	}
	return doneFiles, doneBytes, nil
}

// ensureUnflattenDir creates dir when nothing is there and accepts an existing directory (a
// symlink to one included); anything else is an error naming dir.
func ensureUnflattenDir(ctx context.Context, dir pathloc.Path) error {
	ent, err := statEntryFollow(ctx, dir)
	switch {
	case err == nil && ent.Type == fsbackend.EntryDirectory:
		return nil
	case err == nil:
		return &Error{Op: "unflatten", Text: fmt.Sprintf("%q exists and is not a directory", dir.String())}
	case !isNotExist(err):
		return &Error{Op: "unflatten", Text: fmt.Sprintf("stat %q: %v", dir.String(), err), Err: err}
	}
	be, err := backendFor(dir)
	if err != nil {
		return err
	}
	if err := be.Mkdir(ctx, dir, 0o755); err != nil {
		return &Error{Op: "unflatten", Text: fmt.Sprintf("mkdir %q: %v", dir.String(), err), Err: err}
	}
	reportCreated(ctx, dir)
	return nil
}
