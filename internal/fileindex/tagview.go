package fileindex

import (
	"cmp"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// A tag view (pathloc.TagQuery) shows the index entries that carry tags themselves: files by
// their content's tags (or a pending tag while unhashed), directories by their own. The match
// set of a query is read from the tag tables, never by walking the tree, and kept in memory:
// browsing a tag view is map lookups.

// tagViewMax is how many queries keep their match set in memory.
const tagViewMax = 4

// tagViewSettle is how long the database must have been unchanged for its match sets to be kept.
// File times tick in milliseconds and a write-ahead log at its high-water mark keeps its size, so
// two commits close together can leave one stamp.
const tagViewSettle = 100 * time.Millisecond

// NotTaggedError is the refusal of an index change that would also reach records a tag view
// hides: a directory that is neither tagged nor inside a tagged one, or the top of a view.
type NotTaggedError struct{ Name string }

func (e *NotTaggedError) Error() string { return "Not tagged itself: " + e.Name }

// tagKey identifies a query's match set; Flat is left out, both modes share it.
type tagKey struct {
	any  bool
	tags string
}

func keyOf(q pathloc.TagQuery) tagKey {
	return tagKey{any: q.Any, tags: strings.Join(q.Tags, "\x00")}
}

// fileStamp is what a changed database file looks like from outside: size and mtime.
type fileStamp struct{ size, mtime int64 }

// dbStamp is the database file and its write-ahead log. Both change on every commit by pc or by
// another process, which is how the cache notices writes it was not told about.
type dbStamp struct {
	path      string
	main, wal fileStamp
}

func stampOf(path string) dbStamp {
	one := func(p string) fileStamp {
		fi, err := os.Stat(p)
		if err != nil {
			return fileStamp{}
		}
		return fileStamp{size: fi.Size(), mtime: fi.ModTime().UnixNano()}
	}
	return dbStamp{path: path, main: one(path), wal: one(path + "-wal")}
}

// settled reports whether the files last changed long enough ago for the stamp to tell the next
// change apart.
func (s dbStamp) settled() bool {
	return time.Since(time.Unix(0, max(s.main.mtime, s.wal.mtime))) >= tagViewSettle
}

// tagViewCache holds the match sets valid for one database stamp.
//
// ponytail: a running scan commits often and each commit changes the stamp, so every listing in
// a tag view then rebuilds the match set; no row cap either (a flat list of every file holds
// them all). Two listings that miss together both build. Per-query invalidation or a row limit
// if that bites.
type tagViewCache struct {
	mu    sync.Mutex
	stamp dbStamp
	sets  map[tagKey]*matchSet
	order []tagKey // oldest first
}

// matchSet is the entries of one tag query: the files and directories that carry the tags
// themselves, and the directories above them, which are all that a listing needs.
type matchSet struct {
	files   map[int64][]db.ListedFile // matching files by directory id, sorted by name
	matched map[int64]bool            // directories carrying the tags themselves
	kids    map[int64][]int64         // subdirectories in the set by directory id, sorted by name
	name    map[int64]string
	path    map[int64]string // host path of every directory in the set
	byPath  map[string]int64

	flatOnce sync.Once
	flat     []fsbackend.Entry
}

// matches returns the match set of q, from the cache while the database is unchanged.
func (x *Index) matches(ctx context.Context, q pathloc.TagQuery) (*matchSet, error) {
	d, err := x.DB()
	if err != nil {
		return nil, err
	}
	stamp := stampOf(d.Path) // before reading: a write during the build leaves the stamp stale
	keep := stamp.settled()
	key, c := keyOf(q), &x.tagViews
	c.mu.Lock()
	if c.sets == nil || c.stamp != stamp {
		c.stamp, c.sets, c.order = stamp, map[tagKey]*matchSet{}, nil
	}
	ms := c.sets[key]
	c.mu.Unlock()
	if ms != nil {
		return ms, nil
	}
	if ms, err = buildMatchSet(ctx, d, q); err != nil {
		return nil, err
	}
	if !keep {
		return ms, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, dup := c.sets[key]; c.stamp == stamp && !dup {
		if len(c.order) >= tagViewMax {
			delete(c.sets, c.order[0])
			c.order = c.order[1:]
		}
		c.sets[key] = ms
		c.order = append(c.order, key)
	}
	return ms, nil
}

const (
	tagIDs = `SELECT id FROM tag WHERE name IN (SELECT value FROM json_each(?1))`

	// matchedFiles selects the instances carrying at least ?2 of the tags named in ?1, by content
	// (tags on absent content match nothing) or pending.
	matchedFiles = `WITH hit(inst, tag) AS (
  SELECT i.id, ct.tag_id FROM content_tag ct CROSS JOIN file f ON f.sha256 = ct.sha256 CROSS JOIN instance i ON i.file_id = f.id
  WHERE ct.tag_id IN (` + tagIDs + `)
  UNION SELECT instance_id, tag_id FROM pending_tag WHERE tag_id IN (` + tagIDs + `))
SELECT i.dir_id, i.filename, i.size, COALESCE(CAST(i.mtime AS TEXT), '')
FROM (SELECT inst FROM hit GROUP BY inst HAVING COUNT(DISTINCT tag) >= ?2) m CROSS JOIN instance i ON i.id = m.inst`

	matchedDirs = `SELECT dir_id FROM dir_tag WHERE tag_id IN (` + tagIDs + `) AND dir_id IS NOT NULL
GROUP BY dir_id HAVING COUNT(DISTINCT tag_id) >= ?2`

	// aboveDirs lists the directories in ?1 (a JSON array of ids) and all their ancestors.
	aboveDirs = `WITH RECURSIVE above(id, parent_id, name) AS (
  SELECT id, parent_id, name FROM dir WHERE id IN (SELECT value FROM json_each(?1))
  UNION SELECT d.id, d.parent_id, d.name FROM dir d JOIN above ON d.id = above.parent_id)
SELECT id, parent_id, name FROM above`
)

// buildMatchSet reads the match set in one transaction, so a prune running meanwhile cannot take
// a directory from under the paths built from it.
func buildMatchSet(ctx context.Context, d *db.DB, q pathloc.TagQuery) (*matchSet, error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	ms := &matchSet{
		files:   map[int64][]db.ListedFile{},
		matched: map[int64]bool{},
		kids:    map[int64][]int64{},
		name:    map[int64]string{},
		path:    map[int64]string{},
		byPath:  map[string]int64{},
	}
	names, err := json.Marshal(q.Tags)
	if err != nil {
		return nil, err
	}
	need := 1
	if !q.Any {
		need = len(q.Tags)
	}
	var known int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tag WHERE name IN (SELECT value FROM json_each(?1))`, string(names)).Scan(&known); err != nil {
		return nil, err
	}
	if known < need { // an unknown tag matches nothing when all are required; none known, nothing at all
		return ms, nil
	}
	seed := map[int64]bool{}
	rows, err := tx.QueryContext(ctx, matchedFiles, string(names), need)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var dir int64
		var f db.ListedFile
		if err := rows.Scan(&dir, &f.Name, &f.Size, &f.Mtime); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ms.files[dir] = append(ms.files[dir], f)
		seed[dir] = true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, matchedDirs, string(names), need)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ms.matched[id], seed[id] = true, true
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(seed) == 0 {
		return ms, nil
	}
	ids := make([]int64, 0, len(seed))
	for id := range seed {
		ids = append(ids, id)
	}
	list, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	rows, err = tx.QueryContext(ctx, aboveDirs, string(list))
	if err != nil {
		return nil, err
	}
	parent := map[int64]int64{} // the root rows have none
	for rows.Next() {
		var id int64
		var up *int64
		var name string
		if err := rows.Scan(&id, &up, &name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ms.name[id] = name
		if up != nil {
			parent[id] = *up
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var build func(id int64) string
	build = func(id int64) string {
		if p, ok := ms.path[id]; ok {
			return p
		}
		p := ms.name[id]
		if up, ok := parent[id]; ok {
			p = db.JoinParts([]string{build(up), p})
		}
		ms.path[id] = p
		return p
	}
	for id := range ms.name {
		ms.byPath[build(id)] = id
	}
	for id, up := range parent {
		ms.kids[up] = append(ms.kids[up], id)
	}
	for _, ids := range ms.kids {
		slices.SortFunc(ids, func(a, b int64) int { return cmp.Compare(ms.name[a], ms.name[b]) })
	}
	for _, list := range ms.files {
		slices.SortFunc(list, func(a, b db.ListedFile) int { return cmp.Compare(a.Name, b.Name) })
	}
	return ms, ctx.Err()
}

// covered reports whether host is a directory carrying the tags itself or inside one.
func (ms *matchSet) covered(host string) bool {
	for {
		if id, ok := ms.byPath[host]; ok && ms.matched[id] {
			return true
		}
		up := filepath.Dir(host)
		if up == host {
			return false
		}
		host = up
	}
}

// partial lists host, a directory that is not covered: the subdirectories that lead to a match
// or are one, then the matching files in it.
func (ms *matchSet) partial(dir pathloc.Path, host string) []fsbackend.Entry {
	id, ok := ms.byPath[host]
	if !ok {
		return nil
	}
	var out []fsbackend.Entry
	for _, kid := range ms.kids[id] {
		if loc, err := dir.Join(ms.name[kid]); err == nil {
			out = append(out, dirEntry(loc, ms.name[kid]))
		}
	}
	for _, f := range ms.files[id] {
		if loc, err := dir.Join(f.Name); err == nil {
			out = append(out, fileEntry(loc, f))
		}
	}
	return out
}

// flatRows is the list of a flat view: one entry per match, a file inside a matching directory
// included, sorted by host path. The slice is shared: callers copy it.
func (ms *matchSet) flatRows(q pathloc.TagQuery) []fsbackend.Entry {
	ms.flatOnce.Do(func() {
		q.Flat = true
		type row struct {
			host string
			e    fsbackend.Entry
		}
		var rows []row
		add := func(host string, entry func(pathloc.Path) fsbackend.Entry) {
			if loc, err := pathloc.TagView(q, host); err == nil {
				rows = append(rows, row{host, entry(loc)})
			}
		}
		for id := range ms.matched {
			name := ms.name[id]
			add(ms.path[id], func(loc pathloc.Path) fsbackend.Entry { return dirEntry(loc, name) })
		}
		for dir, files := range ms.files {
			for _, f := range files {
				add(db.JoinParts([]string{ms.path[dir], f.Name}), func(loc pathloc.Path) fsbackend.Entry { return fileEntry(loc, f) })
			}
		}
		slices.SortFunc(rows, func(a, b row) int { return cmp.Compare(a.host, b.host) })
		ms.flat = make([]fsbackend.Entry, len(rows))
		for i, r := range rows {
			ms.flat[i] = r.e
		}
	})
	return ms.flat
}

// tagList lists the root of a flat view.
func (x *Index) tagList(ctx context.Context, q pathloc.TagQuery) ([]fsbackend.Entry, error) {
	ms, err := x.matches(ctx, q)
	if err != nil {
		return nil, err
	}
	return slices.Clone(ms.flatRows(q)), nil
}

// TagViewStart is where a structure view of q opens: the deepest directory holding every match
// ("/" when there is none), and how many matches there are (files and directories).
func (x *Index) TagViewStart(ctx context.Context, q pathloc.TagQuery) (host string, matches int, err error) {
	ms, err := x.matches(ctx, q)
	if err != nil {
		return "", 0, err
	}
	var common string
	hold := func(dir string) {
		if common == "" {
			common = dir
		}
		for !strings.HasPrefix(dirPrefix(dir), dirPrefix(common)) {
			common = filepath.Dir(common)
		}
	}
	for dir, files := range ms.files {
		matches += len(files)
		hold(ms.path[dir])
	}
	for id := range ms.matched {
		matches++
		hold(filepath.Dir(ms.path[id]))
	}
	if matches == 0 {
		return "/", 0, nil
	}
	return common, matches, nil
}

// TagNames lists the names of all tags, sorted.
func (x *Index) TagNames(ctx context.Context) ([]string, error) {
	d, err := x.DB()
	if err != nil {
		return nil, err
	}
	rows, err := d.QueryContext(ctx, `SELECT name FROM tag ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// guardTagChange refuses a rename or removal at loc that a tag view would only partly show: the
// top of a view, or a directory of a structure view that is neither tagged nor inside a tagged
// one (the change would also reach the records the view hides).
func (x *Index) guardTagChange(ctx context.Context, d *db.DB, loc pathloc.Path, host string) error {
	q, ok := loc.TagQuery()
	if !ok {
		return nil
	}
	if host == "/" {
		return &NotTaggedError{Name: host}
	}
	if q.Flat {
		return nil
	}
	if _, isDir, err := db.DirByPath(d, host); err != nil || !isDir {
		return err
	}
	ms, err := x.matches(ctx, q)
	if err != nil {
		return err
	}
	if !ms.covered(host) {
		return &NotTaggedError{Name: loc.Base()}
	}
	return nil
}

// totals sums the matches at and below the directory under, without counting a match twice: the
// files outside every tagged directory, and each outermost tagged directory's whole tree.
func (ms *matchSet) totals(ctx context.Context, d *db.DB, under string) (bytes, files int64, err error) {
	within := func(p string) bool { return under == "/" || strings.HasPrefix(dirPrefix(p), dirPrefix(under)) }
	for dir, list := range ms.files {
		if p := ms.path[dir]; within(p) && !ms.covered(p) {
			for _, f := range list {
				bytes += f.Size
			}
			files += int64(len(list))
		}
	}
	for id := range ms.matched {
		p := ms.path[id]
		if !within(p) || (p != "/" && ms.covered(filepath.Dir(p))) {
			continue
		}
		var n, b int64
		if err := d.QueryRowContext(ctx, subtreeQuery, id).Scan(&n, &b); err != nil {
			return 0, 0, err
		}
		files, bytes = files+n, bytes+b
	}
	return bytes, files, nil
}
