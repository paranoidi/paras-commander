package dialog

// FindDialogMetrics returns dialog width/height and visible list row count for the find overlay.
// listH matches the row loop in DrawFindDialog and is used for list scroll math.
func FindDialogMetrics(layout Layout, showSearchSelectionsOption bool) (width, height, listH int, ok bool) {
	width = findDialogPreferredWidth
	if width > layout.Width-4 {
		width = layout.Width - 4
	}
	if width < 54 {
		return 0, 0, 0, false
	}

	checkboxRows := 1
	if showSearchSelectionsOption {
		checkboxRows = 2
	}
	baseHeight := 7 + checkboxRows

	listH = layout.Height - 14
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	height = baseHeight + listH
	if height > layout.Height-2 {
		height = layout.Height - 2
		listH = height - baseHeight
		if listH < 4 {
			return 0, 0, 0, false
		}
	}
	return width, height, listH, true
}
