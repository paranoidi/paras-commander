package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	FilterFocusShellRadio  = 0
	FilterFocusRegexRadio  = 1
	FilterFocusSimpleRadio = 2
	FilterFocusPattern     = 3
	FilterFocusDirsOnly    = 4
	FilterFocusCase        = 5
	FilterFocusIncludeMeta = 6
	FilterFocusOnlyMeta    = 7
)

// FilterShowsCaseSensitive reports whether the case-sensitive checkbox is shown.
func FilterShowsCaseSensitive(state FilterDialogState) bool {
	return state.PatternMode != panel.GroupPatternRegex
}

func filterPatternHintText(state FilterDialogState) string {
	if state.PatternMode != panel.GroupPatternRegex && state.PatternMode != panel.GroupPatternShell {
		return ""
	}
	pat := strings.TrimSpace(state.Text)
	if pat == "" {
		return ""
	}
	_, err := panel.NewGroupMatcher(pat, state.PatternMode, state.CaseSensitive)
	if err == nil {
		return ""
	}
	msg := err.Error()
	const regexPrefix = "invalid regexp: "
	const shellPrefix = "invalid shell pattern: "
	if strings.HasPrefix(msg, regexPrefix) {
		msg = strings.TrimPrefix(msg, regexPrefix)
	} else if strings.HasPrefix(msg, shellPrefix) {
		msg = strings.TrimPrefix(msg, shellPrefix)
	}
	if msg == "" {
		if state.PatternMode == panel.GroupPatternRegex {
			return "invalid regexp"
		}
		return "invalid shell pattern"
	}
	return msg
}

func filterShowsPatternHint(state FilterDialogState) bool {
	return filterPatternHintText(state) != ""
}

func filterPatternHintStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	errFG, _, _ := styles.DialogInputActiveError.Decompose()
	return styles.DialogText.Foreground(errFG).Background(dbg)
}

// filterPatternInvalid reports whether the pattern input row should render in the dialog's
// error style: an uncompilable pattern, or a valid pattern that currently matches nothing.
func filterPatternInvalid(state FilterDialogState) bool {
	if strings.TrimSpace(state.Text) == "" {
		return false
	}
	if filterShowsPatternHint(state) {
		return true
	}
	return state.PreviewShow && state.PreviewFiles == 0 && state.PreviewFolders == 0
}

func filterDialogInnerRows(state FilterDialogState) int {
	rows := 3 + 1 + 1 + 1 + 2 + 1 + 1 // mode radios, separator, Pattern label, input, filter checkboxes, separator, buttons
	if filterShowsPatternHint(state) {
		rows++
	}
	if state.MetaColumnCount > 0 {
		rows++
	}
	return rows
}

func filterDialogHeight(state FilterDialogState, layoutHeight int) int {
	height := filterDialogInnerRows(state) + 2 // top and bottom border rows
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	return height
}

// FilterLastContentFocus returns the last navigable content index for the given mode and meta
// column count.
func FilterLastContentFocus(mode panel.GroupPatternMode, metaColumnCount int) int {
	if metaColumnCount > 0 {
		return FilterFocusIncludeMeta // leftmost item on the meta row
	}
	if mode == panel.GroupPatternRegex {
		return FilterFocusDirsOnly
	}
	return FilterFocusCase
}

// FilterMoveFocus applies dialog navigation, including 2D checkbox layout: Directories only
// sits above Case sensitive. When metaColumnCount > 0,
// "Include meta columns" and "Only meta columns" share a row below case sensitive.
func FilterMoveFocus(focus int, key tcell.Key, mode panel.GroupPatternMode, metaColumnCount int) (int, bool) {
	numContent := 6
	if metaColumnCount > 0 {
		numContent += 2 // IncludeMeta + OnlyMeta
	}
	form := NewDialogLinearForm(numContent)
	showCase := FilterShowsCaseSensitive(FilterDialogState{PatternMode: mode})
	showMeta := metaColumnCount > 0

	okIdx := form.OKIndex()
	switch key {
	case tcell.KeyRight:
		if focus == FilterFocusIncludeMeta && showMeta {
			return FilterFocusOnlyMeta, true
		}
		if focus == okIdx {
			return form.CancelIndex(), true
		}
		return focus, false
	case tcell.KeyLeft:
		if focus == FilterFocusOnlyMeta && showMeta {
			return FilterFocusIncludeMeta, true
		}
		if focus == form.CancelIndex() {
			return okIdx, true
		}
		return focus, false
	case tcell.KeyTab:
		// Segment jumps: mode radios(0-2) → pattern(3) → filters(4+) → buttons → mode radios
		switch {
		case focus < FilterFocusPattern:
			return FilterFocusPattern, true
		case focus == FilterFocusPattern:
			return FilterFocusDirsOnly, true
		case focus < okIdx:
			return okIdx, true
		default:
			return FilterFocusShellRadio, true
		}
	case tcell.KeyBacktab:
		// Reverse segment jumps
		switch {
		case focus < FilterFocusPattern:
			return okIdx, true
		case focus == FilterFocusPattern:
			return FilterFocusShellRadio, true
		case focus < okIdx:
			return FilterFocusPattern, true
		default:
			return FilterFocusDirsOnly, true
		}
	case tcell.KeyDown:
		switch focus {
		case FilterFocusDirsOnly:
			if showCase {
				return FilterFocusCase, true
			}
			if showMeta {
				return FilterFocusIncludeMeta, true
			}
			return okIdx, true
		case FilterFocusCase:
			if showMeta {
				return FilterFocusIncludeMeta, true
			}
			return okIdx, true
		case FilterFocusIncludeMeta, FilterFocusOnlyMeta:
			return okIdx, true
		default:
			next, ok := form.MoveFocus(focus, tcell.KeyDown)
			if !ok {
				return focus, false
			}
			return filterSkipHiddenCase(next, mode), true
		}
	case tcell.KeyUp:
		switch focus {
		case FilterFocusDirsOnly:
			return FilterFocusPattern, true
		case FilterFocusCase:
			return FilterFocusDirsOnly, true
		case FilterFocusIncludeMeta, FilterFocusOnlyMeta:
			if showCase {
				return FilterFocusCase, true
			}
			return FilterFocusDirsOnly, true
		case okIdx, form.CancelIndex():
			return FilterLastContentFocus(mode, metaColumnCount), true
		default:
			next, ok := form.MoveFocus(focus, tcell.KeyUp)
			if !ok {
				return focus, false
			}
			return filterSkipHiddenCase(next, mode), true
		}
	default:
		next, ok := form.MoveFocus(focus, key)
		if !ok {
			return focus, false
		}
		return filterSkipHiddenCase(next, mode), true
	}
}

func filterSkipHiddenCase(focus int, mode panel.GroupPatternMode) int {
	if mode == panel.GroupPatternRegex && focus == FilterFocusCase {
		return FilterFocusDirsOnly
	}
	return focus
}
