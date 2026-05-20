package ui

import (
	"testing"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestDrawMessagesViewAlignsMessageHeaderWithContent(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	layout := Layout{
		Left:  Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Right: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	entries := []MessageLogEntry{{
		Time: "12:34:56",
		Text: "Connecting to sftp://user@host/",
		Urg:  MessageUrgencyInfo,
	}}
	drawMessagesView(screen, layout, MessagesViewState{}, entries, styles, false)

	contentX := layout.Left.X + 2
	msgCol := contentX + messagesListColTime
	str, _, _ := screen.Get(msgCol, layout.Left.Y+1)
	r, _ := utf8.DecodeRuneInString(str)
	if r != 'M' {
		t.Fatalf("header Message at col %d = %q, want 'M'", msgCol, r)
	}
}

func TestDrawMessagesViewInfoUsesReadableForeground(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)

	styles := theme.Default()
	layout := Layout{
		Left:  Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Right: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	entries := []MessageLogEntry{{
		Time: "12:34:56",
		Text: "Connecting to sftp://user@host/",
		Urg:  MessageUrgencyInfo,
	}}
	drawMessagesView(screen, layout, MessagesViewState{}, entries, styles, false)

	contentX := layout.Left.X + 2
	msgCol := contentX + messagesListColTime + 1 // first content rune after 'C'
	_, st, _ := screen.Get(msgCol, layout.Left.Y+2)
	fg, _, _ := st.Decompose()
	bannerFG, _, _ := styles.MessageInfo.Decompose()
	if fg == bannerFG {
		t.Fatalf("info message fg = banner fg %v (black on panel bg)", fg)
	}
	_, bannerBG, _ := styles.MessageInfo.Decompose()
	if fg != bannerBG {
		t.Fatalf("info message fg = %v, want banner bg %v", fg, bannerBG)
	}
}
