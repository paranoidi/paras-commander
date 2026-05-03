package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// MessageUrgency selects status.info / status.warn / status.error for the transient banner
// (MessageUrgencyCritical uses status.error like MessageUrgencyError).
type MessageUrgency int

const (
	MessageUrgencyInfo MessageUrgency = iota
	MessageUrgencyWarn
	MessageUrgencyError
	// MessageUrgencyCritical uses the same palette as MessageUrgencyError (status.error).
	MessageUrgencyCritical
)

func statusUrgencyStyle(styles theme.Theme, u MessageUrgency) tcell.Style {
	switch u {
	case MessageUrgencyWarn:
		return styles.StatusWarn
	case MessageUrgencyError, MessageUrgencyCritical:
		return styles.StatusError
	default:
		return styles.StatusInfo
	}
}

// drawStatusMessageOverlay draws a right-aligned status message in the given row, using only as
// many columns as the text needs (after reserveExclusiveEnd+leftGap). It does not erase the rest
// of the row (menu labels or footer keys remain visible outside the message cells).
// reserveRightColumns reserves space on the right before the exclusive edge (use 0 to align to rect's right edge).
func drawStatusMessageOverlay(screen tcell.Screen, rect Rect, reserveExclusiveEnd, leftGap int, reserveRightColumns int, message string, urgency MessageUrgency, styles theme.Theme) {
	msg := strings.TrimSpace(message)
	if msg == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	maxStart := max(reserveExclusiveEnd, rect.X) + leftGap
	rightExclusive := rect.X + rect.Width - max(0, reserveRightColumns)
	if maxStart >= rightExclusive {
		return
	}
	maxW := rightExclusive - maxStart
	if maxW < 1 {
		return
	}
	st := statusUrgencyStyle(styles, urgency)
	msgRunes := []rune(msg)
	if len(msgRunes) > maxW {
		msgRunes = msgRunes[:maxW]
	}
	startCol := rightExclusive - len(msgRunes)
	if startCol < maxStart {
		startCol = maxStart
	}
	for i, r := range msgRunes {
		screen.SetContent(startCol+i, rect.Y, r, nil, st)
	}
}
