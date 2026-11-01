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
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	entries := []MessageLogEntry{{
		Time: "12:34:56",
		Text: "Connecting to sftp://user@host/",
		Urg:  MessageUrgencyInfo,
	}}
	drawMessagesView(screen, layout, MessagesViewState{}, entries, styles, false, SplitHorizontal, ListRowHover{})

	contentX := layout.Primary.X + 2
	msgCol := contentX + messagesListColTime
	str, _, _ := screen.Get(msgCol, layout.Primary.Y+1)
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
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	entries := []MessageLogEntry{
		{Time: "12:34:55", Text: "Job failed", Urg: MessageUrgencyError},
		{Time: "12:34:56", Text: "Connecting to sftp://user@host/", Urg: MessageUrgencyInfo},
	}
	// Selected points at row 0 so row 1 (checked below) is unselected and keeps
	// its urgency-derived color instead of the selected row's cursor style.
	drawMessagesView(screen, layout, MessagesViewState{Selected: 0}, entries, styles, false, SplitHorizontal, ListRowHover{})

	contentX := layout.Primary.X + 2
	msgCol := contentX + messagesListColTime + 1 // first content rune after 'C'
	_, st, _ := screen.Get(msgCol, layout.Primary.Y+3)
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

// The hovered message takes the hover background on its head and continuation rows, keeping its
// urgency color; the other messages keep the row background.
func TestDrawMessagesViewHoverCoversWholeMessage(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 12)
	styles := theme.Default()
	layout := Layout{
		Primary:   Rect{X: 0, Y: 1, Width: 40, Height: 10},
		Secondary: Rect{X: 40, Y: 1, Width: 40, Height: 10},
	}
	entries := []MessageLogEntry{
		{Time: "12:34:55", Text: "lantern moved"},
		{Time: "12:34:56", Text: "harbor failed", Urg: MessageUrgencyError},
		{Text: "harbor continued", Urg: MessageUrgencyError},
		{Time: "12:34:57", Text: "meadow copied"},
	}
	drawMessagesView(screen, layout, MessagesViewState{Selected: 0}, entries, styles, false, SplitHorizontal, ListRowHover{Active: true, Row: 1})

	_, hoverBG, _ := styles.JobsRowHover.Decompose()
	msgCol := layout.Primary.X + 2 + messagesListColTime + 2 // past the urgency icon
	top := layout.Primary.Y + 2
	for row, hovered := range []bool{false, true, true, false} {
		_, st, _ := screen.Get(msgCol, top+row)
		if _, bg, _ := st.Decompose(); (bg == hoverBG) != hovered {
			t.Fatalf("row %d bg = %v, hovered %v", row, bg, hovered)
		}
	}
	_, st, _ := screen.Get(msgCol, top+1)
	fg, _, _ := st.Decompose()
	wantFG, _, _ := messageUrgencyListStyle(styles, MessageUrgencyError, hoverBG).Decompose()
	if fg != wantFG {
		t.Fatalf("hovered error message fg = %v, want its urgency color %v", fg, wantFG)
	}
}
