package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
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

// MessageDialogView is the message dialog's widget spec: the wrapped body (at most
// messageDialogMaxBodyLines rows, shortened to the screen with a trailing ellipsis) above a single
// focused OK button. The width follows the title and the longest body line up to
// messageDialogMaxInnerWidth.
func MessageDialogView(state MessageDialogState, layout Layout) dlg.Spec {
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

	items := make([]dlg.Item, 0, max(len(lines), 2))
	for _, line := range lines {
		items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: line, Style: dlg.TextMessageError})
	}
	for len(items) < minMsgDialogHeight-4 { // a short body keeps the dialog at its minimum height
		items = append(items, dlg.Item{Kind: dlg.KindLabel})
	}
	return dlg.Spec{
		Title: title, Width: dialogW, MinWidth: dialogW, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}},
	}
}

// DrawMessageDialog paints the message dialog from its widget spec (see MessageDialogView).
func DrawMessageDialog(screen tcell.Screen, layout Layout, state MessageDialogState, styles theme.Theme) {
	if !state.Open {
		return
	}
	dlg.Draw(screen, layout.Width, layout.Height, MessageDialogView(state, layout), dlg.OK, styles)
}
