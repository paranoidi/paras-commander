package fileindex

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DedupFiles lists every indexed file under root with its exact SHA-256 (not the
// noexif content hash, so groups stay byte-identical), keyed by Rel. Nothing on
// disk is read. Entries without a valid hash are skipped.
func (x *Index) DedupFiles(ctx context.Context, root pathloc.Path) ([]compare.FileRecord, map[string][32]byte, error) {
	host, err := pathloc.IndexHostPath(root)
	if err != nil {
		return nil, nil, err
	}
	d, err := x.DB()
	if err != nil {
		return nil, nil, err
	}
	id, found, err := db.DirByPath(d, host)
	if err != nil || !found {
		return nil, nil, err
	}
	roots := []int64{}
	if id != nil {
		roots = append(roots, *id)
	} else if roots, err = topDirs(d); err != nil { // the index root: every top-level tree
		return nil, nil, err
	}
	var entries []db.Entry
	for _, r := range roots {
		under, err := d.FilesUnder(r, true)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, under...)
	}
	var files []compare.FileRecord
	hashes := make(map[string][32]byte, len(entries))
	for i, e := range entries {
		if i%1024 == 0 && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		b, err := hex.DecodeString(e.SHA256)
		if err != nil || len(b) != 32 {
			continue
		}
		h := [32]byte(b)
		abs, err := pathloc.FileIndex(e.Path)
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(strings.TrimPrefix(e.Path, strings.TrimSuffix(host, "/")+"/"))
		files = append(files, compare.FileRecord{Abs: abs, Rel: rel, Size: e.Size})
		hashes[rel] = h
	}
	return files, hashes, nil
}

// topDirs lists the ids of the index's top-level directories.
func topDirs(d *db.DB) ([]int64, error) {
	rows, err := d.Query("SELECT id FROM dir WHERE parent_id IS NULL")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
