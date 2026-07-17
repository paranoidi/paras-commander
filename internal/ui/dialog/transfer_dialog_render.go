package dialog

import (
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// TransferListMaxRows caps visible preview-list rows per paint in the multi-location
// layout (full list still scrolls).
const TransferListMaxRows = 10

func transferDialogMaxHeight(layoutHeight int) int {
	maxH := layoutHeight * 80 / 100
	if maxH > layoutHeight-2 {
		maxH = layoutHeight - 2
	}
	if maxH < 8 {
		maxH = 8
	}
	return maxH
}

// transferMultiChromeRows returns the interior row count for the multi-location layout
// other than the scrollable preview list itself (top/bottom border added separately):
// Source label, blank, root path, blank, Destination label, blank, input row, separator,
// preserve-permissions/timestamps checkboxes (copy only) + flatten checkbox, "Result"
// separator, separator, blank, button row.
func transferMultiChromeRows(kind TransferKind) int {
	checkboxRows := 1 // Flatten into destination
	if kind == TransferKindCopy {
		checkboxRows += 2 // Preserve permissions + Preserve timestamps
	}
	const fixedRows = 12 // 8 rows through the destination separator + Result separator(1) + separator/blank/button(3)
	return fixedRows + checkboxRows
}

// TransferListViewportRows returns how many preview-list rows fit for the given layout
// and dialog state. Exported so the key handler clamps EntriesScroll with the same number.
func TransferListViewportRows(layout Layout, st TransferDialogState) int {
	n := len(st.Entries)
	capped := n
	if capped > TransferListMaxRows {
		capped = TransferListMaxRows
	}
	if capped < 1 {
		capped = 1
	}
	maxH := transferDialogMaxHeight(layout.Height)
	available := maxH - transferMultiChromeRows(st.Kind) - 2 // minus top/bottom border
	if available < 1 {
		available = 1
	}
	if capped <= available {
		return capped
	}
	return available
}

// transferEntryLabel returns the preview-list label for one entry: root-relative
// structure label, or (when flatten is true) the basename with a trailing "/" for
// directories.
func transferEntryLabel(e DeleteListEntry, flatten bool) string {
	if flatten {
		base := filepath.Base(e.Path)
		if e.Type == localfs.EntryDirectory && base != "" {
			return base + "/"
		}
		return base
	}
	if e.Type == localfs.EntryDirectory && e.Name != "" {
		return e.Name + "/"
	}
	return e.Name
}

// transferMultiDialogWidth computes the multi-location dialog width from the common-root
// path and the entry structure labels (always un-flattened, so the dialog never resizes
// when Flatten into destination is toggled).
func transferMultiDialogWidth(layout Layout, state TransferDialogState, userHomeDir string, iconLead int) int {
	width := PreferredFormDialogWidth
	rootDisp := primitive.PathWithHomeTilde(state.CommonRoot, userHomeDir)
	if w := utf8.RuneCountInString(rootDisp) + 4; w > width {
		width = w
	}
	for _, e := range state.Entries {
		w := utf8.RuneCountInString(transferEntryLabel(e, false)) + 4 + iconLead
		if w > width {
			width = w
		}
	}
	if width > layout.Width {
		width = layout.Width
	}
	return width
}

func DrawTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, styles theme.Theme, userHomeDir string, showIcons bool, iconLead int, paintIcon DeleteRowIconPainter) {
	if state.MultiLocation() {
		drawMultiLocationTransferDialog(screen, layout, state, styles, userHomeDir, showIcons, iconLead, paintIcon)
		return
	}

	width := PreferredFormDialogWidth
	height := 10
	title := "Copy"
	if state.Kind == TransferKindMove {
		height = 8
		title = "Move"
	}
	if state.Phase == TransferPhaseSelfCopyRename {
		height = 9
		if state.Kind == TransferKindCopy {
			title = "Copy — New name"
		} else {
			title = "Move — New name"
		}
	}

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()

	if state.Phase == TransferPhaseSelfCopyRename {
		reason := "Cannot copy onto itself."
		if state.Kind == TransferKindMove {
			reason = "Cannot move onto itself."
		}
		primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, reason, styles.DialogText.Background(dbg))

		nameLabel := "New name:"
		primitive.Text(screen, rect.X+2, rect.Y+3, rect.Width-4, nameLabel, styles.DialogText.Background(dbg))

		inputY := rect.Y + 5
		inputWidth := rect.Width - 4
		drawInputField(screen, rect.X+2, inputY, inputWidth, state.SelfCopyNewName, state.FocusField == 0, styles)

		sepY := rect.Y + 6
		draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
			{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
		}, styles)
		return
	}

	destLabel := "Destination:"
	primitive.Text(screen, rect.X+2, rect.Y+1, rect.Width-4, destLabel, styles.DialogText.Background(dbg))

	inputY := rect.Y + 3
	inputWidth := rect.Width - 4
	rowFocused := state.FocusField == 0
	pickerFocused := rowFocused && state.DestSubFocus == TransferDestSubFocusPicker
	destInvalid := state.Phase == TransferPhaseDestination && state.DestPathInvalid && !state.DestPathCheckPending
	drawPathInputRow(screen, rect.X+2, inputY, inputWidth, state.Destination, rowFocused, pickerFocused, destInvalid, styles)

	if state.Kind == TransferKindCopy {
		sep1Y := rect.Y + 4
		draw.DrawDialogHSeparator(screen, rect, sep1Y, borderStyle)

		// One cell left of labels/fields so "[ ]" aligns with other dialog content (see mass rename).
		draw.DrawDialogCheckbox(screen, rect.X+1, sep1Y+1, "Preserve permissions", 'r', state.PreservePermissions, state.FocusField == 1, styles)
		draw.DrawDialogCheckbox(screen, rect.X+1, sep1Y+2, "Preserve timestamps", 't', state.PreserveTimestamps, state.FocusField == 2, styles)

		sep2Y := sep1Y + 3
		draw.DrawDialogHSeparator(screen, rect, sep2Y, borderStyle)

		tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
		buttonY := rect.Y + rect.Height - 2
		draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
			{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
			{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
		}, styles)
		return
	}

	sepY := rect.Y + 4
	draw.DrawDialogHSeparator(screen, rect, sepY, borderStyle)

	tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
		{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
	}, styles)
}

