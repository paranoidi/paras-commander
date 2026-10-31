package fileindex

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Fileindex column provider names (meta entry names, as [fileindex].default_columns lists them).
const (
	StatusColumn    = "fileindex.status"
	TagsColumn      = config.FileIndexTagsColumn
	CopiesColumn    = "fileindex.copies"
	FirstSeenColumn = "fileindex.first-seen"
	LastSeenColumn  = "fileindex.last-seen"
	TreeFilesColumn = "fileindex.tree-files"
	TreeSizeColumn  = "fileindex.tree-size"
	HashColumn      = "fileindex.hash"
)

// Index status values shown by the status column.
const (
	StatusIndexed = "indexed" // in the index and, for a file, the same size and mtime on disk
	StatusChanged = "changed" // a file whose size or mtime on disk differs from the index
	StatusNew     = "new"     // on disk but not in the index
	StatusMissing = "missing" // in the index but gone from disk
)

// hashCells is how many hex digits the hash column shows.
const hashCells = 12

// columnValues computes one fileindex column for a listing: host is the listing's directory as
// the index stores it, onDisk whether entries come from the disk (a local panel) rather than
// from the index (a fileindex:// panel). It returns cell text keyed by entry name.
type columnValues func(ctx context.Context, d *db.DB, host string, entries []localfs.Entry, onDisk bool) (map[string]string, error)

// Providers returns the fileindex column providers in picker (and left-to-right) order, listed
// in the meta picker's Fileindex section while the index opens.
func (x *Index) Providers() []*metacmds.Provider {
	cols := []struct {
		name, title string
		values      columnValues
	}{
		{StatusColumn, "Status", statusValues},
		{TagsColumn, "Tags", tagValues},
		{CopiesColumn, "Copies", byName(copiesQuery, strconv.Itoa)},
		{FirstSeenColumn, "First seen", byName(timestampQuery("created_at"), formatTimestamp)},
		{LastSeenColumn, "Last seen", byName(timestampQuery("last_seen"), formatTimestamp)},
		{TreeFilesColumn, "Tree files", treeValues(func(files, _ int64) string { return strconv.FormatInt(files, 10) })},
		{TreeSizeColumn, "Tree size", treeValues(func(_, size int64) string {
			return panellist.FormatByteSizeCompact(size, panellist.SizeCells)
		})},
		{HashColumn, "Hash", byName(hashQuery, func(h string) string { return h[:min(len(h), hashCells)] })},
	}
	out := make([]*metacmds.Provider, len(cols))
	for i, c := range cols {
		out[i] = x.provider(c.name, c.title, config.FileIndexColumnOrder+i, c.values)
	}
	return out
}

