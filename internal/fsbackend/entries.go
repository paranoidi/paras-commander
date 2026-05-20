package fsbackend

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ToPanelEntries converts backend rows to localfs entries for panel rendering.
// Entry.Path is the canonical pathloc string (host path or sftp:// URI).
func ToPanelEntries(entries []Entry) ([]localfs.Entry, error) {
	out := make([]localfs.Entry, len(entries))
	for i, e := range entries {
		out[i] = localfs.Entry{
			Name:       e.Name,
			Path:       e.Loc.String(),
			Type:       localTypeFromBackend(e.Type),
			Size:       e.Size,
			Mode:       e.Mode,
			ModifiedAt: e.ModifiedAt,
		}
	}
	return out, nil
}

func localTypeFromBackend(t EntryType) localfs.EntryType {
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
