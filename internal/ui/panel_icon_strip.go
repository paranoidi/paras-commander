package ui

import (
	devicons "github.com/epilande/go-devicons"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// panelIconStripCells is the fixed terminal cell width reserved for devicons before the name column.
const panelIconStripCells = 2

// panelIconListLeadingGutter is blank cells between the left panel border and the icon strip when icons are on.
const panelIconListLeadingGutter = 1

// PanelIconStripContext carries panel state for painting one icon-strip cell.
type PanelIconStripContext struct {
	CursorStyleKey string
	ChromeBlocked  bool
	Folder         panellist.FolderIconContext
	// PreviewLoading replaces the file icon with the prefetch-loading glyph (magenta like
	// panel.icon.folder.scanning). Ignored for directories.
	PreviewLoading bool
	// PreviewWarm tints the file icon bright magenta (warm/preloaded) vs standard magenta (not
	// yet preloaded) for image/video files — a prefetch-cache debugging aid. Ignored for
	// directories, non-image/video files, and while PreviewLoading is true.
	PreviewWarm bool
}

// panelIconStripContextFor builds the icon-strip context shared by every panel row painter
// (plain list row, tree row, carousel column row) so the preview-prefetch marks and the
// folder-icon context can't drift between them. Tree callers set Folder.TreeExpanded /
// Folder.TreeLoading on the result rather than threading them through every call site.
func panelIconStripContextFor(display PanelDisplayConfig, ctx PanelContext, state panel.State,
	entry localfs.Entry, cursorStyleKey string, diskPending, diskExcluded bool) PanelIconStripContext {
	_, previewLoading := display.PreviewPrefetchLoading[entry.Path]
	_, previewWarm := display.PreviewPrefetchWarm[entry.Path]
	return PanelIconStripContext{
		CursorStyleKey: cursorStyleKey,
		ChromeBlocked:  ctx.ChromeBlocked,
		PreviewLoading: previewLoading,
		PreviewWarm:    previewWarm,
		Folder: panellist.FolderIconContext{
			OtherPanelPath:         ctx.OtherPanelPath,
			DescendIntoMountPoints: display.DiskUsageDescendIntoMountPoints,
			ListingDev:             state.ListingDevice,
			ListingDevValid:        state.ListingDeviceValid,
			DiskPending:            diskPending,
			DiskExcluded:           diskExcluded,
			DiskUsageChrome:        display.ShowDiskUsage,
		},
	}
}

// fileDeviconForeground picks the file-icon color: cursor override, else devicon hex, else row FG.
// When chromeBlocked is true, devicon hex is skipped so icons match panel.blocked.row.* foreground.
func fileDeviconForeground(rowStyle tcell.Style, deviconHex string, th theme.Theme, cursorStyleKey string, chromeBlocked bool) tcell.Color {
	rowFG, _, _ := rowStyle.Decompose()
	if cursorStyleKey != "" && th.PanelFileIconFG != nil {
		if c, ok := th.PanelFileIconFG[cursorStyleKey]; ok {
			return c
		}
	}
	if !chromeBlocked && deviconHex != "" {
		if c, ok := deviconHexForeground(deviconHex); ok {
			return c
		}
	}
	return rowFG
}

// previewIconForeground picks the warm/cold prefetch-debug tint color: cursor override, else the
// theme's panel.icon.preview.{warm,cold} color.
func previewIconForeground(warm bool, th theme.Theme, cursorStyleKey string) tcell.Color {
	if warm {
		return th.PanelRowIconForeground(cursorStyleKey, th.PanelIconPreviewWarm)
	}
	return th.PanelRowIconForeground(cursorStyleKey, th.PanelIconPreviewCold)
}

func paintPanelIconStrip(
	screen tcell.Screen,
	x, y int,
	entry localfs.Entry,
	rowStyle tcell.Style,
	th theme.Theme,
	ctx PanelIconStripContext,
) {
	var icon string
	var fg tcell.Color
	if entry.Type == localfs.EntryDirectory {
		kind, ok := panellist.ResolveFolderIconKind(entry, ctx.Folder)
		if !ok {
			icon = " "
			fg, _, _ = rowStyle.Decompose()
		} else {
			icon = th.FolderIconGlyph(kind)
			if ctx.ChromeBlocked {
				fg = fileDeviconForeground(rowStyle, "", th, ctx.CursorStyleKey, true)
			} else {
				fg = th.FolderIconForeground(kind, ctx.CursorStyleKey, rowStyle)
			}
		}
	} else if ctx.PreviewLoading {
		icon = string(th.SymbolFilelistPreviewLoading())
		if icon == "" {
			icon = " "
		}
		if ctx.ChromeBlocked {
			fg = fileDeviconForeground(rowStyle, "", th, ctx.CursorStyleKey, true)
		} else {
			// Same magenta as disk-usage folder scanning (panel.icon.folder.scanning).
			fg = th.FolderIconForeground(theme.FolderIconScanning, ctx.CursorStyleKey, rowStyle)
		}
	} else {
		st := devicons.IconForInfo(fileInfoFromEntry(entry))
		icon = st.Icon
		if icon == "" {
			icon = " "
		}
		switch {
		case ctx.ChromeBlocked:
			fg = fileDeviconForeground(rowStyle, "", th, ctx.CursorStyleKey, true)
		case localfs.IsImagePath(entry.Path) || localfs.IsVideoPath(entry.Path):
			fg = previewIconForeground(ctx.PreviewWarm, th, ctx.CursorStyleKey)
		default:
			fg = fileDeviconForeground(rowStyle, st.Color, th, ctx.CursorStyleKey, ctx.ChromeBlocked)
		}
	}
	iconStyle := rowStyle.Foreground(fg)

	col := 0
	for _, r := range icon {
		w := runewidth.RuneWidth(r)
		if w < 1 {
			w = 1
		}
		if col+w > panelIconStripCells {
			break
		}
		screen.SetContent(x+col, y, r, nil, iconStyle)
		col += w
	}
	for col < panelIconStripCells {
		screen.SetContent(x+col, y, ' ', nil, rowStyle)
		col++
	}
}

func paintPanelIconStripBlank(screen tcell.Screen, x, y int, rowStyle tcell.Style) {
	for col := 0; col < panelIconStripCells; col++ {
		screen.SetContent(x+col, y, ' ', nil, rowStyle)
	}
}
