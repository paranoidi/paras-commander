package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// massRenamePatternLabelText returns the Find/Pattern field's label with its live pattern-match
// count, e.g. "Pattern (12):". The count is omitted (plain "Pattern:") when the field is empty.
func massRenamePatternLabelText(state FileDialogState) string {
	label := "Find"
	pattern := ""
	if len(state.Fields) > 0 {
		label = state.Fields[0].Label
		pattern = state.Fields[0].Value
	}
	if pattern == "" {
		return label + ":"
	}
	return fmt.Sprintf("%s (%d):", label, state.MassRenameMatchCount)
}

// MassRenameMarkerWidth returns the drawn marker width for styles' radio/checkbox icons
// (mirrors DrawDialogRadio / DrawDialogCheckbox).
func MassRenameMarkerWidth(styles theme.Theme) int {
	m := 0
	for _, icon := range []string{styles.IconDialogRadio(false), styles.IconDialogRadio(true), styles.IconDialogCheckbox(false), styles.IconDialogCheckbox(true)} {
		m = max(m, utf8.RuneCountInString(" "+icon+" "))
	}
	return m
}
