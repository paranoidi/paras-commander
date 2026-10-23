package ui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// MessageUrgency selects message.info / message.warn / message.error / message.success for the
// transient banner (MessageUrgencyCritical uses message.error like MessageUrgencyError).
type MessageUrgency int

const (
	MessageUrgencyInfo MessageUrgency = iota
	MessageUrgencyWarn
	MessageUrgencyError
	// MessageUrgencyCritical uses the same palette as MessageUrgencyError (message.error).
	MessageUrgencyCritical
	// MessageUrgencySuccess uses message.success and is prefixed with Theme.IconMessageSuccess.
	MessageUrgencySuccess
)

// messageWithUrgencyIcon prefixes the success icon (when icons are on) to a success message.
func messageWithUrgencyIcon(styles theme.Theme, u MessageUrgency, msg string) string {
	if icon := styles.IconMessageSuccess(); u == MessageUrgencySuccess && icon != "" {
		return icon + " " + msg
	}
	return msg
}

func messageUrgencyStyle(styles theme.Theme, u MessageUrgency) tcell.Style {
	switch u {
	case MessageUrgencyWarn:
		return styles.MessageWarn
	case MessageUrgencyError, MessageUrgencyCritical:
		return styles.MessageError
	case MessageUrgencySuccess:
		return styles.MessageSuccess
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
	case MessageUrgencyInfo, MessageUrgencyWarn, MessageUrgencySuccess:
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

// PaintTransientStatusMessage redraws the status row above the footer when a banner is active.
// It owns the row choice: normally the row immediately above the footer, relocated onto the
// terminal panel's separator row when that panel is visible (message wins over the panel label).
func PaintTransientStatusMessage(screen tcell.Screen, layout Layout, model Model, styles theme.Theme) {
	msg := strings.TrimSpace(model.Message)
	if msg == "" || layout.Footer.Height <= 0 {
		return
	}
	msgY := layout.Footer.Y - 1
	if model.TerminalPanel.Visible && layout.Terminal.Height > 0 {
		msgY = layout.Terminal.Y
	}
	row := Rect{X: 0, Y: msgY, Width: layout.Width, Height: 1}
	sweep := 1.0
	if !model.MessageSweepStart.IsZero() {
		sweep = float64(time.Since(model.MessageSweepStart)) / float64(ToastSweepDuration)
	}
	drawStatusMessageOverlay(screen, row, msg, model.MessageUrgency, styles, sweep)
}

// ToastSweepDuration is how long a new toast takes to sweep in from left to right.
const ToastSweepDuration = 200 * time.Millisecond

// drawStatusMessageOverlay draws a horizontally centered status message in the given row.
// It paints only the message cells and does not erase the rest of the row (panel chrome remains visible outside the text).
// sweep in [0,1) reveals only that leading fraction of the cells, the last revealed one in
// message.sweep (the sweep's leading edge); sweep >= 1 paints the whole message.
func drawStatusMessageOverlay(screen tcell.Screen, rect Rect, message string, urgency MessageUrgency, styles theme.Theme, sweep float64) {
	if strings.TrimSpace(message) == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	msg := FormatToastDisplay(messageWithUrgencyIcon(styles, urgency, message))
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
	shown, edge := len(msgRunes), -1
	if sweep < 1 {
		shown = int(sweep*float64(len(msgRunes))) + 1
		edge = shown - 1
	}
	for i, r := range msgRunes[:shown] {
		cst := st
		if i == edge {
			cst = styles.MessageSweep
		}
		screen.SetContent(startCol+i, rowY, r, nil, cst)
	}
}
