package fsbackend

import "github.com/paranoidi/paras-commander/internal/localfs"

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

// sameListingFacts reports whether two rows of one name show the same listing metadata.
func sameListingFacts(a, b *Entry) bool {
	return a.Type == b.Type && a.Size == b.Size && a.ModifiedAt.Equal(b.ModifiedAt) &&
		listingIndexState(*a) == listingIndexState(*b) &&
		a.Mode == b.Mode && a.Owner == b.Owner && a.Group == b.Group && a.Links == b.Links &&
		a.ChangedAt.Equal(b.ChangedAt)
}

// EntriesListingEqual reports whether two directory snapshots have the same visible
// membership and per-name metadata (type, size, modification time, mode, owner, group, links,
// ctime, fileindex overlay state). Access time is left out: reading a file changes it, so it
// would re-apply listings endlessly, and an Accessed column updates with the next change or reload.
func EntriesListingEqual(a, b []Entry) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	byName := make(map[string]int, len(b))
	for i := range b {
		byName[b[i].Name] = i
	}
	for i := range a {
		j, ok := byName[a[i].Name]
		if !ok || !sameListingFacts(&a[i], &b[j]) {
			return false
		}
	}
	return true
}
