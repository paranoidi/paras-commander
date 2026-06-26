package compare

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

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
		if err != nil {
			return nil
		}
		if path == host {
			return nil
		}

		name := d.Name()
		isDir := d.IsDir()
		if d.Type()&fs.ModeSymlink != 0 {
			if info, statErr := os.Stat(path); statErr == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			if !localfs.EntryVisible(name, filepath.Dir(path), true, listOpts) {
				return filepath.SkipDir
			}
			if opts.ShouldSkipDir != nil && opts.ShouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !localfs.EntryVisible(name, filepath.Dir(path), false, listOpts) {
			return nil
		}

		rel, relErr := filepath.Rel(host, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		var size int64
		if fi, infoErr := d.Info(); infoErr == nil {
			size = fi.Size()
		}
		loc, locErr := pathloc.File(filepath.Clean(path))
		if locErr != nil {
			return nil
		}
		out = append(out, FileRecord{Abs: loc, Rel: rel, Size: size})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}
