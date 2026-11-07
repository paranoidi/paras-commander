package keymap

import (
	"strings"
	"unicode"
)

// fileIndexMenuGroups is the fileindex menu's grouped layout. Tools stacks under Tagging in the
// same macro column; every other group has a column of its own.
var fileIndexMenuGroups = leaderMenuViewSpec{
	order: []string{"Browse", "Tagging", "Tools", "Actions", "Duplicates"},
	actions: map[string][]string{
		"Browse":     {ActionFileIndexBrowseWith, ActionFileIndexBrowse, ActionFileIndexReveal},
		"Tagging":    {ActionFileIndexTags},
		"Tools":      {ActionFileIndexFind, ActionFileIndexReportDifferences},
		"Actions":    {ActionFileIndexScan, ActionFileIndexScanFast, ActionFileIndexPrune},
		"Duplicates": {ActionFileIndexFindDuplicates, ActionFileIndexCopies, ActionFileIndexCopiesOutside},
	},
	column: map[string]int{"Browse": 0, "Tagging": 1, "Tools": 1, "Actions": 2, "Duplicates": 3},
}

// DefaultFileIndexMenuKeys returns built-in default fileindex-menu letters per action. Each is
// its action's default chord letter in lowercase (every fileindex chord is Alt+Shift+letter),
// except fileindex.scan-fast (uppercase of fileindex.scan's letter) and fileindex.find-duplicates
// (uppercase of fileindex.find's letter).
func DefaultFileIndexMenuKeys() map[string]string {
	out := make(map[string]string)
	for _, spec := range DefaultActionSpecs() {
		if k := strings.TrimSpace(spec.FileIndexMenuKey); k != "" {
			out[spec.ID] = k
		}
	}
	return out
}

func validateFileIndexMenuKeys(keys map[string]string) error {
	return validateMenuKeys(FileIndexMenuShortcutsTable, keys, unicode.IsLetter, "a letter")
}

// FileIndexMenuEntries returns the fileindex menu rows from the bundle's merged keys, grouped
// under a header per column; a group with no bound actions is omitted.
func (b *Bundle) FileIndexMenuEntries() []LeaderMenuEntry {
	if b == nil {
		return nil
	}
	g := fileIndexMenuGroups
	return buildLeaderMenuEntries(b.FileIndexMenuKey, g.order, g.actions, g.column)
}
