package dialog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func DrawGroupSelectDialog(screen tcell.Screen, layout Layout, state GroupSelectState, styles theme.Theme) {
	title := "Unselect group"
	if state.Mode == "select" {
		title = "Select group"
	}

	width := PatternDialogWidth
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 30 {
		return
	}

	height := groupSelectDialogHeight(state, layout.Height)
	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	itemBg := dbg
	textX := draw.DialogTextX(rect)
	optionX := draw.DialogOptionX(rect)
	inputWidth := draw.DialogContentWidth(rect)

	y := rect.Y + 1
	innerBottom := rect.Y + rect.Height - 2

	// Radios in the left column, type checkboxes in two columns to their right (same layout as
	// the Filter dialog).
	typeCol1X := optionX + utf8.RuneCountInString(draw.RadioText("Regular expression", false)) + 2
	typeCol2X := typeCol1X + utf8.RuneCountInString(" "+draw.CheckboxText("Pictures", false)) + 2
	radios := []struct {
		label string
		key   rune
		mode  panel.GroupPatternMode
		focus int
	}{
		{"Shell patterns", PatternMnemonicShell, panel.GroupPatternShell, GroupSelectFocusShellRadio},
		{"Regular expression", PatternMnemonicRegex, panel.GroupPatternRegex, GroupSelectFocusRegexRadio},
		{"Simple", PatternMnemonicSimple, panel.GroupPatternSimple, GroupSelectFocusSimpleRadio},
	}
	for i, r := range radios {
		if y >= innerBottom {
			return
		}
		draw.DrawDialogRadio(screen, optionX, y, r.label, r.key, state.PatternMode == r.mode, state.Focus == r.focus, false, styles)
		for t := i; t < FilterTypeCount; t += 3 {
			x := typeCol1X
			if t >= 3 {
				x = typeCol2X
			}
			draw.DrawDialogCheckbox(screen, x, y, FilterTypeLabels[t], FilterTypeMnemonics[t], state.Types[t], state.Focus == GroupSelectFocusTypeFirst+t, PatternTypesDisabled(state.DirsOnly), styles)
		}
		y++
	}
	if y >= innerBottom {
		return
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	if y >= innerBottom {
		return
	}

	primitive.Text(screen, textX, y, inputWidth, "Pattern:", styles.DialogText.Background(itemBg))
	if preview := groupSelectPreviewText(state, styles); preview != "" {
		pw := utf8.RuneCountInString(preview)
		px := textX + inputWidth - pw
		if px > textX+len("Pattern:") {
			primitive.Text(screen, px, y, pw, preview, styles.DialogText.Background(itemBg))
		}
	}
	y++
	if y >= innerBottom {
		return
	}
	// The row after the input is reserved for the compile-error hint so the dialog never grows
	// when the hint appears.
	draw.DrawScrollingDialogInput(screen, textX, y, inputWidth, draw.ScrollingInputState{Value: state.Text, Cursor: state.TextCursor, Scroll: state.TextScroll}, state.Focus == GroupSelectFocusPattern, groupSelectPatternInvalid(state), styles)
	y++
	if y >= innerBottom {
		return
	}

	if hint := groupSelectPatternHintText(state); hint != "" {
		primitive.Text(screen, textX, y, inputWidth, hint, groupSelectPatternHintStyle(styles, dbg))
	}
	y++ // reserved hint row (blank when pattern is valid)
	if y >= innerBottom {
		return
	}

	// col2X aligns the second checkbox on every two-column row with the type checkboxes above.
	col2X := typeCol1X
	draw.DrawDialogRadio(screen, optionX, y, "Files only", PatternMnemonicFilesOnly, state.FilesOnly, state.Focus == GroupSelectFocusFilesOnly, false, styles)
	draw.DrawDialogRadio(screen, col2X, y, "Directories only", PatternMnemonicDirsOnly, state.DirsOnly, state.Focus == GroupSelectFocusDirsOnly, false, styles)
	y++
	if y >= innerBottom {
		return
	}

	draw.DrawDialogCheckbox(screen, optionX, y, "Case sensitive", PatternMnemonicCase, state.CaseSensitive, state.Focus == GroupSelectFocusCase, false, styles)
	if state.Context == "find" {
		draw.DrawDialogCheckbox(screen, col2X, y, "Match full path", PatternMnemonicFullPath, state.FullPath, state.Focus == GroupSelectFocusFullPath, false, styles)
	}
	y++
	if y >= innerBottom {
		return
	}

	if state.MetaColumnCount > 0 {
		draw.DrawDialogCheckbox(screen, optionX, y, "Include meta columns", PatternMnemonicIncludeMeta, state.IncludeMetaColumns, state.Focus == GroupSelectFocusIncludeMeta, false, styles)
		draw.DrawDialogCheckbox(screen, col2X, y, "Only meta columns", PatternMnemonicOnlyMeta, state.OnlyMetaColumns, state.Focus == GroupSelectFocusOnlyMeta, false, styles)
		y++
		if y >= innerBottom {
			return
		}
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)

	form := NewDialogLinearForm(GroupSelectContentCount(state.MetaColumnCount))
	buttonY := rect.Y + rect.Height - 2
	draw.DrawOKCancelButtonRow(screen, rect, buttonY, state.Focus == form.OKIndex(), state.Focus == form.CancelIndex(), styles)
}

// groupSelectPreviewText formats the live result preview shown on the Pattern row:
// "<files> <file-icon> <folders> <folder-icon>", omitting either count when it's zero. Empty
// when the preview is hidden or both counts are zero.
func groupSelectPreviewText(state GroupSelectState, styles theme.Theme) string {
	if !state.PreviewShow {
		return ""
	}
	var parts []string
	if state.PreviewFiles > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", state.PreviewFiles, styles.IconFile()))
	}
	if state.PreviewFolders > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", state.PreviewFolders, styles.IconFolder()))
	}
	return strings.Join(parts, " ")
}
