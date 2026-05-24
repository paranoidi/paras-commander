package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// DeleteListIconLeadingWidth is the horizontal space before list text when file icons are shown.
func DeleteListIconLeadingWidth(showIcons bool) int {
	if !showIcons {
		return 0
	}
	return panelIconStripCells
}

func deleteListEntryLocalfs(e DeleteListEntry) localfs.Entry {
	t := e.Type
	if t == 0 {
		t = localfs.EntryFile
	}
	return localfs.Entry{
		Name: e.Name,
		Path: e.Path,
		Type: t,
	}
}

// PaintDeleteDialogRowIcon draws the same devicon strip as the panel file list (no cursor/selection tint).
func PaintDeleteDialogRowIcon(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme) {
	local := deleteListEntryLocalfs(entry)
	typeStyle := panelEntryStyle(local, false, styles)
	typeFG, _, _ := typeStyle.Decompose()
	_, surfBg, _ := styles.DialogSurface.Decompose()
	iconStyle := typeStyle.Foreground(typeFG).Background(surfBg)
	paintPanelIconStrip(screen, x, y, local, iconStyle, styles, "", false, false, false)
}
