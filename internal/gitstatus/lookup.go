package gitstatus

import (
	"path/filepath"
	"strings"
)

// snapshot holds parsed porcelain for one work tree query.
type snapshot struct {
	entries []entry
}

func (sn *snapshot) cellForPath(absPath string, isDir bool) Cell {
	absPath = filepath.Clean(absPath)
	if isDir {
		return sn.dirCell(absPath)
	}
	return sn.fileCell(absPath)
}

func (sn *snapshot) fileCell(path string) Cell {
	var staged, unstaged Status
	for _, e := range sn.entries {
		if e.unstaged == Ignored || e.staged == Ignored {
			if strings.HasPrefix(path, e.path+string(filepath.Separator)) || path == e.path {
				staged = combineStatus(staged, Ignored)
				unstaged = combineStatus(unstaged, Ignored)
			}
			continue
		}
		// Git collapses a dir with no tracked files into one "?? dir/" entry, so a file under
		// such an ancestor is untracked too.
		if e.path == path || (e.staged == NotModified && e.unstaged == New && strings.HasPrefix(path, e.path+string(filepath.Separator))) {
			staged = combineStatus(staged, e.staged)
			unstaged = combineStatus(unstaged, e.unstaged)
		}
	}
	if staged == NotModified && unstaged == NotModified {
		return Cell{Staged: NotModified, Unstaged: NotModified}
	}
	return Cell{Staged: staged, Unstaged: unstaged}
}

func (sn *snapshot) dirCell(dir string) Cell {
	var staged, unstaged Status
	var hasStaged, hasUnstaged, hasUntracked, noTracked bool
	sep := string(filepath.Separator)
	for _, e := range sn.entries {
		untracked := e.staged == NotModified && e.unstaged == New
		if e.unstaged == Ignored || e.staged == Ignored {
			if strings.HasPrefix(dir, e.path+sep) || dir == e.path {
				noTracked = true
				staged = combineStatus(staged, Ignored)
				unstaged = combineStatus(unstaged, Ignored)
			}
			continue
		}
		if untracked && strings.HasPrefix(dir, e.path+sep) {
			noTracked, hasUntracked = true, true
		}
		if strings.HasPrefix(e.path, dir+sep) || e.path == dir {
			staged = combineStatus(staged, e.staged)
			unstaged = combineStatus(unstaged, e.unstaged)
			hasStaged = hasStaged || (e.staged != NotModified && e.staged != Ignored)
			hasUnstaged = hasUnstaged || (e.unstaged != NotModified && e.unstaged != New && e.unstaged != Ignored)
			hasUntracked = hasUntracked || untracked
			noTracked = noTracked || (untracked && e.path == dir)
		}
	}
	return Cell{Staged: staged, Unstaged: unstaged, HasStaged: hasStaged, HasUnstaged: hasUnstaged, HasUntracked: hasUntracked, NoTracked: noTracked}
}

// MapForListing builds per-entry Git cells for visible listing paths.
func MapForListing(sn *snapshot, paths []ListingPaths) map[string]Cell {
	if sn == nil {
		return nil
	}
	out := make(map[string]Cell, len(paths))
	for _, p := range paths {
		out[p.AbsPath] = sn.cellForPath(p.AbsPath, p.IsDir)
	}
	return out
}
