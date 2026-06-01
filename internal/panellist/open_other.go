package panellist

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// EntryOpenInOtherPanel reports whether the other panel's current path equals this directory entry.
func EntryOpenInOtherPanel(entry localfs.Entry, otherPanelPath string) bool {
	if entry.Type != localfs.EntryDirectory || otherPanelPath == "" {
		return false
	}
	want, wantErr := pathloc.Parse(otherPanelPath)
	entryLoc, entryErr := pathloc.Parse(entry.Path)
	if wantErr == nil && entryErr == nil {
		return entryLoc.Equal(want)
	}
	return filepath.Clean(entry.Path) == filepath.Clean(otherPanelPath)
}
