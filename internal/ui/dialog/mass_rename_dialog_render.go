package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// massRenameDialogHeight returns the dialog outer height for mass rename.
// The dialog is sized identically for all three modes (Simple / Regex / ExternalEditor) so
// switching modes does not resize it. ExternalEditor skips the fields section at render time,
// which gives the preview area the freed rows automatically.
func massRenameDialogHeight(layoutHeight int, state FileDialogState) int {
	maxVP := massRenameSizingMaxPreviewRows(layoutHeight)
	previewCount := len(state.MassRenamePreviewBefore)
	if previewCount < 1 {
		previewCount = 1
	}
	vp := maxVP
	if previewCount < vp {
		vp = previewCount
	}
	// radio row + options checkbox row + sep before the fields section (always sized as if
	// Simple/Regex regardless of the actual mode, per the doc comment above).
	fixed := 1 + 1 + 1 + massRenameFieldsSectionRows(state)
	height := 1 + fixed + vp + 4 // top pad + body + sep + blank + buttons row + bottom border
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 12 {
		height = 12
	}
	return height
}

// massRenameSizingMaxPreviewRows estimates how many preview rows the Simple/Regex layout (the
// fixed sizing baseline — see massRenameDialogHeight's doc comment) could show at layoutHeight.
// Used only to pick how tall to make the dialog before its final height is known; not accurate
// enough for scroll paging or the scrollbar — see MassRenamePreviewViewportRows for that.
func massRenameSizingMaxPreviewRows(layoutHeight int) int {
	// 1 top pad + (radio row + options row + sep + 2 fields x2 rows + sep) + sep + blank +
	// buttons row + bottom border.
	maxBody := layoutHeight - 13
	if maxBody < 3 {
		maxBody = 3
	}
	return maxBody
}

// massRenameFieldsSectionRows returns the row count of the Simple/Regex fields section: two
// fields (label, input each), a separator, and any visible regex replacement hint row. The
// pattern compile-error hint shares the Pattern/Find label row (right-aligned) rather than
// consuming its own row — see massRenamePatternLabelText and its use in drawMassRenameDialog.
// Shared by massRenameDialogHeight (which always sizes for this section, regardless of the
// actual mode) and massRenameFixedRows (which only counts it when the mode is Simple/Regex).
func massRenameFieldsSectionRows(state FileDialogState) int {
	rows := 4 + 1 // two fields (label+input each) + separator
	if massRenameShowsReplacementHint(state) {
		rows++
	}
	return rows
}

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

// massRenameFixedRows returns the number of dialog rows consumed above the preview list for
// state's mode: mode radios, options row, separators, and (for Simple/Regex/Capitalize) the
// fields or checkboxes section, including any visible regex hint rows.
func massRenameFixedRows(state FileDialogState) int {
	fixed := 1 + 1 + 1 // radio row + options row + separator
	switch state.MassRenameMode {
	case MassRenameModeUIExternalEditor:
		// No fields/checkboxes section.
	case MassRenameModeUICapitalize:
		fixed += 2 + 1 // two checkboxes + separator
	default:
		fixed += massRenameFieldsSectionRows(state)
	}
	return fixed
}

// massRenamePreviewViewportRowsForHeight returns the preview row count visible in a mass
// rename dialog of dialogHeight for state — the exact geometry drawMassRenameDialog paints.
func massRenamePreviewViewportRowsForHeight(dialogHeight int, state FileDialogState) int {
	// 5 = top pad (1) + sep + blank + buttons row + bottom border (4), mirroring
	// massRenameDialogHeight's "1 + fixed + vp + 4" (dialogHeight already bakes in fixed).
	vp := dialogHeight - 5 - massRenameFixedRows(state)
	if vp < 1 {
		vp = 1
	}
	return vp
}

// MassRenamePreviewViewportRows returns the preview page size for PgUp/PgDn scrolling and the
// scrollbar, matching exactly what drawMassRenameDialog draws for state at layoutHeight (the
// same value FileDialogRect uses to size the dialog) — the single source of truth so paging
// and the visual scrollbar never drift apart.
func MassRenamePreviewViewportRows(layoutHeight int, state FileDialogState) int {
	return massRenamePreviewViewportRowsForHeight(massRenameDialogHeight(layoutHeight, state), state)
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
