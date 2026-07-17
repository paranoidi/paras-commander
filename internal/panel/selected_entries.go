package panel

import (
	"sort"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// SelectedEntries returns selected entries in stable sorted path order.
// Listing hits use listingByPath; off-listing paths call resolve (required when
// any selection is off-listing). When skipMissing is true, resolve errors are skipped.
func (s *State) SelectedEntries(skipMissing bool, resolve func(path string) (localfs.Entry, error)) ([]localfs.Entry, error) {
	if len(s.SelectedPaths) == 0 {
		return nil, nil
	}
	if resolve == nil {
		resolve = localfs.EntryFromPath
	}
	paths := make([]string, 0, len(s.SelectedPaths))
	for path, on := range s.SelectedPaths {
		if !on {
			continue
		}
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	entries := make([]localfs.Entry, 0, len(paths))
	for _, path := range paths {
		if e, ok := s.listingEntry(path); ok {
			entries = append(entries, e)
			continue
		}
		e, err := resolve(path)
		if err != nil {
			if skipMissing {
				continue
			}
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
