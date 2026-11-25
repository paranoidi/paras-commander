package fileindex

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/findcrit"
)

// FindQuery is one find-in-index search. Mode says how Text is read: Terms (the default) and
// Pattern match basenames of files and directories, ignoring case; Boolean is an FTS5 query on
// file names (files only).
type FindQuery struct {
	Text      string
	Mode      findcrit.IndexMode
	OnlyDirs  bool
	OnlyFiles bool
	// Roots, when set, are searched instead of the root Find is given; depth and hidden checks
	// stay relative to that root.
	Roots []string
	// MaxDepth limits results to this many path segments below the root (0 = unlimited).
	MaxDepth int
	// IncludeHidden keeps results with a dot-prefixed path component below the root.
	IncludeHidden bool
}

// Find searches the index under the host directory root (the whole index for "/" without Roots)
// and returns at most limit results (limit <= 0: all), files and directories merged by name, with
// truncated set when more matched. A FTS5 syntax error comes back as err, and so does a cancelled
// ctx (as ctx.Err()).
//
// The search is one read transaction of SQL on the index tables: it never builds the directory
// tree of a whole-index search, narrows by the trigram index when the query has a literal of three
// characters, and stops reading after limit+1 rows of each kind.
func (x *Index) Find(ctx context.Context, root string, q FindQuery, limit int) (found []db.Found, truncated bool, err error) {
	d, err := x.DB()
	if err != nil {
		return nil, false, err
	}
	var terms termGroups
	switch q.Mode {
	case findcrit.IndexTerms:
		terms = parseTerms(q.Text)
	case findcrit.IndexPattern:
		terms = patternTerms(q.Text)
	}
	if q.Mode != findcrit.IndexBoolean && len(terms) == 0 {
		return nil, false, nil
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	whole, ids, err := findScope(tx, root, q.Roots)
	if err != nil || (!whole && len(ids) == 0) {
		return nil, false, err
	}
	run := &findRun{ctx: ctx, tx: tx, root: root, q: q, terms: terms, limit: limit, parts: map[int64][]string{}}
	if !whole {
		list, _ := json.Marshal(ids)
		run.scope = string(list)
	}
	if !q.OnlyDirs {
		if found, err = run.query(false); err != nil {
			return nil, false, err
		}
	}
	if !q.OnlyFiles && q.Mode != findcrit.IndexBoolean {
		dirs, err := run.query(true)
		if err != nil {
			return nil, false, err
		}
		found = append(found, dirs...)
	}
	slices.SortStableFunc(found, func(a, b db.Found) int {
		if c := strings.Compare(db.Lower(a.Name), db.Lower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	if limit > 0 && len(found) > limit {
		found, truncated = found[:limit], true
	}
	return found, truncated, nil
}

// findScope resolves the directories a search covers: whole (the index root, no tree needed) or
// the ids of the scope directories the index knows (roots, else root).
func findScope(tx *sql.Tx, root string, roots []string) (whole bool, ids []int64, err error) {
	if len(roots) == 0 {
		roots = []string{root}
	}
	if slices.Contains(roots, "/") {
		return true, nil, nil
	}
	for _, dir := range roots {
		id, ok, err := db.DirByPath(tx, dir)
		if err != nil {
			return false, nil, err
		}
		if ok {
			ids = append(ids, *id)
		}
	}
	return false, ids, nil
}

// findRun is one Find's state: the transaction, the compiled terms and the directory paths
// resolved so far.
type findRun struct {
	ctx   context.Context
	tx    *sql.Tx
	root  string
	q     FindQuery
	terms termGroups
	limit int
	scope string // JSON array of the scope directory ids; "" = the whole index
	parts map[int64][]string
}

// query runs the file query (dirs false) or the directory query and returns the first limit+1
// rows that pass the depth and hidden filters, in name order.
func (r *findRun) query(dirs bool) ([]db.Found, error) {
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "?" + strconv.Itoa(len(args))
	}
	var sb strings.Builder
	var conds []string
	scopeArg := ""
	if r.scope != "" {
		// ponytail: the tree below a scope of 100k+ directories takes a second or two to build, for
		// each of the two queries. Build it once into a temp table if that shows up slow.
		scopeArg = arg(r.scope)
		sb.WriteString("WITH RECURSIVE dir_tree(id) AS (SELECT value FROM json_each(" + scopeArg + ") " +
			"UNION ALL SELECT d.id FROM dir d JOIN dir_tree t ON d.parent_id = t.id) ")
	}
	col := "i.filename_lower"
	if dirs {
		col = "d.name_lower"
		sb.WriteString("SELECT d.id, d.name FROM dir d WHERE ")
		conds = append(conds, "d.parent_id IS NOT NULL") // the "/" row is not a name
		if r.scope != "" {
			conds = append(conds, "d.parent_id IN (SELECT id FROM dir_tree)") // below a scope root, never the root itself
		}
	} else {
		sb.WriteString("SELECT i.dir_id, i.filename, i.size FROM instance i WHERE ")
		if r.scope != "" {
			conds = append(conds, "i.dir_id IN (SELECT id FROM dir_tree)")
		}
	}
	if r.q.Mode == findcrit.IndexBoolean {
		conds = append(conds, "i.id IN (SELECT rowid FROM instance_words WHERE instance_words MATCH "+arg(r.q.Text)+")")
	} else {
		conds = append(conds, r.terms.where(col, arg))
		if match, ok := r.terms.trigram(); ok && !dirs {
			conds = append(conds, "i.id IN (SELECT rowid FROM instance_trigram WHERE instance_trigram MATCH "+arg(match)+")")
		}
	}
	if !r.q.IncludeHidden {
		conds = append(conds, col+" NOT LIKE '.%'")
	}
	sb.WriteString(strings.Join(conds, " AND "))
	if dirs {
		sb.WriteString(" ORDER BY d.name_lower, d.id")
	} else {
		sb.WriteString(" ORDER BY i.filename_lower, i.id")
	}
	rows, err := r.tx.QueryContext(r.ctx, sb.String(), args...)
	if err != nil {
		return nil, r.fail(err)
	}
	defer func() { _ = rows.Close() }()
	var out []db.Found
	for (r.limit <= 0 || len(out) <= r.limit) && rows.Next() {
		f := db.Found{IsDir: dirs, Hashed: true}
		var id int64
		if dirs {
			err = rows.Scan(&id, &f.Name)
		} else {
			err = rows.Scan(&id, &f.Name, &f.Size)
		}
		if err != nil {
			return nil, r.fail(err)
		}
		parts, err := r.dirParts(id)
		if err != nil {
			return nil, r.fail(err)
		}
		if !dirs {
			parts = append(slices.Clone(parts), f.Name)
		}
		f.Path = db.JoinParts(parts)
		if r.q.keep(r.root, f.Path) {
			out = append(out, f)
		}
	}
	return out, r.fail(rows.Err())
}

// fail is err, or the ctx error when the search was cancelled (the driver's own text for an
// interrupted statement is not one).
func (r *findRun) fail(err error) error {
	if err != nil && r.ctx.Err() != nil {
		return r.ctx.Err()
	}
	return err
}

// dirParts is the path components of directory id, memoised with its ancestors.
func (r *findRun) dirParts(id int64) ([]string, error) {
	if p, ok := r.parts[id]; ok {
		return p, nil
	}
	var chain []int64
	var names []string
	var base []string
	for cur := id; ; {
		if p, ok := r.parts[cur]; ok {
			base = p
			break
		}
		var name string
		var parent sql.NullInt64
		if err := r.tx.QueryRowContext(r.ctx, "SELECT name, parent_id FROM dir WHERE id = ?", cur).Scan(&name, &parent); err != nil {
			return nil, err
		}
		chain, names = append(chain, cur), append(names, name)
		if !parent.Valid {
			break
		}
		cur = parent.Int64
	}
	for i := len(chain) - 1; i >= 0; i-- {
		base = append(slices.Clone(base), names[i])
		r.parts[chain[i]] = base
	}
	return r.parts[id], nil
}

// keep reports whether path passes the depth limit and the hidden filter, both read from the
// path below root.
func (q FindQuery) keep(root, path string) bool {
	if q.MaxDepth <= 0 && q.IncludeHidden {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if q.MaxDepth > 0 && len(parts) > q.MaxDepth {
		return false
	}
	return q.IncludeHidden || !slices.ContainsFunc(parts, func(p string) bool { return strings.HasPrefix(p, ".") })
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

// CopyFile is one indexed copy of a file: its host path and size.
type CopyFile struct {
	Path string
	Size int64
}

// OtherCopies lists the indexed files with the same content as the file at path (a host path,
// resolved like the index records it), sorted by path, without path itself. A JPEG also matches the
// files that differ from it only in metadata, as the Copies column counts them. The list is
// only meaningful for CopiesHashed.
func (x *Index) OtherCopies(path string) ([]CopyFile, CopiesState, error) {
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
	var out []CopyFile
	for _, m := range matches {
		if !seen[m.Path] {
			seen[m.Path] = true
			out = append(out, CopyFile{Path: m.Path, Size: m.Size})
		}
	}
	slices.SortFunc(out, func(a, b CopyFile) int { return strings.Compare(a.Path, b.Path) })
	return out, CopiesHashed, nil
}
