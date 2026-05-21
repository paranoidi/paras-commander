package theme

import (
	"github.com/gdamore/tcell/v2"
)

// DiskUsageBarStyle returns the semantic overlay row used when painting the disk-usage meter columns.
// Typically only the background color is read while the active row foreground still comes from the underlying panel row style.
// When panel chrome is blocked (modal/menu focus), the renderer skips this overlay so listing uses panel.blocked.* only.
func (t Theme) DiskUsageBarStyle(fileListActive, cursor, selected bool) tcell.Style {
	if cursor {
		if fileListActive {
			if selected {
				return t.PanelActiveUsageCursorSelected
			}
			return t.PanelUsageCursorActive
		}
		if selected {
			return t.PanelInactiveUsageCursorSelected
		}
		return t.PanelUsageCursorInactive
	}
	if selected {
		return t.PanelUsageSelected
	}
	return t.PanelUsageNormal
}
