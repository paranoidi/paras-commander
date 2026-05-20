package localfs

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/gitignore"
)

// EntryVisible reports whether a directory entry should appear in listings and find walks.
func EntryVisible(name, parentAbs string, isDir bool, opts ListOptions) bool {
	if opts.ShowHidden {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return false
	}
	if opts.Gitignore != nil {
		abs := filepath.Join(parentAbs, name)
		if opts.Gitignore.Ignored(abs, isDir) {
			return false
		}
	}
	return true
}

// MatcherForListing returns a gitignore matcher when hidden files are hidden and cache is set.
func MatcherForListing(showHidden bool, cache *gitignore.Cache, listDir string) (*gitignore.Matcher, error) {
	if showHidden || cache == nil {
		return nil, nil
	}
	return cache.MatcherForDir(listDir)
}
