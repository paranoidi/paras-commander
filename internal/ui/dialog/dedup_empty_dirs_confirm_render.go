package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// dedupEmptyDirsConfirmMaxListRows caps visible directory rows (a trailing
// "(+N more)" line takes the last slot when the list is longer).
const dedupEmptyDirsConfirmMaxListRows = 10

func dedupEmptyDirsConfirmWidth(state DedupEmptyDirsConfirmState, msg string, iconLead int) int {
	width := utf8.RuneCountInString(msg) + 4
	for _, dir := range state.Dirs {
		if w := utf8.RuneCountInString(dir) + 4 + iconLead; w > width {
			width = w
		}
	}
	return clampDedupConfirmWidth(width)
}

// clampDedupConfirmWidth keeps a dedup confirmation's width between 50 and 78 cells.
func clampDedupConfirmWidth(width int) int { return min(max(width, 50), 78) }

// dedupConfirmPathRows are the path rows of a dedup confirmation: paths indented by indent, at most
// maxRows of them, the last slot a "(+N more)" line when the list is longer. rowID (may be nil)
// names the row of path i.
func dedupConfirmPathRows(paths []string, maxRows, indent int, rowID func(i int) dlg.ID) []dlg.Item {
	shown := len(paths)
	truncated := shown > maxRows
	if truncated {
		shown = maxRows - 1
	}
	var items []dlg.Item
	for i, p := range paths[:shown] {
		it := dlg.Item{Kind: dlg.KindLabel, Label: p, Fit: dlg.FitPath, Indent: indent}
		if rowID != nil {
			it.ID = rowID(i)
		}
		items = append(items, it)
	}
	if truncated {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: fmt.Sprintf("(+%d more)", len(paths)-shown), Indent: indent})
	}
	return items
}

// dedupEmptyDirsRowID is the label ID of directory row i, so the caller can find the row's rect
// to paint its icon.
func dedupEmptyDirsRowID(i int) dlg.ID { return dlg.IndexedID("dedup-dir-", i) }

// DedupEmptyDirsConfirmView describes the confirmation: the question, the directories (capped at
// dedupEmptyDirsConfirmMaxListRows rows, a trailing "(+N more)" line replacing the last one) and
// Yes / No / Cancel. iconLead cells are kept free before each directory for a row icon the
// renderer paints (0 = no icons); key handling passes 0.
func DedupEmptyDirsConfirmView(state DedupEmptyDirsConfirmState, iconLead int) dlg.Spec {
	noun := "directories"
	if len(state.Dirs) == 1 {
		noun = "directory"
	}
	msg := fmt.Sprintf("Remove %d empty %s left by this delete?", len(state.Dirs), noun)
	items := []dlg.Item{{Kind: dlg.KindLabel, Label: msg}, {Kind: dlg.KindLabel}}
	items = append(items, dedupConfirmPathRows(state.Dirs, dedupEmptyDirsConfirmMaxListRows, iconLead, dedupEmptyDirsRowID)...)
	if len(state.Dirs) == 0 {
		items = append(items, dlg.Item{Kind: dlg.KindLabel}) // an empty list keeps one blank row
	}
	return dlg.Spec{
		Title: "Remove Empty Directories", Width: dedupEmptyDirsConfirmWidth(state, msg, iconLead), Unclamped: true, AltOnly: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: DedupEmptyDirsYes, Label: "Yes", Mnemonic: 'Y'},
			{ID: DedupEmptyDirsNo, Label: "No", Mnemonic: 'N'},
			dlg.CancelButton,
		},
	}
}

func DrawDedupEmptyDirsConfirmDialog(screen tcell.Screen, layout Layout, state DedupEmptyDirsConfirmState, ctx DialogRenderContext, paintIcon DirRowIconPainter) {
	iconLead := 0
	if ctx.ShowIcons && paintIcon != nil && ctx.IconLead > 0 {
		iconLead = ctx.IconLead
	}
	spec := DedupEmptyDirsConfirmView(state, iconLead)
	p, ok := dlg.Draw(screen, layout.Width, layout.Height, spec, state.Focus, ctx.Styles)
	if !ok || iconLead == 0 {
		return
	}
	// Icons are the overlay: each sits in the cells the row's Indent left free.
	for i, dir := range state.Dirs {
		row, found := p.Item(dedupEmptyDirsRowID(i))
		if !found || row.Rect.Y >= p.ContentBottom { // rows cut off by a short screen have no icon either
			break
		}
		paintIcon(screen, row.Rect.X-iconLead, row.Rect.Y, dir, ctx.Styles)
	}
}
