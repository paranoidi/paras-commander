package dialog

import "fmt"

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
