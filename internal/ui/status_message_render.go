package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// MessageUrgency selects message.info / message.warn / message.error for the transient banner
// (MessageUrgencyCritical uses message.error like MessageUrgencyError).
type MessageUrgency int

const (
	MessageUrgencyInfo MessageUrgency = iota
	MessageUrgencyWarn
	MessageUrgencyError
	// MessageUrgencyCritical uses the same palette as MessageUrgencyError (message.error).
	MessageUrgencyCritical
)

func messageUrgencyStyle(styles theme.Theme, u MessageUrgency) tcell.Style {
	switch u {
	case MessageUrgencyWarn:
		return styles.MessageWarn
	case MessageUrgencyError, MessageUrgencyCritical:
		return styles.MessageError
	default:
		return styles.MessageInfo
	}
}

// messageUrgencyListStyle applies message urgency on a panel list row. Banner message.*
// styles pair dark text on light backgrounds; on panel interior rows use the banner
// background as foreground for info/warn so text stays readable on panel.surface.
func messageUrgencyListStyle(styles theme.Theme, u MessageUrgency, bg tcell.Color) tcell.Style {
	banner := messageUrgencyStyle(styles, u)
	fg, bannerBG, attrs := banner.Decompose()
	switch u {
	case MessageUrgencyInfo, MessageUrgencyWarn:
		fg = bannerBG
	}
	st := tcell.StyleDefault.Foreground(fg).Background(bg)
	if attrs&tcell.AttrBold != 0 {
		st = st.Bold(true)
	}
	if attrs&tcell.AttrUnderline != 0 {
		st = st.Underline(true)
	}
	if attrs&tcell.AttrReverse != 0 {
		st = st.Reverse(true)
	}
	return st
}

// drawStatusMessageOverlay draws a horizontally centered status message in the given row.
// It paints only the message cells and does not erase the rest of the row (panel chrome remains visible outside the text).
func drawStatusMessageOverlay(screen tcell.Screen, rect Rect, message string, urgency MessageUrgency, styles theme.Theme) {
	if strings.TrimSpace(message) == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	msg := FormatToastDisplay(message)
	rowY := rect.Y
	maxW := rect.Width
	if maxW < 1 {
		return
	}
	st := messageUrgencyStyle(styles, urgency)
	msgRunes := []rune(msg)
	if len(msgRunes) > maxW {
		msgRunes = msgRunes[:maxW]
	}
	startCol := rect.X + (maxW-len(msgRunes))/2
	for i, r := range msgRunes {
		screen.SetContent(startCol+i, rowY, r, nil, st)
	}
}
