package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const messagesListColTime = 9 // "15:04:05 "

func drawMessagesView(
	screen tcell.Screen,
	layout Layout,
	state MessagesViewState,
	entries []MessageLogEntry,
	styles theme.Theme,
	chromeBlocked bool,
) {
	rect := MergeTwinPanelRects(layout.Left, layout.Right)
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else {
			surface = styles.PanelActiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Messages ", titleStyle)

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 || len(entries) == 0 {
		if visibleRows > 0 {
			emptyStyle := styles.JobsRow.Background(bg)
			primitive.Text(screen, contentX, rect.Y+2, contentW, " No messages yet ", emptyStyle)
		}
		return
	}

	msgStart := contentX + messagesListColTime
	msgW := contentW - messagesListColTime
	if msgW < 1 {
		msgW = 1
	}

	hdrTime := fmt.Sprintf("%-*s", messagesListColTime, "Time")
	headerStyle := styles.PanelActiveHeader.Background(bg)
	if chromeBlocked {
		headerStyle = styles.PanelBlockedHeader
	}
	primitive.Text(screen, contentX, rect.Y+1, messagesListColTime, hdrTime, headerStyle)
	primitive.TextOverlay(screen, msgStart, rect.Y+1, msgW, "Message", headerStyle)

	n := len(entries)
	scroll := state.ListScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll >= n {
		scroll = max(0, n-visibleRows)
	}
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		entry := entries[idx]
		lineStyle := styles.JobsRow.Background(bg)
		if idx == state.Selected {
			if chromeBlocked {
				lineStyle = styles.PanelBlockedCursor
			} else {
				lineStyle = styles.PanelRowSelected.Background(bg)
			}
		}

		timeStyle := styles.JobsRow.Background(bg)
		urgStyle := messageUrgencyListStyle(styles, entry.Urg, bg)
		if idx == state.Selected {
			timeStyle = lineStyle
			_, rowBg, _ := lineStyle.Decompose()
			urgStyle = urgStyle.Background(rowBg)
		}
		timeShow := entry.Time
		if timeShow == "" {
			timeShow = "        " // align with hh:mm:ss (8 runes)
		}
		timeCell := truncateRunes(timeShow+" ", messagesListColTime)
		primitive.Text(screen, contentX, y, messagesListColTime, timeCell, timeStyle)

		shown := truncateRunes(strings.TrimSpace(entry.Text), msgW)
		primitive.Text(screen, msgStart, y, msgW, shown, urgStyle)
	}
}
