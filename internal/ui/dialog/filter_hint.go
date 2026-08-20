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
	FilterFocusFilesOnly   = 4
	FilterFocusDirsOnly    = 5
	FilterFocusCase        = 6
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

func filterDialogHeight(state FilterDialogState, layoutHeight int) int {
	// 3 radios + sep + label (doubles as the preview row) + blank + input + optional hint
	// + 2 checkbox rows (files/dirs only, case sensitive) + sep + buttons + borders.
	fixed := 3 + 1 + 1 + 1 + 1 + 2 + 1 + 2 // inner rows + top/bottom border rows
	if filterShowsPatternHint(state) {
		fixed++
	}
	height := fixed
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 14 {
		height = 14
	}
	return height
}

// FilterLastContentFocus returns the last navigable content index for the given mode.
func FilterLastContentFocus(mode panel.GroupPatternMode) int {
	if mode == panel.GroupPatternRegex {
		return FilterFocusDirsOnly
	}
	return FilterFocusCase
}

// FilterMoveFocus applies dialog navigation, including 2D checkbox layout: Files only and
// Directories only share a row (Left/Right); Case sensitive sits below.
func FilterMoveFocus(focus int, key tcell.Key, mode panel.GroupPatternMode) (int, bool) {
	form := NewDialogLinearForm(7)
	showCase := FilterShowsCaseSensitive(FilterDialogState{PatternMode: mode})

	okIdx := form.OKIndex()
	switch key {
	case tcell.KeyRight:
		if focus == FilterFocusFilesOnly {
			return FilterFocusDirsOnly, true
		}
		if focus == okIdx {
			return form.CancelIndex(), true
		}
		return focus, false
	case tcell.KeyLeft:
		if focus == FilterFocusDirsOnly {
			return FilterFocusFilesOnly, true
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
			return FilterFocusFilesOnly, true
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
			return FilterFocusFilesOnly, true
		}
	case tcell.KeyDown:
		switch focus {
		case FilterFocusFilesOnly:
			if showCase {
				return FilterFocusCase, true
			}
			return okIdx, true
		case FilterFocusDirsOnly:
			return okIdx, true
		case FilterFocusCase:
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
		case FilterFocusFilesOnly, FilterFocusDirsOnly:
			return FilterFocusPattern, true
		case FilterFocusCase:
			return FilterFocusFilesOnly, true
		case okIdx, form.CancelIndex():
			return FilterLastContentFocus(mode), true
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