// drawMultiLocationTransferDialog renders the Copy/Move dialog when the selection spans
// multiple directories away from their common root (TransferDialogState.MultiLocation()):
// a Source: header with the common root, the usual Destination block, preserve/flatten
// checkboxes, and a scrollable "Result" preview of the selections.
func drawMultiLocationTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, styles theme.Theme, userHomeDir string, showIcons bool, iconLead int, paintIcon DeleteRowIconPainter) {
	title := "Copy"
	if state.Kind == TransferKindMove {
		title = "Move"
	}
	width := transferMultiDialogWidth(layout, state, userHomeDir, iconLead)
	vp := TransferListViewportRows(layout, state)
	height := vp + transferMultiChromeRows(state.Kind) + 2

	rect := draw.CenteredDialogRect(layout, width, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)

	contentW := draw.DialogContentWidth(rect)
	textX := draw.DialogTextX(rect)
	optX := draw.DialogOptionX(rect)

	y := rect.Y + 1
	primitive.Text(screen, textX, y, contentW, "Source:", textStyle)
	y++
	y++ // blank row between label and content
	rootLabel := primitive.FitPathForWidth(primitive.PathWithHomeTilde(state.CommonRoot, userHomeDir), contentW)
	primitive.Text(screen, textX, y, contentW, rootLabel, textStyle)
	y++
	y++ // blank row between root path and Destination:
	primitive.Text(screen, textX, y, contentW, "Destination:", textStyle)
	y++
	y++ // blank row between label and input

	rowFocused := state.FocusField == 0
	pickerFocused := rowFocused && state.DestSubFocus == TransferDestSubFocusPicker
	destInvalid := state.DestPathInvalid && !state.DestPathCheckPending
	drawPathInputRow(screen, textX, y, contentW, state.Destination, rowFocused, pickerFocused, destInvalid, styles)
	y++

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	focusIdx := 1
	if state.Kind == TransferKindCopy {
		draw.DrawDialogCheckbox(screen, optX, y, "Preserve permissions", 'r', state.PreservePermissions, state.FocusField == focusIdx, styles)
		y++
		focusIdx++
		draw.DrawDialogCheckbox(screen, optX, y, "Preserve timestamps", 't', state.PreserveTimestamps, state.FocusField == focusIdx, styles)
		y++
		focusIdx++
	}
	draw.DrawDialogCheckbox(screen, optX, y, "Flatten into destination", 'i', state.FlattenIntoDest, state.FocusField == focusIdx, styles)
	y++

	bfg, _, _ := styles.DialogFrame.Decompose()
	resultLabelStyle := styles.DialogTitle.Foreground(bfg).Background(dbg)
	draw.DrawDialogHSeparatorWithCenteredLabel(screen, rect, y, borderStyle, resultLabelStyle, " Result ")
	y++

	n := len(state.Entries)
	scroll := state.EntriesScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > n {
		scroll = n
	}
	end := scroll + vp
	if end > n {
		end = n
	}

	listStyle := styles.DialogOptionRowStyle(false, false)
	listTextX, listTextW := textX, contentW
	if showIcons && paintIcon != nil && iconLead > 0 {
		listTextX = textX + iconLead
		listTextW = contentW - iconLead
		if listTextW < 1 {
			listTextW = 1
		}
	}
	for _, e := range state.Entries[scroll:end] {
		if showIcons && paintIcon != nil && iconLead > 0 {
			paintIcon(screen, textX, y, e, styles)
		}
		label := transferEntryLabel(e, state.FlattenIntoDest)
		line := DeleteListEntryNameFitsWidth(label, e.Path, listTextW)
		primitive.Text(screen, listTextX, y, listTextW, line, listStyle)
		y++
	}
	for i := end - scroll; i < vp; i++ {
		y++ // pad unused viewport rows (fewer entries than vp) so the button row stays put
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++
	y++ // blank row above buttons

	tform := NewTransferDialogLinearForm(TransferDialogEffectiveNumContent(state))
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.FocusField == tform.OKIndex()},
		{Label: "Add paused", Shortcut: 'p', Focused: state.FocusField == tform.AddPausedIndex()},
		{Label: "Cancel", Shortcut: 'C', Focused: state.FocusField == tform.CancelIndex()},
	}, styles)
}
