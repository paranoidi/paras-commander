package fileindex

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TagsColumn is the name of the fileindex tags column provider (its meta picker entry).
const TagsColumn = config.FileIndexTagsColumn

// TagsProvider returns the Go column provider showing each entry's fileindex tags (a file's
// instance tags, a directory's dir tags), comma separated. It reads the whole listing in one
// EntryTags call, on local panels and fileindex:// panels alike; other schemes show no tags.
// With no usable index it shows empty cells (the open failure is reported once by Index).
func (x *Index) TagsProvider() *metacmds.Provider {
	return &metacmds.Provider{
		Name:        TagsColumn,
		Description: "fileindex tags",
		Column:      "Tags",
		Order:       config.FileIndexTagsColumnOrder,
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
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name
			}
			tags, err := d.EntryTags(host, names)
			if err != nil {
				return nil, err
			}
			out := make(map[string]string, len(tags))
			for _, e := range entries {
				if t := tags[e.Name]; len(t) > 0 {
					out[e.Path] = strings.Join(t, ", ")
				}
			}
			return out, nil
		},
	}
}

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

// TagEdit is the outcome of EditTags: how many associations changed and how many paths the
// index does not know.
type TagEdit struct {
	Changed  int
	Unknown  []string
	Targeted int
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
		res.Changed, err = d.AddTags(target, tags)
		return res, err
	}
	removed, err := d.RemoveTags(target, tags)
	res.Changed = len(removed)
	return res, err
}
