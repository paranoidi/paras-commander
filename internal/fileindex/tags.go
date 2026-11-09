package fileindex

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/paranoidi/fileindex/db"
)

// ParseTags splits a typed tag list on commas and whitespace, dropping empties and repeats.
func ParseTags(raw string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// TagEdit is the outcome of EditTags: how many associations changed, how many paths the index does
// not know, the empty files an add skipped (all empty files are one content, so they can't be
// tagged) and the copies outside the paths that the change also reached.
type TagEdit struct {
	Changed      int
	Unknown      []string
	Targeted     int
	SkippedEmpty int
	OtherCopies  int
}

// EditTags adds (add) or removes tags on paths (local or fileindex://), resolved in the index,
// so a file gone from disk can still be tagged. recursive extends a directory to its whole
// tree; it never applies to files. Paths the index lacks are listed in Unknown, not an error.
func (x *Index) EditTags(paths []string, tags []string, add, recursive bool) (TagEdit, error) {
	var res TagEdit
	d, err := x.DB()
	if err != nil {
		return res, err
	}
	var target db.TagTarget
	for _, p := range paths {
		host, err := IndexPath(p)
		if err != nil {
			return res, err
		}
		_, isDir, err := db.DirByPath(d, host)
		if err != nil {
			return res, err
		}
		if !isDir {
			if _, found, err := d.InstanceID(host); err != nil {
				return res, err
			} else if !found {
				res.Unknown = append(res.Unknown, filepath.Clean(host))
				continue
			}
		}
		t, err := d.IndexTagTarget(host, recursive && isDir)
		if err != nil {
			return res, err
		}
		target.Dirs = append(target.Dirs, t.Dirs...)
		target.Instances = append(target.Instances, t.Instances...)
		res.Targeted++
	}
	if res.Targeted == 0 {
		return res, nil
	}
	if add {
		if res.SkippedEmpty, err = d.EmptyInstances(target); err != nil {
			return res, err
		}
		if res.Changed, err = d.AddTags(target, tags); err != nil {
			return res, err
		}
	} else {
		removed, err := d.RemoveTags(target, tags)
		if err != nil {
			return res, err
		}
		res.Changed = len(removed)
	}
	if res.Changed > 0 {
		res.OtherCopies, err = d.OtherCopies(target)
	}
	return res, err
}

// TagState is what a tag editor shows for a set of paths: every tag in the database, sorted by
// name, how many distinct paths were asked about, how many of them carry each tag (absent: none)
// and whether any of them is a directory.
type TagState struct {
	Names   []string
	Targets int
	Counts  map[string]int
	HasDir  bool
}

// TagState reads the tags of the index and of paths (local or fileindex://). A path the index
// lacks carries no tags, and a path named twice counts once.
func (x *Index) TagState(paths []string) (TagState, error) {
	state := TagState{Counts: map[string]int{}}
	d, err := x.DB()
	if err != nil {
		return state, err
	}
	all, err := d.ListTags()
	if err != nil {
		return state, err
	}
	for _, t := range all {
		state.Names = append(state.Names, t.Name)
	}
	byParent := map[string][]string{}
	seen := map[string]bool{}
	for _, p := range paths {
		host, err := IndexPath(p)
		if err != nil {
			return state, err
		}
		host = filepath.Clean(host)
		if !seen[host] {
			seen[host] = true
			state.Targets++
			parent := filepath.Dir(host)
			byParent[parent] = append(byParent[parent], filepath.Base(host))
			if !state.HasDir {
				if state.HasDir, err = isTagDir(d, host); err != nil {
					return state, err
				}
			}
		}
	}
	for parent, names := range byParent {
		tags, err := d.EntryTags(parent, names)
		if err != nil {
			return state, err
		}
		for _, entry := range tags {
			slices.Sort(entry)
			for _, t := range slices.Compact(entry) { // a file and a subdir may share a name
				state.Counts[t]++
			}
		}
	}
	return state, nil
}

// isTagDir reports whether a recursive tag edit of host would reach a tree: a directory of the
// index, also when gone from disk, or a directory on disk the scan of an unknown path will walk.
func isTagDir(d *db.DB, host string) (bool, error) {
	_, isDir, err := db.DirByPath(d, host)
	if err != nil || isDir {
		return isDir, err
	}
	info, err := os.Stat(host)
	return err == nil && info.IsDir(), nil
}
