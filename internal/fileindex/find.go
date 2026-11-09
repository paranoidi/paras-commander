package fileindex

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/fileindex/db"
)

// FindQuery is one find-in-index search. Boolean picks the FTS5 filename search (files only);
// otherwise Text is a case-insensitive glob on basenames, matched as a substring when it has no
// wildcard.
type FindQuery struct {
	Text      string
	Boolean   bool
	OnlyDirs  bool
	OnlyFiles bool
	// Roots, when set, are searched instead of the root Find is given (results merged and
	// deduplicated); depth and hidden checks stay relative to that root.
	Roots []string
	// MaxDepth limits results to this many path segments below the root (0 = unlimited).
	MaxDepth int
	// IncludeHidden keeps results with a dot-prefixed path component below the root.
	IncludeHidden bool
}

// Find searches the index under the host directory root and returns at most limit results
// (limit <= 0: all) with truncated set when more matched. A FTS5 syntax error comes back as err.
func (x *Index) Find(root string, q FindQuery, limit int) (found []db.Found, truncated bool, err error) {
	d, err := x.DB()
	if err != nil {
		return nil, false, err
	}
	scope := q.Roots
	if len(scope) == 0 {
		scope = []string{root}
	}
	var roots []int64
	for _, dir := range scope {
		id, ok, err := db.DirByPath(d, dir)
		if err != nil {
			return nil, false, err
		}
		switch {
		case !ok:
		case id != nil:
			roots = append(roots, *id)
		default: // the index root: every top-level tree
			tops, err := topDirs(d)
			if err != nil {
				return nil, false, err
			}
			roots = append(roots, tops...)
		}
	}
	pattern := q.Text
	if !q.Boolean && !strings.ContainsAny(pattern, "*?") {
		pattern = "*" + pattern + "*"
	}
	for _, r := range roots {
		var part []db.Found
		if q.Boolean {
			part, err = d.FindByWords(q.Text, r)
		} else {
			part, err = d.FindByName(pattern, r, !q.OnlyDirs, !q.OnlyFiles)
		}
		if err != nil {
			return nil, false, err
		}
		found = append(found, part...)
	}
	found = filterFound(found, root, q, len(scope) > 1)
	// ponytail: the library has no LIMIT, so results are capped after the query. Push a LIMIT
	// into fileindex/db if huge result sets show up slow.
	if limit > 0 && len(found) > limit {
		found, truncated = found[:limit], true
	}
	return found, truncated, nil
}

// FilterUnique narrows the files of a UniqueResult as Find narrows its results: a blank q.Text
// keeps every name, else q.Text is the Pattern query; then q's depth and hidden rules under root,
// and the cap at limit (limit <= 0: all). files is left untouched.
func FilterUnique(files []db.Found, root string, q FindQuery, limit int) (found []db.Found, truncated bool) {
	match := func(string) bool { return true }
	if text := strings.TrimSpace(q.Text); text != "" {
		pattern := db.Lower(q.Text)
		if !strings.ContainsAny(pattern, "*?") {
			pattern = "*" + pattern + "*"
		}
		// The library's glob knows only * and ?: everything else is literal.
		pattern = strings.NewReplacer(`\`, `\\`, `[`, `\[`).Replace(pattern)
		match = func(name string) bool {
			ok, err := filepath.Match(pattern, db.Lower(name))
			return err == nil && ok
		}
	}
	found = make([]db.Found, 0, len(files))
	for _, f := range files {
		if match(f.Name) {
			found = append(found, f)
		}
	}
	found = filterFound(found, root, q, false)
	if limit > 0 && len(found) > limit {
		found, truncated = found[:limit], true
	}
	return found, truncated
}

// filterFound drops results beyond the depth limit or under a hidden component, and with dedupe
// the repeats of nested scope roots.
func filterFound(found []db.Found, root string, q FindQuery, dedupe bool) []db.Found {
	if q.MaxDepth <= 0 && q.IncludeHidden && !dedupe {
		return found
	}
	seen := map[string]bool{}
	out := found[:0]
	for _, f := range found {
		if dedupe {
			if seen[f.Path] {
				continue
			}
			seen[f.Path] = true
		}
		rel, err := filepath.Rel(root, f.Path)
		if err != nil {
			rel = f.Path
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if q.MaxDepth > 0 && len(parts) > q.MaxDepth {
			continue
		}
		if !q.IncludeHidden && slices.ContainsFunc(parts, func(p string) bool { return strings.HasPrefix(p, ".") }) {
			continue
		}
		out = append(out, f)
	}
	return out
}

// CopiesState says what the index knows about a file's copies.
type CopiesState int

const (
	// CopiesNotIndexed: the path is not in the index.
	CopiesNotIndexed CopiesState = iota
	// CopiesNotHashed: the path is indexed but has no full hash yet (a fast scan or an
	// unfinished scan), so its copies are unknown.
	CopiesNotHashed
	// CopiesHashed: the path is fully hashed; the returned list holds its copies.
	CopiesHashed
	// CopiesEmpty: the path is an empty file. All empty files share one content, so they are
	// not listed as copies of each other.
	CopiesEmpty
)

// OtherCopies lists the indexed files with the same content as the file at path (a host path,
// resolved like the index records it), sorted, without path itself. A JPEG also matches the
// files that differ from it only in metadata, as the Copies column counts them. The list is
// only meaningful for CopiesHashed.
func (x *Index) OtherCopies(path string) ([]string, CopiesState, error) {
	d, err := x.DB()
	if err != nil {
		return nil, CopiesNotIndexed, err
	}
	host, err := IndexPath(path)
	if err != nil {
		return nil, CopiesNotIndexed, err
	}
	rec, found, err := d.FileInfo(host)
	if err != nil {
		return nil, CopiesNotIndexed, err
	}
	if !found {
		return nil, CopiesNotIndexed, nil
	}
	if rec.Size == 0 {
		return nil, CopiesEmpty, nil
	}
	if !rec.Hashed {
		return nil, CopiesNotHashed, nil
	}
	matches, err := d.FindByHash(rec.SHA256)
	if err != nil {
		return nil, CopiesHashed, err
	}
	for _, m := range matches {
		if m.Path == host && m.NoExifJPEG != "" {
			more, err := d.FindByHash(m.NoExifJPEG)
			if err != nil {
				return nil, CopiesHashed, err
			}
			matches = append(matches, more...)
			break
		}
	}
	seen := map[string]bool{host: true}
	var out []string
	for _, m := range matches {
		if !seen[m.Path] {
			seen[m.Path] = true
			out = append(out, m.Path)
		}
	}
	slices.Sort(out)
	return out, CopiesHashed, nil
}
