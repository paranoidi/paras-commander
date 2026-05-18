package ui

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// FindListIconLeadingWidth is the horizontal space before list text when file icons are shown.
func FindListIconLeadingWidth(showIcons bool) int {
	if !showIcons {
		return 0
	}
	return panelIconStripCells
}

func findEntryLocalfs(e FindEntry) localfs.Entry {
	t := e.Type
	if t == 0 {
		if e.IsDir {
			t = localfs.EntryDirectory
		} else {
			t = localfs.EntryFile
		}
	}
	return localfs.Entry{
		Name: filepath.Base(e.Path),
		Path: e.Path,
		Type: t,
	}
}

// PaintFindDialogRowIcon draws the same devicon strip as the panel file list (no cursor/selection tint).
func PaintFindDialogRowIcon(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme) {
	local := findEntryLocalfs(entry)
	typeStyle := panelEntryStyle(local, false, styles)
	typeFG, _, _ := typeStyle.Decompose()
	_, surfBg, _ := styles.DialogSurface.Decompose()
	iconStyle := typeStyle.Foreground(typeFG).Background(surfBg)
	paintPanelIconStrip(screen, x, y, local, iconStyle, styles, "", false, false, false)
}