// Provider returns the fileindex column provider called name, or nil.
func (x *Index) Provider(name string) *metacmds.Provider {
	for _, p := range x.Providers() {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// provider builds a fileindex column. It runs on local panels and fileindex:// panels alike;
// other schemes show empty cells.
func (x *Index) provider(name, title string, order int, values columnValues) *metacmds.Provider {
	return &metacmds.Provider{
		Name:      name,
		Column:    title,
		Order:     order,
		Section:   "Fileindex",
		Available: x.Available,
		Values: func(ctx context.Context, dir string, entries []localfs.Entry) (map[string]string, error) {
			loc, err := pathloc.Parse(dir)
			if err != nil || loc.Scheme() == pathloc.SchemeSFTP || len(entries) == 0 {
				return nil, nil
			}
			host, err := indexPath(loc)
			if err != nil {
				return nil, nil
			}
			d, err := x.DB()
			if err != nil {
				return nil, nil
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			byName, err := values(ctx, d, host, entries, loc.Scheme() == pathloc.SchemeFile)
			if err != nil {
				return nil, err
			}
			out := make(map[string]string, len(byName))
			for _, e := range entries {
				if v := byName[e.Name]; v != "" {
					out[e.Path] = v
				}
			}
			return out, nil
		},
	}
}

// tagValues shows a file's instance tags or a directory's dir tags, comma separated.
func tagValues(_ context.Context, d *db.DB, host string, entries []localfs.Entry, _ bool) (map[string]string, error) {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	tags, err := d.EntryTags(host, names)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(tags))
	for n, t := range tags {
		out[n] = strings.Join(t, ", ")
	}
	return out, nil
}

// byName runs query, which takes the listing's dir id for each of its parameters and returns
// (entry name, value) rows, and formats each value.
func byName[T any](query string, format func(T) string) columnValues {
	return func(_ context.Context, d *db.DB, host string, _ []localfs.Entry, _ bool) (map[string]string, error) {
		id, found, err := db.DirByPath(d, host)
		if err != nil || !found {
			return nil, err
		}
		args := make([]any, strings.Count(query, "?"))
		for i := range args {
			args[i] = id
		}
		rows, err := d.Query(query, args...)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		out := map[string]string{}
		for rows.Next() {
			var name string
			var v T
			if err := rows.Scan(&name, &v); err != nil {
				return nil, err
			}
			out[name] = format(v)
		}
		return out, rows.Err()
	}
}

// timestampQuery lists (entry name, col) for the files and subdirectories directly in a dir; a
// NULL dir id is the index root, which holds only top-level dirs.
func timestampQuery(col string) string {
	return `SELECT filename, CAST(` + col + ` AS TEXT) FROM instance WHERE dir_id IS ?
UNION ALL SELECT name, CAST(` + col + ` AS TEXT) FROM dir WHERE parent_id IS ?`
}

// copiesQuery counts, per file directly in a dir, the indexed files with the same content: the
// same metadata-free hash for a JPEG (as find duplicates groups them), else the same file row.
const copiesQuery = `SELECT i.filename, CASE WHEN f.sha256_noexif_jpeg IS NOT NULL
THEN (SELECT COUNT(*) FROM instance i2 JOIN file f2 ON f2.id = i2.file_id WHERE f2.sha256_noexif_jpeg = f.sha256_noexif_jpeg)
ELSE (SELECT COUNT(*) FROM instance i2 WHERE i2.file_id = i.file_id) END
FROM instance i JOIN file f ON f.id = i.file_id WHERE i.dir_id IS ?`

// hashQuery lists the SHA-256 of each file directly in a dir.
const hashQuery = `SELECT i.filename, f.sha256 FROM instance i JOIN file f ON f.id = i.file_id
WHERE i.dir_id IS ?`

// treeQuery totals the indexed files and bytes under each subdirectory directly in a dir.
const treeQuery = `WITH RECURSIVE t(top, id) AS (
  SELECT id, id FROM dir WHERE parent_id IS ?
  UNION ALL SELECT t.top, d.id FROM dir d JOIN t ON d.parent_id = t.id)
SELECT top.name, COUNT(i.id), COALESCE(SUM(f.filesize), 0) FROM t JOIN dir top ON top.id = t.top
LEFT JOIN instance i ON i.dir_id = t.id LEFT JOIN file f ON f.id = i.file_id GROUP BY t.top`

// treeValues formats treeQuery's totals for each subdirectory; files get no value.
// ponytail: the tree-files and tree-size columns each run treeQuery, which walks the whole
// subtree (the whole index at its root); share one result per listing if that shows up.
func treeValues(format func(files, size int64) string) columnValues {
	return func(_ context.Context, d *db.DB, host string, _ []localfs.Entry, _ bool) (map[string]string, error) {
		id, found, err := db.DirByPath(d, host)
		if err != nil || !found {
			return nil, err
		}
		rows, err := d.Query(treeQuery, id)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		out := map[string]string{}
		for rows.Next() {
			var name string
			var files, size int64
			if err := rows.Scan(&name, &files, &size); err != nil {
				return nil, err
			}
			out[name] = format(files, size)
		}
		return out, rows.Err()
	}
}

// statusValues compares each entry with the index: a file by size and mtime, a directory by
// whether the index has it. Entries of a local panel carry their disk state; entries of a
// fileindex:// panel are stat'ed. Other entry types (symlinks, devices) get no status.
// ponytail: files the scan rules in config.ini exclude show as new.
func statusValues(ctx context.Context, d *db.DB, host string, entries []localfs.Entry, onDisk bool) (map[string]string, error) {
	files, err := d.DirFiles(host)
	if err != nil {
		return nil, err
	}
	dirs := map[string]bool{}
	if id, found, err := db.DirByPath(d, host); err != nil {
		return nil, err
	} else if found {
		names, err := subdirNames(d, id)
		if err != nil {
			return nil, err
		}
		for _, n := range names {
			dirs[n] = true
		}
	}
	out := make(map[string]string, len(entries))
	for i, e := range entries {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		typ, size, mtime := e.Type, e.Size, e.ModifiedAt
		if !onDisk {
			fi, err := os.Lstat(filepath.Join(host, e.Name))
			if errors.Is(err, fs.ErrNotExist) {
				out[e.Name] = StatusMissing
				continue
			}
			if err != nil {
				continue
			}
			typ, size, mtime = entryType(fi), fi.Size(), fi.ModTime()
		}
		switch typ {
		case localfs.EntryFile:
			r, ok := files[e.Name]
			switch {
			case !ok:
				out[e.Name] = StatusNew
			case r.Unchanged(size, mtime):
				out[e.Name] = StatusIndexed
			default:
				out[e.Name] = StatusChanged
			}
		case localfs.EntryDirectory:
			if dirs[e.Name] {
				out[e.Name] = StatusIndexed
			} else {
				out[e.Name] = StatusNew
			}
		}
	}
	return out, nil
}

func subdirNames(d *db.DB, id *int64) ([]string, error) {
	rows, err := d.Query("SELECT name FROM dir WHERE parent_id IS ?", id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n sql.NullString
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n.String)
	}
	return out, rows.Err()
}

// entryType is the listing type of a stat'ed path as far as statusValues cares: file,
// directory or anything else.
func entryType(fi fs.FileInfo) localfs.EntryType {
	switch {
	case fi.Mode().IsRegular():
		return localfs.EntryFile
	case fi.IsDir():
		return localfs.EntryDirectory
	}
	return localfs.EntryOther
}

// formatTimestamp renders a stored CURRENT_TIMESTAMP (UTC "YYYY-MM-DD HH:MM:SS") in local time
// to the minute, which still sorts as text; anything else is shown as stored.
func formatTimestamp(s string) string {
	t, err := time.ParseInLocation(time.DateTime, s, time.UTC)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04")
}
