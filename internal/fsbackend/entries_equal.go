package fsbackend

import "time"

type listingEntryKey struct {
	name       string
	entryType  EntryType
	size       int64
	modifiedAt time.Time
}

func listingEntryKeys(entries []Entry) map[string]listingEntryKey {
	out := make(map[string]listingEntryKey, len(entries))
	for _, e := range entries {
		out[e.Name] = listingEntryKey{
			name:       e.Name,
			entryType:  e.Type,
			size:       e.Size,
			modifiedAt: e.ModifiedAt,
		}
	}
	return out
}

// EntriesListingEqual reports whether two directory snapshots have the same visible
// membership and per-name metadata (type, size, modification time).
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
		if other.entryType != e.Type || other.size != e.Size || !other.modifiedAt.Equal(e.ModifiedAt) {
			return false
		}
	}
	return true
}
