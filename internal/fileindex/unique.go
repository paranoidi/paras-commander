package fileindex

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/fileindex/db"
)

// UniqueResult is the outcome of UniqueFiles.
type UniqueResult struct {
	// Files are the listed files, sorted by path.
	Files []db.Found
	// Unverified holds the Files paths whose copies cannot be ruled out yet: not fully hashed, or
	// with a potential duplicate outside the scope.
	Unverified map[string]bool
	// NotIndexed are the scope directories the index does not know.
	NotIndexed []string
}

// UniqueFiles lists the indexed files under the scope directories that have no confirmed copy
// outside them. Copies inside the scope do not count, so a file copied only within it is listed
// with all its copies. A file is left out only on a full-hash match (for a JPEG, the metadata-free
// one the Copies column uses); a file that cannot be ruled out is listed as Unverified. Empty files
// are skipped: all of them share one content, and their copies are not tracked. Nothing on disk
// is read.
func (x *Index) UniqueFiles(scope []string) (UniqueResult, error) {
	var res UniqueResult
	d, err := x.DB()
	if err != nil {
		return res, err
	}
	var roots []int64
	var prefixes []string
	for _, dir := range scope {
		host, err := IndexPath(dir)
		if err != nil {
			return res, err
		}
		id, found, err := db.DirByPath(d, host)
		if err != nil {
			return res, err
		}
		switch {
		case !found:
			res.NotIndexed = append(res.NotIndexed, dir)
			continue
		case id != nil:
			roots = append(roots, *id)
		default: // the index root: every top-level tree
			tops, err := topDirs(d)
			if err != nil {
				return res, err
			}
			roots = append(roots, tops...)
		}
		prefixes = append(prefixes, dirPrefix(host))
	}
	if len(roots) == 0 { // a nil filter would mean the whole index
		return res, nil
	}
	var filter []int64
	var entries []db.Entry
	for _, r := range roots {
		ids, err := db.TreeIDs(d, &r, true)
		if err != nil {
			return res, err
		}
		filter = append(filter, ids...)
		under, err := d.FilesUnder(r, true)
		if err != nil {
			return res, err
		}
		entries = append(entries, under...)
	}
	confirmed, err := d.Duplicates(filter, false)
	if err != nil {
		return res, err
	}
	potential, err := d.PotentialDuplicates(filter, false)
	if err != nil {
		return res, err
	}
	hidden := copiedOutside(confirmed, prefixes)
	possible := copiedOutside(potential, prefixes)
	res.Unverified = map[string]bool{}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Size == 0 || hidden[e.Path] || seen[e.Path] {
			continue
		}
		seen[e.Path] = true
		res.Files = append(res.Files, db.Found{Name: filepath.Base(e.Path), Path: e.Path, Size: e.Size, Mtime: e.Mtime, Hashed: e.Hashed})
		if !e.Hashed || possible[e.Path] {
			res.Unverified[e.Path] = true
		}
	}
	slices.SortFunc(res.Files, func(a, b db.Found) int { return cmp.Compare(a.Path, b.Path) })
	return res, nil
}

// copiedOutside is every location of the groups that have a location outside prefixes.
func copiedOutside(groups []db.DupGroup, prefixes []string) map[string]bool {
	out := map[string]bool{}
	for _, g := range groups {
		if !hasLocationOutside(g, prefixes) {
			continue
		}
		for _, p := range g.Locations {
			out[p] = true
		}
	}
	return out
}

// hasLocationOutside reports whether a location of g is under none of prefixes.
func hasLocationOutside(g db.DupGroup, prefixes []string) bool {
	return slices.ContainsFunc(g.Locations, func(p string) bool { return !underAnyPrefix(p, prefixes) })
}

func underAnyPrefix(path string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(path, prefix) })
}

// dirPrefix is the prefix of every path below the directory host.
func dirPrefix(host string) string {
	return strings.TrimSuffix(host, "/") + "/"
}
