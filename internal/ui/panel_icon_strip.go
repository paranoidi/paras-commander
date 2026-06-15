package ui

import (
	devicons "github.com/epilande/go-devicons"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// panelIconStripCells is the fixed terminal cell width reserved for devicons before the name column.
const panelIconStripCells = 2

// panelIconListLeadingGutter is blank cells between the left panel border and the icon strip when icons are on.
const panelIconListLeadingGutter = 1

// diskUsageExcludedFolderGlyph marks directories skipped by disk-usage traversal (always this glyph).
// panel.folder.diskscan_excluded grey applies only when disk-usage metering is active for the panel.
const diskUsageExcludedFolderGlyph = "\uf114"

// panelDeviconForeground picks the file-icon color: theme cursor override, else devicon hex, else row FG.
// diskExcludedGrey applies panel.folder.diskscan_excluded only when disk-usage metering is active for this panel.
func panelDeviconForeground(rowStyle tcell.Style, deviconHex string, th theme.Theme, cursorStyleKey string, diskPending, diskExcludedGrey, openInOther, onOtherMount bool) tcell.Color {
	rowFG, _, _ := rowStyle.Decompose()
	if cursorStyleKey != "" && th.PanelFileIconFG != nil {
		if c, ok := th.PanelFileIconFG[cursorStyleKey]; ok {
			return c
		}
	}
	if diskPending {
		dfg, _, _ := th.PanelFolderDiskscan.Decompose()
		if dfg != tcell.ColorDefault {
			return dfg
		}
	}
	if openInOther {
		return th.PanelRowSuffixIconForeground(cursorStyleKey, th.PanelRowFolderOpen)
	}
	if onOtherMount {
		return th.PanelRowSuffixIconForeground(cursorStyleKey, th.PanelRowFolderMount)
	}
	if diskExcludedGrey {
		efg, _, _ := th.PanelFolderDiskscanExcluded.Decompose()
		if efg != tcell.ColorDefault {
			return efg
		}
	}
	if deviconHex != "" {
		if c, ok := deviconHexForeground(deviconHex); ok {
			return c
		}
	}
	return rowFG
}

func directoryIconGlyph(
	entry localfs.Entry,
	th theme.Theme,
	otherPanelPath string,
	descendIntoMountPoints bool,
	listingDev uint64,
	listingDevValid bool,
	diskPending, diskExcluded, diskUsageChrome bool,
) string {
	if diskExcluded && diskUsageChrome {
		return diskUsageExcludedFolderGlyph
	}
	if diskPending {
		return th.SymbolFoldersScanning()
	}
	if panellist.EntryOpenInOtherPanel(entry, otherPanelPath) {
		return th.SymbolFoldersOpen()
	}
	if panellist.EntryOnOtherMount(entry, descendIntoMountPoints, listingDev, listingDevValid) {
		return th.SymbolFoldersMount()
	}
	return th.SymbolFoldersFolder()
}

func paintPanelIconStrip(
	screen tcell.Screen,
	x, y int,
	entry localfs.Entry,
	rowStyle tcell.Style,
	th theme.Theme,
	cursorStyleKey string,
	diskPending, diskExcluded bool,
	diskUsageChrome bool,
	otherPanelPath string,
	descendIntoMountPoints bool,
	listingDev uint64,
	listingDevValid bool,
) {
	var icon string
	var deviconHex string
	openInOther := false
	onOtherMount := false
	if entry.Type == localfs.EntryDirectory {
		icon = directoryIconGlyph(entry, th, otherPanelPath, descendIntoMountPoints, listingDev, listingDevValid, diskPending, diskExcluded, diskUsageChrome)
		openInOther = panellist.EntryOpenInOtherPanel(entry, otherPanelPath) && !diskPending
		onOtherMount = panellist.EntryOnOtherMount(entry, descendIntoMountPoints, listingDev, listingDevValid) && !diskPending && !openInOther
	} else {
		st := devicons.IconForInfo(fileInfoFromEntry(entry))
		icon = st.Icon
		deviconHex = st.Color
	}
	if icon == "" {
		icon = " "
	}
	excludedGrey := diskExcluded && diskUsageChrome
	fg := panelDeviconForeground(rowStyle, deviconHex, th, cursorStyleKey, diskPending, excludedGrey, openInOther, onOtherMount)
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
