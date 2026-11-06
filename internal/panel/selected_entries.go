package panel

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// SelectedEntries returns selected entries in stable sorted path order.
// Listing hits use listingByPath; off-listing paths call resolve (required when
// any selection is off-listing). When skipMissing is true, resolve errors are skipped.
func (s *State) SelectedEntries(skipMissing bool, resolve func(path string) (localfs.Entry, error)) ([]localfs.Entry, error) {
	if len(s.SelectedPaths) == 0 {
		return nil, nil
	}
	if resolve == nil {
		resolve = localfs.EntryFromPath
	}
	paths := make([]string, 0, len(s.SelectedPaths))
	for path, on := range s.SelectedPaths {
		if !on {
			continue
		}
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	entries := make([]localfs.Entry, 0, len(paths))
	for _, path := range paths {
		if e, ok := s.listingEntry(path); ok {
			entries = append(entries, e)
			continue
		}
		e, err := resolve(path)
		if err != nil {
			if skipMissing {
				continue
			}
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// CommandTags returns the tagged paths a command's %t (%T for the other panel) expands to, and
// the `t t` condition tests: the tags directly in the panel directory, plus, in a tree layout,
// the ones in any expanded folder under it. A fileindex:// tag (a ghost row, or a row of an
// index panel) belongs when its host path is there, and stays a fileindex:// value, so callers
// can refuse it. Tags elsewhere (another directory) are not included. The order is unspecified.
func (s *State) CommandTags() []string {
	root := filepath.Clean(s.PathString())
	tree := s.ListLayout == ListLayoutTree
	// indexRoot is the host directory a fileindex:// tag is compared with.
	indexRoot := ""
	if s.Path.IsLocal() {
		indexRoot = root
	} else if host, err := pathloc.IndexHostPath(s.Path); err == nil {
		indexRoot = host
	}
	var out []string
	for p, on := range s.SelectedPaths {
		if !on {
			continue
		}
		if strings.HasPrefix(p, pathloc.FileIndexPrefix) {
			loc, err := pathloc.Parse(p)
			if err != nil {
				continue
			}
			host, err := pathloc.IndexHostPath(loc)
			if err == nil && indexRoot != "" && (filepath.Dir(host) == indexRoot || tree && underDir(indexRoot, host)) {
				out = append(out, p)
			}
			continue
		}
		if filepath.Clean(filepath.Dir(p)) == root || tree && underDir(root, p) {
			out = append(out, p)
		}
	}
	return out
}

func underDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, "../")
}
