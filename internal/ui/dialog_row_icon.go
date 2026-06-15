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

// PaintDialogRowIcon draws the same devicon strip as the panel file list (no cursor/selection tint).
// Find and delete dialog lists pass a zero PanelIconStripContext so directories always use the
// default closed-folder glyph and color—no other-panel, mount, or disk-usage icon semantics.
func PaintDialogRowIcon(screen tcell.Screen, x, y int, entry localfs.Entry, styles theme.Theme) {
	typeStyle := styles.PanelListingEntryStyle(entry.Type, false)
	typeFG, _, _ := typeStyle.Decompose()
	_, surfBg, _ := styles.DialogSurface.Decompose()
	iconStyle := typeStyle.Foreground(typeFG).Background(surfBg)
	paintPanelIconStrip(screen, x, y, entry, iconStyle, styles, PanelIconStripContext{})
}

// PaintFindDialogRowIcon draws file-list devicons for one find dialog row.
func PaintFindDialogRowIcon(screen tcell.Screen, x, y int, entry FindEntry, styles theme.Theme) {
	PaintDialogRowIcon(screen, x, y, findEntryLocalfs(entry), styles)
}

// PaintDeleteDialogRowIcon draws file-list devicons for one delete dialog row.
func PaintDeleteDialogRowIcon(screen tcell.Screen, x, y int, entry DeleteListEntry, styles theme.Theme) {
	PaintDialogRowIcon(screen, x, y, deleteListEntryLocalfs(entry), styles)
}
