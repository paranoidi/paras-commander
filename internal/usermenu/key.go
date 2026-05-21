package usermenu

import "strings"

// EntryIndexForKey returns the entry index when r matches the entry key's first rune (case-insensitive).
func EntryIndexForKey(entries []MenuEntry, r rune) (int, bool) {
	for i, e := range entries {
		k := []rune(strings.TrimSpace(e.Key))
		if len(k) == 0 {
			continue
		}
		if keyRuneMatches(k[0], r) {
			return i, true
		}
	}
	return -1, false
}

func keyRuneMatches(key, r rune) bool {
	if key == r {
		return true
	}
	if key >= 'A' && key <= 'Z' && key+32 == r {
		return true
	}
	if key >= 'a' && key <= 'z' && key-32 == r {
		return true
	}
	return false
}
