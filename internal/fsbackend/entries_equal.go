package fsbackend

import (
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

type listingEntryKey struct {
	name       string
	entryType  EntryType
	size       int64
	modifiedAt time.Time
	indexState string
}

// listingIndexState is the fileindex overlay state that counts when comparing listings. The
// drift pass rewrites an indexed directory's state on the panel after its listing landed
// (indexed, tagged, unchecked and differs), so a fresh overlay can never be compared against
// those four exactly: they are one state here. Every other state (new, missing, changed...)
// compares as is, which is what lets a file turning into a ghost row, or back, count as a change.
func listingIndexState(e Entry) string {
	if e.Type == EntryDirectory {
		switch e.IndexStatus {
		case localfs.IndexStatusIndexed, localfs.IndexStatusTagged, localfs.IndexStatusUnchecked, localfs.IndexStatusDiffers:
			return localfs.IndexStatusIndexed
		}
	}
	return e.IndexStatus
}

func listingEntryKeys(entries []Entry) map[string]listingEntryKey {
	out := make(map[string]listingEntryKey, len(entries))
	for _, e := range entries {
		out[e.Name] = listingEntryKey{
			name:       e.Name,
			entryType:  e.Type,
			size:       e.Size,
			modifiedAt: e.ModifiedAt,
			indexState: listingIndexState(e),
		}
	}
	return out
}

// EntriesListingEqual reports whether two directory snapshots have the same visible
// membership and per-name metadata (type, size, modification time, fileindex overlay state).
func EntriesListingEqual(a, b []Entry) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	bMap := listingEntryKeys(b)
	for _, e := range a {
		other, ok := bMap[e.Name]
		if !ok {
			return false
		}
		if other.entryType != e.Type || other.size != e.Size || !other.modifiedAt.Equal(e.ModifiedAt) || other.indexState != listingIndexState(e) {
			return false
		}
	}
	return true
}
