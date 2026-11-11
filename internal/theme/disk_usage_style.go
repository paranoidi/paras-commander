package theme

import (
	"github.com/gdamore/tcell/v2"
)

// Disk-usage bar style keys: the styles that may carry `icon` group colors (see BarIconGroup).
const (
	barKeyNormal                 = "panel.usage.normal"
	barKeySelected               = "panel.usage.selected"
	barKeyActiveCursor           = "panel.active.usage.cursor"
	barKeyActiveCursorSelected   = "panel.active.usage.cursor.selected"
	barKeyInactiveCursor         = "panel.inactive.usage.cursor"
	barKeyInactiveCursorSelected = "panel.inactive.usage.cursor.selected"
)

// diskUsageBarKeys lists every disk-usage bar style key.
var diskUsageBarKeys = map[string]struct{}{
	barKeyNormal: {}, barKeySelected: {}, barKeyActiveCursor: {}, barKeyActiveCursorSelected: {},
	barKeyInactiveCursor: {}, barKeyInactiveCursorSelected: {},
}

// BarIconGroup names a group of row icons a disk-usage bar style recolors on the cells where the
// bar is behind them (`icon.<group> = "<color>"` in the theme file). A group the style does not list
// keeps the icon's normal color on the bar.
type BarIconGroup string

const (
	BarIconFolder         BarIconGroup = "folder"          // FolderIconDefault
	BarIconFolderExpanded BarIconGroup = "folder_expanded" // FolderIconTreeExpanded
	BarIconFolderScanning BarIconGroup = "folder_scanning" // FolderIconScanning
	BarIconFolderOpen     BarIconGroup = "folder_open"     // FolderIconOpen
	BarIconFolderMount    BarIconGroup = "folder_mount"    // FolderIconMount
	BarIconFolderExcluded BarIconGroup = "folder_excluded" // FolderIconExcluded
	BarIconFile           BarIconGroup = "file"            // every file icon in the icon strip
	BarIconPreviewLoading BarIconGroup = "preview_loading" // the preview-loading icon in the icon strip
	BarIconFileIndex      BarIconGroup = "fileindex"       // fileindex state cell
	BarIconGit            BarIconGroup = "git"             // git status cells
	BarIconSuffix         BarIconGroup = "suffix"          // suffix marks in the name column
)

var barIconGroups = map[BarIconGroup]struct{}{
	BarIconFolder: {}, BarIconFolderExpanded: {}, BarIconFolderScanning: {}, BarIconFolderOpen: {},
	BarIconFolderMount: {}, BarIconFolderExcluded: {}, BarIconFile: {}, BarIconPreviewLoading: {},
	BarIconFileIndex: {}, BarIconGit: {}, BarIconSuffix: {},
}

// BarIconGroupForFolder is the bar icon group of a directory icon kind.
func BarIconGroupForFolder(kind FolderIconKind) BarIconGroup {
	switch kind {
	case FolderIconTreeExpanded:
		return BarIconFolderExpanded
	case FolderIconScanning:
		return BarIconFolderScanning
	case FolderIconOpen:
		return BarIconFolderOpen
	case FolderIconMount:
		return BarIconFolderMount
	case FolderIconExcluded:
		return BarIconFolderExcluded
	default:
		return BarIconFolder
	}
}

// diskUsageBar is the one place that picks the disk-usage bar variant of a row: its style and
// its theme key.
func (t Theme) diskUsageBar(fileListActive, cursor, selected bool) (tcell.Style, string) {
	if cursor {
		if fileListActive {
			if selected {
				return t.PanelActiveUsageCursorSelected, barKeyActiveCursorSelected
			}
			return t.PanelUsageCursorActive, barKeyActiveCursor
		}
		if selected {
			return t.PanelInactiveUsageCursorSelected, barKeyInactiveCursorSelected
		}
		return t.PanelUsageCursorInactive, barKeyInactiveCursor
	}
	if selected {
		return t.PanelUsageSelected, barKeySelected
	}
	return t.PanelUsageNormal, barKeyNormal
}

// DiskUsageBarStyle returns the semantic overlay row used when painting the disk-usage meter columns.
// Typically only the background color is read while the active row foreground still comes from the underlying panel row style.
// When panel chrome is blocked (modal/menu focus), the renderer skips this overlay so listing uses panel.blocked.* only.
func (t Theme) DiskUsageBarStyle(fileListActive, cursor, selected bool) tcell.Style {
	style, _ := t.diskUsageBar(fileListActive, cursor, selected)
	return style
}

// DiskUsageBarIconKey returns the bar style key for icons on cells with the bar background, or ""
// when that bar style lists no icon group. Each paint site looks up its own group with
// DiskUsageBarIconColor; the key says only that some group may apply.
func (t Theme) DiskUsageBarIconKey(fileListActive, cursor, selected bool) string {
	_, key := t.diskUsageBar(fileListActive, cursor, selected)
	if len(t.DiskUsageBarIconFG[key]) == 0 {
		return ""
	}
	return key
}

// DiskUsageBarIconColor returns the color the bar style barKey (from DiskUsageBarIconKey) gives the
// icon group; ok is false when the style does not list the group (or barKey is ""), and the icon
// then keeps the color it shows off the bar.
func (t Theme) DiskUsageBarIconColor(barKey string, group BarIconGroup) (tcell.Color, bool) {
	c, ok := t.DiskUsageBarIconFG[barKey][group]
	return c, ok
}
