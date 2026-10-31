package fileindex

import (
	"context"
	"time"

	"github.com/paranoidi/fileindex/db"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// TagsColumn is the name of the fileindex tags column provider (its meta picker entry).
const TagsColumn = config.FileIndexTagsColumn

// LastSeenColumn is the name of the fileindex last-seen column provider.
const LastSeenColumn = config.FileIndexLastSeenColumn

// Providers returns the fileindex column providers, listed in the meta picker's Fileindex
// section while the index opens.
func (x *Index) Providers() []*metacmds.Provider {
	return []*metacmds.Provider{x.TagsProvider(), x.LastSeenProvider()}
}

// provider builds a fileindex column: values gets the listing's directory as the index stores
// it and the entry names, and returns cell text keyed by name. It runs on local panels and
// fileindex:// panels alike; other schemes show empty cells.
func (x *Index) provider(name, title string, order int, values func(d *db.DB, host string, names []string) (map[string]string, error)) *metacmds.Provider {
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
			names := make([]string, len(entries))
			for i, e := range entries {
				names[i] = e.Name
			}
			byName, err := values(d, host, names)
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

// lastSeenQuery lists (entry name, last_seen) for the files and subdirectories directly in a
// dir; a NULL dir id is the index root, which holds only top-level dirs.
const lastSeenQuery = `SELECT filename, CAST(last_seen AS TEXT) FROM instance WHERE dir_id IS ?
UNION ALL SELECT name, CAST(last_seen AS TEXT) FROM dir WHERE parent_id IS ?`

// LastSeenProvider returns the column provider showing when a scan last saw each entry (a file
// instance's or a directory's last_seen), in local time.
func (x *Index) LastSeenProvider() *metacmds.Provider {
	return x.provider(LastSeenColumn, "Last seen", config.FileIndexLastSeenColumnOrder,
		func(d *db.DB, host string, _ []string) (map[string]string, error) {
			id, found, err := db.DirByPath(d, host)
			if err != nil || !found {
				return nil, err
			}
			rows, err := d.Query(lastSeenQuery, id, id)
			if err != nil {
				return nil, err
			}
			defer func() { _ = rows.Close() }()
			out := map[string]string{}
			for rows.Next() {
				var name, seen string
				if err := rows.Scan(&name, &seen); err != nil {
					return nil, err
				}
				out[name] = formatTimestamp(seen)
			}
			return out, rows.Err()
		})
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
