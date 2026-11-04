package fileindex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DedupFiles lists every indexed file under root with its content hash (the
// metadata-free hash for JPEGs, else SHA-256), keyed by Rel. Nothing on
// disk is read. Files in a potential duplicate set (same size and first 8 KB, at
// least one only fast-hashed) carry FileRecord.Potential. Other entries without a
// valid hash are skipped.
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
	if err != nil {
		return nil, nil, err
	}
	if !found { // the index records real paths: retry through symlinks (a gone directory stays as is)
		if real, rerr := db.Resolve(host); rerr == nil && real != host {
			host = real
			id, found, err = db.DirByPath(d, host)
		}
	}
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
	potential, err := potentialKeys(d, entries)
	if err != nil {
		return nil, nil, err
	}
	var files []compare.FileRecord
	hashes := make(map[string][32]byte, len(entries))
	for i, e := range entries {
		if i%1024 == 0 && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		b, err := hex.DecodeString(e.Content)
		valid := err == nil && len(b) == 32
		key, isPotential := potential[e.Path]
		if !valid && !isPotential {
			continue
		}
		abs, err := pathloc.FileIndex(e.Path)
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(strings.TrimPrefix(e.Path, strings.TrimSuffix(host, "/")+"/"))
		files = append(files, compare.FileRecord{Abs: abs, Rel: rel, Size: e.Size, Potential: key})
		if valid {
			hashes[rel] = [32]byte(b)
		}
	}
	return files, hashes, nil
}

// DedupOutside lists the files under dir (a host path) that also have a copy somewhere else in
// the index, with every member of those groups, so the dedup view answers "is this safe to delete
// because it exists elsewhere?". Groups entirely inside or entirely outside dir are left out.
// Records are relative to the index root (Rel has no leading "/"). A group with a fast-hashed
// member carries FileRecord.Potential and no hashes entry; a confirmed group's hashes entry is a
// key derived from its Locations, which only needs to be a stable group identity.
// ponytail: JPEG copies differing in metadata all show the group's smallest size, because DupGroup
// carries no per-file size; upgrade path is a per-location size from the library.
func (x *Index) DedupOutside(ctx context.Context, dir string) ([]compare.FileRecord, map[string][32]byte, error) {
	host, err := IndexPath(dir)
	if err != nil {
		return nil, nil, err
	}
	d, err := x.DB()
	if err != nil {
		return nil, nil, err
	}
	id, found, err := db.DirByPath(d, host)
	if err != nil || !found || id == nil { // the index root has nothing outside it
		return nil, nil, err
	}
	filter, err := db.TreeIDs(d, id, true)
	if err != nil {
		return nil, nil, err
	}
	prefix := strings.TrimSuffix(host, "/") + "/"
	// spans reports whether g has a member inside dir and one outside it.
	spans := func(g db.DupGroup) bool {
		in := slices.ContainsFunc(g.Locations, func(p string) bool { return strings.HasPrefix(p, prefix) })
		out := slices.ContainsFunc(g.Locations, func(p string) bool { return !strings.HasPrefix(p, prefix) })
		return in && out
	}
	potential, err := d.PotentialDuplicates(filter, false)
	if err != nil {
		return nil, nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	confirmed, err := d.Duplicates(filter, false)
	if err != nil {
		return nil, nil, err
	}
	var files []compare.FileRecord
	hashes := map[string][32]byte{}
	seen := map[string]bool{}
	add := func(g db.DupGroup, path string, key [32]byte, isPotential bool) {
		abs, err := pathloc.FileIndex(path)
		if err != nil {
			return
		}
		rel := strings.TrimPrefix(path, "/")
		rec := compare.FileRecord{Abs: abs, Rel: rel, Size: g.Size}
		if isPotential {
			rec.Potential = key
		} else {
			hashes[rel] = key
		}
		files = append(files, rec)
		seen[path] = true
	}
	for i, g := range potential {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if len(g.Unverified) == 0 || !spans(g) {
			continue
		}
		key := sha256.Sum256([]byte(strings.Join(g.Locations, "\x00")))
		for _, p := range g.Locations {
			add(g, p, key, true)
		}
	}
	for i, g := range confirmed {
		if i%256 == 0 && ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if !spans(g) {
			continue
		}
		key := sha256.Sum256([]byte(strings.Join(g.Locations, "\x00")))
		for _, p := range g.Locations {
			if !seen[p] { // a fully hashed member of a potential set stays in that set
				add(g, p, key, false)
			}
		}
	}
	return files, hashes, nil
}

// potentialKeys maps the entries in a potential duplicate set to that set's key. A set needs two
// entries with at least one only fast-hashed: without one, full hashes already decide it.
// ponytail: a fully hashed member leaves its confirmed group for the potential set, so a JPEG
// copy of another size (same metadata-free hash) can drop out until a full scan settles it.
func potentialKeys(d *db.DB, entries []db.Entry) (map[string][32]byte, error) {
	groups, err := d.PotentialDuplicates(nil, false)
	if err != nil || len(groups) == 0 {
		return nil, err
	}
	inScope := make(map[string]bool, len(entries))
	for _, e := range entries {
		inScope[e.Path] = true
	}
	keys := map[string][32]byte{}
	for _, g := range groups {
		isIn := func(p string) bool { return inScope[p] }
		if !slices.ContainsFunc(g.Unverified, isIn) {
			continue
		}
		members := slices.DeleteFunc(slices.Clone(g.Locations), func(p string) bool { return !isIn(p) })
		if len(members) < 2 {
			continue
		}
		key := sha256.Sum256([]byte(strings.Join(g.Locations, "\x00")))
		for _, p := range members {
			keys[p] = key
		}
	}
	return keys, nil
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
