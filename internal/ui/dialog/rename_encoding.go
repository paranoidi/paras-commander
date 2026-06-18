package dialog

// RenameEncodingOptionCount returns the number of focusable encoding radio rows.
func RenameEncodingOptionCount(state FileDialogState) int {
	if state.RenamePhase != RenamePhaseEncoding {
		return 0
	}
	return len(state.RenameEncodingCandidates)
}

// RenameEncodingPreviewText returns the UTF-8 preview for the selected encoding candidate.
func RenameEncodingPreviewText(state FileDialogState) string {
	if state.RenamePhase != RenamePhaseEncoding {
		return ""
	}
	cands := state.RenameEncodingCandidates
	if len(cands) == 0 {
		return ""
	}
	idx := state.RenameEncodingSelected
	if idx < 0 || idx >= len(cands) {
		idx = 0
	}
	return cands[idx].UTF8
}

// RenameEncodingOptionLabel returns the radio label for encoding candidate i.
func RenameEncodingOptionLabel(state FileDialogState, i int) string {
	cands := state.RenameEncodingCandidates
	if i < 0 || i >= len(cands) {
		return ""
	}
	return cands[i].Label
}

// RenameEncodingOptionShortcut returns the Alt+letter shortcut for encoding candidate i.
func RenameEncodingOptionShortcut(state FileDialogState, i int) rune {
	return RenameEncodingCandidateShortcut(RenameEncodingOptionLabel(state, i))
}

// RenameEncodingCandidateShortcut returns the Alt+letter shortcut for a candidate label.
func RenameEncodingCandidateShortcut(label string) rune {
	if label == "" {
		return 0
	}
	for _, r := range label {
		switch r {
		case ' ', '-', '(', ')':
			continue
		default:
			if r >= 'a' && r <= 'z' {
				return r - ('a' - 'A')
			}
			return r
		}
	}
	return 0
}
