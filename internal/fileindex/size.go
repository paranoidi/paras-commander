package fileindex

import (
	"context"
	"io/fs"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// subtreeQuery totals the indexed files and bytes under a dir and its descendants; a NULL dir
// id is the index root (every top-level dir).
const subtreeQuery = `WITH RECURSIVE t(id) AS (
  SELECT id FROM dir WHERE id IS ?1 OR (?1 IS NULL AND parent_id IS NULL)
  UNION ALL SELECT d.id FROM dir d JOIN t ON d.parent_id = t.id)
SELECT COUNT(i.id), COALESCE(SUM(f.filesize), 0) FROM t
LEFT JOIN instance i ON i.dir_id = t.id LEFT JOIN file f ON f.id = i.file_id`

// SubtreeSize is the disk-usage total of a fileindex:// path, read from the index without
// touching the disk (diskusage.SubtreeSizer): an indexed directory's files and bytes, or an
// indexed file's size and 1. A path the index does not hold is fs.ErrNotExist.
func (x *Index) SubtreeSize(ctx context.Context, raw string) (bytes, files int64, err error) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		return 0, 0, err
	}
	host, err := pathloc.IndexHostPath(loc)
	if err != nil {
		return 0, 0, err
	}
	d, err := x.DB()
	if err != nil {
		return 0, 0, err
	}
	id, isDir, err := db.DirByPath(d, host)
	if err != nil {
		return 0, 0, err
	}
	if isDir {
		err = d.QueryRowContext(ctx, subtreeQuery, id).Scan(&files, &bytes)
		return bytes, files, err
	}
	rec, found, err := d.FileInfo(host)
	if err != nil {
		return 0, 0, err
	}
	if !found {
		return 0, 0, fs.ErrNotExist
	}
	return rec.Size, 1, nil
}

// changed tells OnChange that what the index holds changed.
func (x *Index) changed() {
	if x != nil && x.OnChange != nil {
		x.OnChange()
	}
}
