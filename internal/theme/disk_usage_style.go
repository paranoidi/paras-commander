package theme

import (
	"github.com/gdamore/tcell/v2"
)

// diskUsageBar is the one place that picks the disk-usage bar variant of a row: its style and
// its theme key.
func (t Theme) diskUsageBar(fileListActive, cursor, selected bool) (tcell.Style, string) {
	if cursor {
		if fileListActive {
			if selected {
				return t.PanelActiveUsageCursorSelected, "panel.active.usage.cursor.selected"
			}
			return t.PanelUsageCursorActive, "panel.active.usage.cursor"
		}
		if selected {
			return t.PanelInactiveUsageCursorSelected, "panel.inactive.usage.cursor.selected"
		}
		return t.PanelUsageCursorInactive, "panel.inactive.usage.cursor"
	}
	if selected {
		return t.PanelUsageSelected, "panel.usage.selected"
	}
	return t.PanelUsageNormal, "panel.usage.normal"
}

// DiskUsageBarStyle returns the semantic overlay row used when painting the disk-usage meter columns.
// Typically only the background color is read while the active row foreground still comes from the underlying panel row style.
// When panel chrome is blocked (modal/menu focus), the renderer skips this overlay so listing uses panel.blocked.* only.
func (t Theme) DiskUsageBarStyle(fileListActive, cursor, selected bool) tcell.Style {
	style, _ := t.diskUsageBar(fileListActive, cursor, selected)
	return style
}

// DiskUsageBarIconKey returns the PanelFileIconFG key for icons on cells with the bar background,
// or "" when that bar style sets no icon color. A non-empty key takes precedence over the cursor
// row's key on those cells only.
func (t Theme) DiskUsageBarIconKey(fileListActive, cursor, selected bool) string {
	_, key := t.diskUsageBar(fileListActive, cursor, selected)
	if _, ok := t.PanelFileIconFG[key]; !ok {
		return ""
	}
	return key
}
