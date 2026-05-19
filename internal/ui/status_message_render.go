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

// drawStatusMessageOverlay draws a horizontally centered status message in the given row.
// It paints only the message cells and does not erase the rest of the row (panel chrome remains visible outside the text).
func drawStatusMessageOverlay(screen tcell.Screen, rect Rect, message string, urgency MessageUrgency, styles theme.Theme) {
	msg := strings.TrimSpace(message)
	if msg == "" || rect.Width <= 0 || rect.Height <= 0 {
		return
	}
	rowY := rect.Y
	maxW := rect.Width
	if maxW < 1 {
		return
	}
	st := statusUrgencyStyle(styles, urgency)
	msgRunes := []rune(msg)
	if len(msgRunes) > maxW {
		msgRunes = msgRunes[:maxW]
	}
	startCol := rect.X + (maxW-len(msgRunes))/2
	for i, r := range msgRunes {
		screen.SetContent(startCol+i, rowY, r, nil, st)
	}
}
