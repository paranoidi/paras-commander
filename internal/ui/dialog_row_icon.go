package ui

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

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
		Name: filepath.Base(filepath.FromSlash(e.RelLine)),
		Path: e.RelLine,
		Type: t,
	}
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

// DialogListIconLeadingWidth is the horizontal space before list text when file icons are shown.
func DialogListIconLeadingWidth(showIcons bool) int {
	if !showIcons {
		return 0
	}
	return panelIconStripCells
}

// FindListIconLeadingWidth is an alias for DialogListIconLeadingWidth (find dialog).
func FindListIconLeadingWidth(showIcons bool) int {
	return DialogListIconLeadingWidth(showIcons)
}

// DeleteListIconLeadingWidth is an alias for DialogListIconLeadingWidth (delete dialog).
func DeleteListIconLeadingWidth(showIcons bool) int {
	return DialogListIconLeadingWidth(showIcons)
}

// PaintDialogRowIcon draws the same devicon strip as the panel file list (no cursor/selection tint).
func PaintDialogRowIcon(screen tcell.Screen, x, y int, entry localfs.Entry, styles theme.Theme) {
	typeStyle := styles.PanelListingEntryStyle(entry.Type, false)
	typeFG, _, _ := typeStyle.Decompose()
	_, surfBg, _ := styles.DialogSurface.Decompose()
	iconStyle := typeStyle.Foreground(typeFG).Background(surfBg)
	paintPanelIconStrip(screen, x, y, entry, iconStyle, styles, "", false, false, false)
}

// PaintFindDialogRowIcon draws file-list devicons for one find dialog row.
func PaintFindDialogRowIcon(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme) {
	PaintDialogRowIcon(screen, x, y, findEntryLocalfs(entry), styles)
}

// PaintDeleteDialogRowIcon draws file-list devicons for one delete dialog row.
func PaintDeleteDialogRowIcon(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme) {
	PaintDialogRowIcon(screen, x, y, deleteListEntryLocalfs(entry), styles)
}
