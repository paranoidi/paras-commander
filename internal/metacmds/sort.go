package metacmds

import "sort"

// SortEntriesForDisplay returns entries from mf whose names appear in names,
// sorted by Order ascending then Name ascending.
func SortEntriesForDisplay(names []string, mf *MetaFile) []MetaEntry {
	if mf == nil || len(names) == 0 {
		return nil
	}
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}
	out := make([]MetaEntry, 0, len(names))
	for _, e := range mf.Entries {
		if _, ok := nameSet[e.Name]; ok {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// SortedEntries returns all entries from mf sorted by Order ascending then Name ascending.
func SortedEntries(mf *MetaFile) []MetaEntry {
	if mf == nil || len(mf.Entries) == 0 {
		return nil
	}
	out := append([]MetaEntry(nil), mf.Entries...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Name < out[j].Name
	})
	return out
}
