package fsbackend

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ToPanelEntry converts one backend row to a localfs entry for panel rendering.
// Entry.Path is the canonical pathloc string (host path or sftp:// URI).
func ToPanelEntry(e Entry) localfs.Entry {
	return localfs.Entry{
		Name:         e.Name,
		Path:         e.Loc.String(),
		Type:         LocalTypeFromBackend(e.Type),
		Size:         e.Size,
		Mode:         e.Mode,
		ModifiedAt:   e.ModifiedAt,
		Dev:          e.Dev,
		DevValid:     e.DevValid,
		AccessDenied: e.AccessDenied,
	}
}

// ToPanelEntries converts backend rows to localfs entries for panel rendering.
func ToPanelEntries(entries []Entry) ([]localfs.Entry, error) {
	out := make([]localfs.Entry, len(entries))
	for i, e := range entries {
		out[i] = ToPanelEntry(e)
	}
	return out, nil
}

// FromPanelEntry converts a panel/localfs entry to a backend Entry (Loc may be zero on parse failure).
func FromPanelEntry(e localfs.Entry) Entry {
	loc, _ := pathloc.Parse(e.Path)
	return Entry{
		Name:         e.Name,
		Loc:          loc,
		Type:         BackendTypeFromLocal(e.Type),
		Size:         e.Size,
		Mode:         e.Mode,
		ModifiedAt:   e.ModifiedAt,
		Dev:          e.Dev,
		DevValid:     e.DevValid,
		AccessDenied: e.AccessDenied,
	}
}

// LocalTypeFromBackend maps backend entry types to localfs types.
func LocalTypeFromBackend(t EntryType) localfs.EntryType {
	switch t {
	case EntryDirectory:
		return localfs.EntryDirectory
	case EntrySymlink:
		return localfs.EntrySymlink
	case EntryOther:
		return localfs.EntryOther
	default:
		return localfs.EntryFile
	}
}

// BackendTypeFromLocal maps localfs entry types to backend types.
func BackendTypeFromLocal(t localfs.EntryType) EntryType {
	switch t {
	case localfs.EntryDirectory:
		return EntryDirectory
	case localfs.EntrySymlink:
		return EntrySymlink
	case localfs.EntryOther:
		return EntryOther
	default:
		return EntryFile
	}
}

// HasDotfileNames reports whether any entry name is dot-prefixed.
func HasDotfileNames(entries []Entry) bool {
	for _, e := range entries {
		if len(e.Name) > 0 && e.Name[0] == '.' {
			return true
		}
	}
	return false
}

// FilterHidden drops dotfile names when showHidden is false.
func FilterHidden(entries []Entry, showHidden bool) []Entry {
	if showHidden {
		return entries
	}
	out := entries[:0]
	for _, e := range entries {
		if len(e.Name) > 0 && e.Name[0] == '.' {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ErrNotPanelEntry reports conversion failure for a single row.
type ErrNotPanelEntry struct {
	Name string
	Err  error
}

func (e ErrNotPanelEntry) Error() string {
	return fmt.Sprintf("entry %q: %v", e.Name, e.Err)
}
