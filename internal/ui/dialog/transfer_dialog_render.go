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
// Result rows' devicons and the destination's completion dropdown, anchored to the placement.
func DrawTransferDialog(screen tcell.Screen, layout Layout, state TransferDialogState, ctx DialogRenderContext, paintIcon DeleteRowIconPainter) {
	spec := TransferView(state, TransferViewContext{Layout: layout, UserHomeDir: ctx.UserHomeDir, IconLead: ctx.IconLead, ScrollbarStyle: ctx.ScrollbarStyle})
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, ctx.Styles)
	if !ok {
		return
	}
	if state.MultiLocation() && ctx.ShowIcons && paintIcon != nil && ctx.IconLead > 0 {
		paintListIcons(p, TransferResult, func(row, x, y int) { paintIcon(screen, x, y, state.Entries[row], ctx.Styles) })
	}
	if state.Phase == TransferPhaseDestination {
		if in, ok := p.Item(TransferDest); ok {
			drawPathCompletionDropdown(screen, in.Rect.X, in.Rect.Y+1, state.Destination.Scroll, state.Destination.Completion, ctx.ScrollbarStyle, ctx.Styles)
		}
	}
}
