package dialog

import (
	"unicode"

	"github.com/paranoidi/paras-commander/internal/usermenu"
)

// UserMenuEntryShortcuts returns the Alt mnemonic for each user menu row. Cancel (c)
// and OK (o) letters are reserved; optional entry key is honored when set and free,
// otherwise the same dynamic picks as the meta dialog are used on title text.
func UserMenuEntryShortcuts(entries []usermenu.MenuEntry) []rune {
	labels := make([]string, len(entries))
	configured := make([]rune, len(entries))
	for i, e := range entries {
		labels[i] = e.Title
		configured[i] = configuredKeyRune(e.Key)
	}
	return assignDialogMnemonics(labels, configured, true)
}

// UserMenuEntryIndexForAltShortcut returns the entry index when r matches a row mnemonic.
func UserMenuEntryIndexForAltShortcut(entries []usermenu.MenuEntry, r rune) (int, bool) {
	if r == 0 || !unicode.IsLetter(r) {
		return 0, false
	}
	shortcuts := UserMenuEntryShortcuts(entries)
	lr := unicode.ToLower(r)
	for i, sh := range shortcuts {
		if sh != 0 && sh == lr {
			return i, true
		}
	}
	return 0, false
}
