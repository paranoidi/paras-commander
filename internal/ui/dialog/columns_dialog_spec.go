package dialog

import "github.com/paranoidi/paras-commander/internal/panel"

// metaEntryDisplayLabel is an entry's label in the checkbox grid; its description has its own row
// (columnsFocusedDescription).
func metaEntryDisplayLabel(e MetaEntry) string {
	if e.Label != "" {
		return e.Label
	}
	return e.Name
}

// ColumnsDialogShortcuts returns the Alt mnemonic of each checkbox: the file columns' pinned ones,
// then one per meta entry. OK/Cancel letters (o, c) are reserved; duplicates across rows get the
// next available letter from each entry's display label.
func ColumnsDialogShortcuts(entries []MetaEntry) []rune {
	specs := panel.Columns()
	labels := make([]string, 0, len(specs)+len(entries))
	pinned := make([]rune, 0, len(specs)+len(entries))
	for _, spec := range specs {
		labels = append(labels, spec.Title)
		pinned = append(pinned, spec.Mnemonic)
	}
	for _, e := range entries {
		labels = append(labels, metaEntryDisplayLabel(e))
		pinned = append(pinned, 0)
	}
	return assignDialogMnemonics(labels, pinned, true)
}
