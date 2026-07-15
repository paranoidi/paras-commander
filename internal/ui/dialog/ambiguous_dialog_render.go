package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// AmbiguousListMaxRows caps visible list rows per paint (full list still scrolls).
const AmbiguousListMaxRows = 10

// ambiguousChromeRows is interior rows other than the scrollable list: header, blank,
// root path, separator, separator, blank, button row (top+bottom border added separately).
const ambiguousChromeRows = 7

func ambiguousDialogMaxHeight(layoutHeight int) int {
	maxH := layoutHeight * 80 / 100
	if maxH > layoutHeight-2 {
		maxH = layoutHeight - 2
	}
	if maxH < 8 {
		maxH = 8
	}
	return maxH
}

// AmbiguousListViewportRows returns how many preview-list rows fit for the given layout
// and entry count. Exported so the key handler clamps Scroll with the same number.
func AmbiguousListViewportRows(layout Layout, n int) int {
	capped := n
	if capped > AmbiguousListMaxRows {
		capped = AmbiguousListMaxRows
	}
	if capped < 1 {
		capped = 1
	}
	maxH := ambiguousDialogMaxHeight(layout.Height)
	available := maxH - ambiguousChromeRows - 2 // minus top/bottom border
	if available < 1 {
		available = 1
	}
	if capped <= available {
		return capped
	}
	return available
}

func ambiguousEntryLabel(e DeleteListEntry) string {
	if e.Type == localfs.EntryDirectory && e.Name != "" {
		return e.Name + "/"
	}
	return e.Name
}

func ambiguousTransferDialogTitle(kind TransferKind) string {
	if kind == TransferKindMove {
		return "Confirm ambiguous move?"
	}
	return "Confirm ambiguous copy?"
}

func ambiguousDialogWidth(layout Layout, state AmbiguousTransferState, userHomeDir string, iconLead int) int {
	width := PreferredFormDialogWidth
	verb := "Copy"
	if state.Kind == TransferKindMove {
		verb = "Move"
	}
	n := len(state.Entries)
	itemWord := "items"
	if n == 1 {
		itemWord = "item"
	}
	header := fmt.Sprintf("%s %d selected %s from:", verb, n, itemWord)
	if w := utf8.RuneCountInString(header) + 4; w > width {
		width = w
	}
	rootDisp := primitive.PathWithHomeTilde(state.CommonRoot, userHomeDir)
	if w := utf8.RuneCountInString(rootDisp) + 4; w > width {
		width = w
	}
	title := ambiguousTransferDialogTitle(state.Kind)
	if w := utf8.RuneCountInString(title) + 4; w > width {
		width = w
	}
	for _, e := range state.Entries {
		w := utf8.RuneCountInString(ambiguousEntryLabel(e)) + 4 + iconLead
		if w > width {
			width = w
		}
	}
	if width > layout.Width {
		width = layout.Width
	}
	return width
}

// DrawAmbiguousTransferDialog draws the copy/move confirm shown when a selection spans
// multiple directories away from their common root: full root path plus a scrollable,
// non-recursive preview of the selections in root-relative shortest form.
func DrawAmbiguousTransferDialog(screen tcell.Screen, layout Layout, state AmbiguousTransferState, styles theme.Theme, userHomeDir string, showIcons bool, iconLead int, paintIcon DeleteRowIconPainter) {
	n := len(state.Entries)
	vp := AmbiguousListViewportRows(layout, n)
	width := ambiguousDialogWidth(layout, state, userHomeDir, iconLead)
	height := vp + ambiguousChromeRows + 2
	rect := draw.CenteredDialogRect(layout, width, height)

	title := ambiguousTransferDialogTitle(state.Kind)
	verb := "Copy"
	if state.Kind == TransferKindMove {
		verb = "Move"
	}
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	textStyle := styles.DialogText.Background(dbg)
	listStyle := styles.DialogOptionRowStyle(false, false)
	contentW := draw.DialogContentWidth(rect)
	listCol := draw.DialogTextX(rect)
	textX, textW := listCol, contentW
	if showIcons && paintIcon != nil && iconLead > 0 {
		textX = listCol + iconLead
		textW = contentW - iconLead
		if textW < 1 {
			textW = 1
		}
	}

	itemWord := "items"
	if n == 1 {
		itemWord = "item"
	}
	header := fmt.Sprintf("%s %d selected %s from:", verb, n, itemWord)

	y := rect.Y + 1
	primitive.Text(screen, listCol, y, contentW, header, textStyle)
	y++ // blank row
	y++
	rootLabel := primitive.FitPathForWidth(primitive.PathWithHomeTilde(state.CommonRoot, userHomeDir), contentW)
	primitive.Text(screen, listCol, y, contentW, rootLabel, textStyle)
	y++
	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++

	scroll := state.Scroll
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
	for _, e := range state.Entries[scroll:end] {
		if showIcons && paintIcon != nil && iconLead > 0 {
			paintIcon(screen, listCol, y, e, styles)
		}
		label := ambiguousEntryLabel(e)
		line := DeleteListEntryNameFitsWidth(label, e.Path, textW)
		primitive.Text(screen, textX, y, textW, line, listStyle)
		y++
	}

	draw.DrawDialogHSeparator(screen, rect, y, borderStyle)
	y++ // blank row
	y++
	draw.DrawDialogButtonRowCentered(screen, rect, y, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: state.Focus == 0},
		{Label: "Cancel", Shortcut: 'C', Focused: state.Focus == 1},
	}, styles)
}
