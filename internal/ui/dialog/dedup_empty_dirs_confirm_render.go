package dialog

import (
	"fmt"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// dedupEmptyDirsConfirmMaxListRows caps visible directory rows (a trailing
// "(+N more)" line takes the last slot when the list is longer).
const dedupEmptyDirsConfirmMaxListRows = 10

// DedupEmptyDirsRowIconPainter draws a folder devicon for one row of the
// empty-dirs confirmation list; nil skips icons. dir is the relative path
// shown on that row (used only to derive the display name for lookup).
type DedupEmptyDirsRowIconPainter func(screen tcell.Screen, x, y int, dir string, styles theme.Theme)

func dedupEmptyDirsConfirmWidth(state DedupEmptyDirsConfirmState, msg string, iconLead int) int {
	width := utf8.RuneCountInString(msg) + 4
	for _, dir := range state.Dirs {
		if w := utf8.RuneCountInString(dir) + 4 + iconLead; w > width {
			width = w
		}
	}
	if width < 50 {
		width = 50
	}
	if width > 78 {
		width = 78
	}
	return width
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
	shown := len(state.Dirs)
	truncated := shown > dedupEmptyDirsConfirmMaxListRows
	if truncated {
		shown = dedupEmptyDirsConfirmMaxListRows - 1
	}
	for i := 0; i < shown; i++ {
		items = append(items, dlg.Item{ID: dedupEmptyDirsRowID(i), Kind: dlg.KindLabel, Label: state.Dirs[i], Fit: dlg.FitPath, Indent: iconLead})
	}
	if truncated {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: fmt.Sprintf("(+%d more)", len(state.Dirs)-shown), Indent: iconLead})
	}
	if len(state.Dirs) == 0 {
		items = append(items, dlg.Item{Kind: dlg.KindLabel}) // an empty list keeps one blank row
	}
	return dlg.Spec{
		Title: "Remove Empty Directories", Width: dedupEmptyDirsConfirmWidth(state, msg, iconLead), Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: DedupEmptyDirsYes, Label: "Yes", Mnemonic: 'Y'},
			{ID: DedupEmptyDirsNo, Label: "No", Mnemonic: 'N'},
			dlg.CancelButton,
		},
	}
}

func DrawDedupEmptyDirsConfirmDialog(screen tcell.Screen, layout Layout, state DedupEmptyDirsConfirmState, ctx DialogRenderContext, paintIcon DedupEmptyDirsRowIconPainter) {
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
