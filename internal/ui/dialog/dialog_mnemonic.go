package dialog

import (
	"strings"
	"unicode"
)

// dialogReservedOKCancelShortcut reports runes reserved for dialog OK/Cancel (Alt+O / Alt+C).
func dialogReservedOKCancelShortcut(r rune) bool {
	switch unicode.ToLower(r) {
	case 'o', 'c':
		return true
	default:
		return false
	}
}

func splitDialogLabelWords(label string) []string {
	parts := strings.FieldsFunc(label, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '—' || r == '_' || r == '/'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstLetterInWord(word string) rune {
	for _, r := range word {
		if unicode.IsLetter(r) {
			return r
		}
	}
	return 0
}

func secondLetterInWord(word string) rune {
	n := 0
	for _, r := range word {
		if unicode.IsLetter(r) {
			n++
			if n == 2 {
				return r
			}
		}
	}
	return 0
}

// dialogMnemonicCandidates returns Alt mnemonic candidates for label text: menu-style
// picks (first letter, second word, second letter of first word, third word), then
// remaining letters left-to-right. Duplicates are omitted.
func dialogMnemonicCandidates(label string) []rune {
	words := splitDialogLabelWords(label)
	var out []rune
	seen := make(map[rune]struct{})
	add := func(r rune) {
		if r == 0 || !unicode.IsLetter(r) {
			return
		}
		lr := unicode.ToLower(r)
		if _, ok := seen[lr]; ok {
			return
		}
		seen[lr] = struct{}{}
		out = append(out, lr)
	}
	if len(words) > 0 {
		add(firstLetterInWord(words[0]))
	}
	if len(words) > 1 {
		add(firstLetterInWord(words[1]))
	}
	if len(words) > 0 {
		add(secondLetterInWord(words[0]))
	}
	if len(words) > 2 {
		add(firstLetterInWord(words[2]))
	}
	for _, r := range label {
		if unicode.IsLetter(r) {
			add(r)
		}
	}
	return out
}

// ConfiguredKeyRune returns the lowercase letter pinned by a configured shortcut
// string (e.g. a menu entry's `key` field), or 0 when key has no letter.
func ConfiguredKeyRune(key string) rune {
	k := strings.TrimSpace(key)
	if k == "" {
		return 0
	}
	for _, r := range k {
		if unicode.IsLetter(r) {
			return unicode.ToLower(r)
		}
	}
	return 0
}

func tryAssignDialogMnemonic(dst *rune, cand rune, used map[rune]struct{}, reserveOKCancel bool) bool {
	if cand == 0 || !unicode.IsLetter(cand) {
		return false
	}
	lr := unicode.ToLower(cand)
	if reserveOKCancel && dialogReservedOKCancelShortcut(lr) {
		return false
	}
	if _, taken := used[lr]; taken {
		return false
	}
	*dst = lr
	used[lr] = struct{}{}
	return true
}

// ItemMnemonics returns one activation letter per label (no OK/Cancel reservation).
func ItemMnemonics(labels []string, configured []rune) []rune {
	return assignDialogMnemonics(labels, configured, false)
}

// ItemIndexForMnemonic returns the item index whose activation letter matches r (case-insensitive).
func ItemIndexForMnemonic(labels []string, configured []rune, r rune) (int, bool) {
	if r == 0 || !unicode.IsLetter(r) {
		return 0, false
	}
	shortcuts := assignDialogMnemonics(labels, configured, false)
	lr := unicode.ToLower(r)
	for i, sh := range shortcuts {
		if sh != 0 && sh == lr {
			return i, true
		}
	}
	return 0, false
}

// assignDialogMnemonics returns one Alt mnemonic per label. When reserveOKCancel is true,
// o and c are reserved for buttons. configured[i], when non-zero, is tried before dynamic
// picks from label; if it is reserved, taken, or invalid, dynamic allocation is used.
func assignDialogMnemonics(labels []string, configured []rune, reserveOKCancel bool) []rune {
	shortcuts := make([]rune, len(labels))
	used := map[rune]struct{}{}
	if reserveOKCancel {
		used['o'] = struct{}{}
		used['c'] = struct{}{}
	}
	for i, label := range labels {
		var cfg rune
		if i < len(configured) {
			cfg = configured[i]
		}
		if tryAssignDialogMnemonic(&shortcuts[i], cfg, used, reserveOKCancel) {
			continue
		}
		for _, cand := range dialogMnemonicCandidates(label) {
			if tryAssignDialogMnemonic(&shortcuts[i], cand, used, reserveOKCancel) {
				break
			}
		}
	}
	return shortcuts
}
