package dialog

const (
	findDialogMinWidth    = 54 // narrower terminals draw no find dialog
	findDialogMinListRows = 4
	findDialogMaxListRows = 26
)

// findDialogBaseHeight is the frame height without the result list: borders, the query row, the
// option rows and the three rules plus the button row.
func findDialogBaseHeight(optionRows int) int { return 7 + optionRows }

// FindDialogMetrics returns dialog width/height and visible list row count for the find overlay.
// listH is the result list's height in FindDialogView and is used for list scroll math.
func FindDialogMetrics(layout Layout, showSearchSelectionsOption bool) (width, height, listH int, ok bool) {
	width = findDialogPreferredWidth
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < findDialogMinWidth {
		return 0, 0, 0, false
	}

	checkboxRows := 1
	if showSearchSelectionsOption {
		checkboxRows = 2
	}
	baseHeight := findDialogBaseHeight(checkboxRows)

	listH = layout.Height - 14
	switch {
	case listH > findDialogMaxListRows:
		listH = findDialogMaxListRows
	case listH < findDialogMinListRows:
		listH = findDialogMinListRows
	}
	height = baseHeight + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - baseHeight
		if listH < findDialogMinListRows {
			return 0, 0, 0, false
		}
	}
	return width, height, listH, true
}
