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
	orientation SplitOrientation,
) {
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, orientation)
	layoutChrome := drawAuxPanelChrome(screen, rect, " Messages ", "", true, chromeBlocked, styles)
	bg := layoutChrome.ContentBG

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
