package compare

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// WalkOptions configures recursive indexing under one compare root.
type WalkOptions struct {
	ShowHidden    bool
	Gitignore     *gitignore.Cache
	ShouldSkipDir diskusage.ShouldIgnoreFolder
	// SkipSymlinks, when true, ignores symlink entries entirely: they are not
	// indexed and symlink targets are not followed during the walk.
	SkipSymlinks bool
	// OnFile, when set, is called after each regular file is indexed (1-based count).
	OnFile func(walked int)
	// Only, when non-empty, limits the walk to these subtrees (slash-separated paths
	// relative to root); files outside them, including root-level files, are skipped.
	Only []string
	// OnSkip, when set, is called with the rel directory ("" = root) that lost an
	// entry the walk dropped (hidden, gitignored, ShouldSkipDir, symlink,
	// unreadable). Skips outside Only are not reported.
	OnSkip func(relDir string)
	// OnDir, when set, is called with the rel path of every directory the walk enters.
	OnDir func(rel string)
}

// walkScope reports whether rel lies in (or is) one of only, and whether it is a
// strict ancestor of one (a directory to pass through on the way there).
func walkScope(rel string, only []string) (inside, ancestor bool) {
	for _, o := range only {
		if rel == o || strings.HasPrefix(rel, o+"/") {
			return true, false
		}
		if strings.HasPrefix(o, rel+"/") {
			ancestor = true
		}
	}
	return false, ancestor
}

// WalkRoot indexes regular files under root (local paths only in phase 1).
func WalkRoot(ctx context.Context, root pathloc.Path, opts WalkOptions) ([]FileRecord, error) {
	if root.IsRemote() {
		return nil, errRemoteNotSupported
	}
	host, err := root.FilePath()
	if err != nil {
		return nil, err
	}
	host = filepath.Clean(host)
	info, err := os.Stat(host)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	listOpts := localfs.ListOptions{ShowHidden: opts.ShowHidden}
	if !opts.ShowHidden {
		matcher, matcherErr := localfs.MatcherForListing(false, opts.Gitignore, host)
		if matcherErr != nil {
			return nil, matcherErr
		}
		listOpts.Gitignore = matcher
	}

	var out []FileRecord
	walkErr := filepath.WalkDir(host, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if path == host {
			return nil
		}

		rel, relErr := filepath.Rel(host, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		skipped := func(relDir string) {
			if opts.OnSkip == nil {
				return
			}
			if inside, _ := walkScope(rel, opts.Only); len(opts.Only) == 0 || inside {
				opts.OnSkip(relDir)
			}
		}
		if err != nil {
			if d != nil && d.IsDir() {
				skipped(rel) // unreadable directory: its content is unknown
			} else {
				skipped(RelDir(rel))
			}
			return nil
		}

		name := d.Name()
		isDir := d.IsDir()
		if d.Type()&fs.ModeSymlink != 0 {
			if opts.SkipSymlinks {
				skipped(RelDir(rel))
				return nil
			}
			if info, statErr := os.Stat(path); statErr == nil {
				isDir = info.IsDir()
			}
		}
		if len(opts.Only) > 0 {
			inside, ancestor := walkScope(rel, opts.Only)
			switch {
			case inside:
			case isDir && ancestor:
				return nil
			case isDir:
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if isDir {
			if !localfs.EntryVisible(name, filepath.Dir(path), true, listOpts) ||
				(opts.ShouldSkipDir != nil && opts.ShouldSkipDir(path)) {
				skipped(RelDir(rel))
				return filepath.SkipDir
			}
			if opts.OnDir != nil {
				opts.OnDir(rel)
			}
			return nil
		}
		if !localfs.EntryVisible(name, filepath.Dir(path), false, listOpts) {
			skipped(RelDir(rel))
			return nil
		}

		var size, modTime int64
		if fi, infoErr := d.Info(); infoErr == nil {
			size, modTime = fi.Size(), fi.ModTime().UnixNano()
		}
		loc, locErr := pathloc.File(filepath.Clean(path))
		if locErr != nil {
			return nil
		}
		out = append(out, FileRecord{Abs: loc, Rel: rel, Size: size, ModTime: modTime})
		if opts.OnFile != nil {
			opts.OnFile(len(out))
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}
