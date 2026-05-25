package dialog

import (
	"fmt"
	"unicode"
)

func metaEntryDisplayLabel(e MetaEntry) string {
	if e.Description != "" {
		return fmt.Sprintf("%s — %s", e.Name, e.Description)
	}
	return e.Name
}

// MetaEntryShortcuts returns the Alt mnemonic for each meta picker row. OK/Cancel
// letters (o, c) are reserved; duplicates across rows get the next available letter
// from each entry's display label.
func MetaEntryShortcuts(entries []MetaEntry) []rune {
	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = metaEntryDisplayLabel(e)
	}
	return assignDialogMnemonics(labels, nil, true)
}

// MetaEntryIndexForAltShortcut returns the entry index when r matches a row mnemonic
// (case-insensitive). The second result is false when there is no match.
func MetaEntryIndexForAltShortcut(entries []MetaEntry, r rune) (int, bool) {
	if r == 0 || !unicode.IsLetter(r) {
		return 0, false
	}
	shortcuts := MetaEntryShortcuts(entries)
	lr := unicode.ToLower(r)
	for i, sh := range shortcuts {
		if sh != 0 && sh == lr {
			return i, true
		}
	}
	return 0, false
}
