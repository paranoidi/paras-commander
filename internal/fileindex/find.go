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
