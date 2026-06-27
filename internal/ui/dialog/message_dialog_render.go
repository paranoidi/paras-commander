package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	messageDialogMinInnerWidth = 24
	messageDialogMaxInnerWidth = 72
	messageDialogMaxBodyLines  = 18
)

// wrapMessageBody splits text into lines that fit maxCols runes, breaking at spaces when practical.
func wrapMessageBody(text string, maxCols int) []string {
	if maxCols < 1 {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	for _, para := range strings.Split(text, "\n") {
		para = strings.TrimRight(para, "\r")
		if strings.TrimSpace(para) == "" {
			lines = append(lines, "")
			continue
		}
		runes := []rune(strings.TrimSpace(para))
		for len(runes) > 0 {
			if len(runes) <= maxCols {
				lines = append(lines, strings.TrimSpace(string(runes)))
				break
			}
			breakAt := maxCols
			minBreak := max(maxCols/3, 1)
			for i := maxCols - 1; i >= minBreak; i-- {
				if runes[i] == ' ' {
					breakAt = i
					break
				}
			}
			line := strings.TrimSpace(string(runes[:breakAt]))
			if line != "" {
				lines = append(lines, line)
			}
			runes = runes[breakAt:]
			for len(runes) > 0 && runes[0] == ' ' {
				runes = runes[1:]
			}
		}
	}
	return lines
}

// longestSingleLineRunes returns the longest trim-non-empty line length after splitting on newlines (no reflow).
func longestSingleLineRunes(msg string) int {
	msg = strings.TrimSpace(strings.ReplaceAll(msg, "\r\n", "\n"))
	if msg == "" {
		return 0
	}
	maxW := 0
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if w := utf8.RuneCountInString(line); w > maxW {
			maxW = w
		}
	}
	return maxW
}

func truncateMessageLines(lines []string, innerW int) []string {
	if len(lines) <= messageDialogMaxBodyLines {
		return lines
	}
	out := append([]string(nil), lines[:messageDialogMaxBodyLines]...)
	ellipsis := " ..."
	el := utf8.RuneCountInString(ellipsis)
	maxLen := innerW - el
	if maxLen < 12 {
		maxLen = 12
	}
	last := out[len(out)-1]
	runes := []rune(last)
	if len(runes) > maxLen {
		out[len(out)-1] = string(runes[:maxLen]) + ellipsis
	} else {
		out[len(out)-1] = last + ellipsis
	}
	return out
}

func DrawMessageDialog(screen tcell.Screen, layout Layout, state MessageDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	title := strings.TrimSpace(state.Title)
	if title == "" {
		title = "Message"
	}

	innerMax := messageDialogMaxInnerWidth
	if avail := layout.Width - 6; avail > 0 && avail < innerMax {
		innerMax = max(messageDialogMinInnerWidth, avail)
	}

	titleW := utf8.RuneCountInString(title)
	innerW := max(titleW+4, messageDialogMinInnerWidth)
	msg := strings.TrimSpace(state.Message)
	if ln := longestSingleLineRunes(msg); ln > innerW {
		innerW = min(ln, innerMax)
	}
	if innerW > innerMax {
		innerW = innerMax
	}
	maxOuter := layout.Width - 4
	if maxOuter < messageDialogMinInnerWidth+4 {
		maxOuter = messageDialogMinInnerWidth + 4
	}
	dialogW := innerW + 4
	if dialogW > maxOuter {
		dialogW = maxOuter
		innerW = dialogW - 4
		if innerW < 8 {
			innerW = 8
		}
	}

	lines := wrapMessageBody(msg, innerW)
	lines = truncateMessageLines(lines, innerW)

	const minMsgDialogHeight = 6
	maxH := layout.Height - 2
	if maxH < minMsgDialogHeight {
		maxH = minMsgDialogHeight
	}
	maxLines := maxH - 4
	if maxLines < 1 {
		maxLines = 1
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = truncateMessageLines(lines, innerW)
	}

	height := len(lines) + 4
	if height < minMsgDialogHeight {
		height = minMsgDialogHeight
	}
	if height > maxH {
		height = maxH
	}

	rect := draw.CenteredDialogRect(layout, dialogW, height)
	borderStyle := draw.DrawDialogFrame(screen, rect, title, styles)
	_, dbg, _ := styles.DialogSurface.Decompose()
	bodyStyle := styles.MessageError.Background(dbg)

	y := rect.Y + 1
	innerX := rect.X + 2
	textWidth := rect.Width - 4
	for _, line := range lines {
		if y >= rect.Y+rect.Height-3 {
			break
		}
		primitive.Text(screen, innerX, y, textWidth, line, bodyStyle)
		y++
	}

	buttonY := rect.Y + rect.Height - 2
	draw.DrawDialogHSeparator(screen, rect, buttonY-1, borderStyle)
	if state.TwoButtons {
		f0 := state.ButtonFocus == 0
		f1 := state.ButtonFocus == 1
		draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
			{Label: "OK", Shortcut: 'O', Focused: f0},
			{Label: "Cancel", Shortcut: 'C', Focused: f1},
		}, styles)
		return
	}
	draw.DrawDialogButtonRowCentered(screen, rect, buttonY, []draw.DialogButtonSpec{
		{Label: "OK", Shortcut: 'O', Focused: true},
	}, styles)
}
