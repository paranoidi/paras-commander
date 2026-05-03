package ui

import (
	"io/fs"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// entryFileInfo adapts localfs.Entry to fs.FileInfo for go-devicons.
type entryFileInfo struct {
	entry localfs.Entry
}

func fileInfoFromEntry(entry localfs.Entry) fs.FileInfo {
	return entryFileInfo{entry: entry}
}

func (e entryFileInfo) Name() string       { return e.entry.Name }
func (e entryFileInfo) Size() int64        { return e.entry.Size }
func (e entryFileInfo) Mode() fs.FileMode  { return e.entry.Mode }
func (e entryFileInfo) ModTime() time.Time { return e.entry.ModifiedAt }
func (e entryFileInfo) IsDir() bool        { return e.entry.Type == localfs.EntryDirectory }
func (e entryFileInfo) Sys() interface{}   { return nil }

// deviconHexForeground parses "#RRGGBB" into a tcell color. Returns ok false if invalid or empty.
func deviconHexForeground(hex string) (tcell.Color, bool) {
	if len(hex) != 7 || hex[0] != '#' {
		return tcell.ColorDefault, false
	}
	var rgb [3]int32
	for i := range rgb {
		part := hex[1+i*2 : 3+i*2]
		parsed, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return tcell.ColorDefault, false
		}
		rgb[i] = int32(parsed)
	}
	return tcell.NewRGBColor(rgb[0], rgb[1], rgb[2]), true
}
