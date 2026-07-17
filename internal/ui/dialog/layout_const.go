package dialog

// PreferredFormDialogWidth is the default width for transfer (copy/move) and
// file-operation dialogs that contain text/path input rows. Input content
// scrolls horizontally when longer than the inner row width.
const PreferredFormDialogWidth = 70

// WideDialogWidthPercent is the share of the terminal width used by the
// rename/duplicate wide mode when the name no longer fits the fixed width.
const WideDialogWidthPercent = 80

// fileDialogBaseMinWidth is the starting width before content sizing;
// fileDialogFloorWidth is the absolute minimum after screen clamping.
const (
	fileDialogBaseMinWidth = 30
	fileDialogFloorWidth   = 20
)

// WideDialogWidth returns the wide-mode dialog width for a terminal width.
func WideDialogWidth(screenWidth int) int {
	return screenWidth * WideDialogWidthPercent / 100
}
