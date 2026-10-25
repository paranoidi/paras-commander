package dialog

import (
	"path/filepath"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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
// Source label, root path, blank, Destination label, input row, separator,
// preserve-permissions/timestamps/dereference-symlinks checkboxes (copy only) + flatten
// checkbox, "Result" separator, button separator, button row, plus one spare row so the
// preview height stays within the dialog's share of the screen.
func transferMultiChromeRows(kind TransferKind) int {
	checkboxRows := 1 // Flatten into destination
	if kind == TransferKindCopy {
		checkboxRows += 3 // Preserve permissions + Preserve timestamps + Dereference symlinks
	}
	const fixedRows = 10 // 6 rows through the destination separator + Result separator(1) + button separator/spare/buttons(3)
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

// transferDestinationFrameWidth is the outer dialog width DrawTransferDialog paints:
// PreferredFormDialogWidth, or the expanded multi-location width when MultiLocation().
func transferDestinationFrameWidth(layout Layout, state TransferDialogState, userHomeDir string, iconLead int) int {
	width := PreferredFormDialogWidth
	if state.MultiLocation() {
		width = transferMultiDialogWidth(layout, state, userHomeDir, iconLead)
	}
	if width > layout.Width {
		width = layout.Width
	}
	return width
}

// TransferDestinationTextWidth is the painted width of the phase's text input (the destination,
// or the new name of the self-copy phase). Completion and scroll use this so FileDialogField.Scroll
// matches the row that is actually drawn.
func TransferDestinationTextWidth(layout Layout, state TransferDialogState, userHomeDir string, iconLead int) int {
	spec := TransferView(state, TransferViewContext{Layout: layout, UserHomeDir: userHomeDir, IconLead: iconLead})
	if p, ok := dlg.Place(layout.Width, layout.Height, spec); ok {
		id := TransferDest
		if state.Phase == TransferPhaseSelfCopyRename {
			id = TransferNewName
		}
		if in, found := p.Item(id); found && in.Rect.Width > 0 {
			return in.Rect.Width
		}
	}
	return max(transferDestinationFrameWidth(layout, state, userHomeDir, iconLead)-4, 1)
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

// DrawTransferDialog paints the copy / move modal from its widget spec (see TransferView), then the
// multi-location Result rows (with their icons) and the destination's completion dropdown, both
// anchored to rects of the placement.
func DrawTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, ctx DialogRenderContext, paintIcon DeleteRowIconPainter) {
	spec := TransferView(state, TransferViewContext{Layout: layout, UserHomeDir: ctx.UserHomeDir, IconLead: ctx.IconLead})
	dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, ctx.Styles)
	p, ok := dlg.Place(layout.Width, layout.Height, spec)
	if !ok {
		return
	}
	if state.MultiLocation() {
		drawTransferResultRows(screen, p, state, ctx, paintIcon)
	}
	if state.Phase == TransferPhaseDestination {
		if in, ok := p.Item(TransferDest); ok {
			drawPathCompletionDropdown(screen, in.Rect.X, in.Rect.Y+1, state.Destination.Scroll, state.Destination.Completion, ctx.ScrollbarStyle, ctx.Styles)
		}
	}
}

// drawTransferResultRows paints the visible slice of the Result preview into the reserved region:
// a devicon (when icons are shown) and the root-relative or basename label of each entry.
func drawTransferResultRows(screen tcell.Screen, p dlg.Placement, state TransferDialogState, ctx DialogRenderContext, paintIcon DeleteRowIconPainter) {
	region, ok := p.Item(transferResult)
	if !ok {
		return
	}
	styles := ctx.Styles
	listStyle := styles.DialogOptionRowStyle(false, false)
	textX, textW := region.Rect.X+1, region.Rect.Width-2
	withIcons := ctx.ShowIcons && paintIcon != nil && ctx.IconLead > 0
	if withIcons {
		textX += ctx.IconLead
		textW = max(textW-ctx.IconLead, 1)
	}
	n := len(state.Entries)
	scroll := min(max(state.EntriesScroll, 0), n)
	for i := 0; i < region.Rect.Height && scroll+i < n; i++ {
		y := region.Rect.Y + i
		if y >= p.ContentBottom {
			break
		}
		e := state.Entries[scroll+i]
		if withIcons {
			paintIcon(screen, region.Rect.X+1, y, e, styles)
		}
		line := DeleteListEntryNameFitsWidth(transferEntryLabel(e, state.FlattenIntoDest), e.Path, textW)
		primitive.Text(screen, textX, y, textW, line, listStyle)
	}
}
