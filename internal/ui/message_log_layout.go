package ui

// MessageLogWrapColsForLayout returns how many runes fit on one Messages log text line for layout.
// It mirrors drawMessagesView content width (panel union minus margins and the time column).
func MessageLogWrapColsForLayout(layout Layout) int {
	if layout.TooSmall {
		return MessageLogWrapRunes
	}
	rect := MergeTwinPanelRects(layout.Left, layout.Right)
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}
	msgW := contentW - messagesListColTime
	if msgW < 1 {
		return 1
	}
	return msgW
}
