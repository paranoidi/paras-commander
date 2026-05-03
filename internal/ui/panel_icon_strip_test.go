package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestPanelDeviconForegroundThemeOverridesDeviconHex(t *testing.T) {
	rowFG := tcell.NewRGBColor(1, 1, 1)
	rowStyle := tcell.StyleDefault.Foreground(rowFG)
	th := theme.Theme{
		PanelFileIconFG: map[string]tcell.Color{
			"panel.row.cursor.active": tcell.NewRGBColor(44, 55, 66),
		},
	}

	got := panelDeviconForeground(rowStyle, "#aabbcc", th, "panel.row.cursor.active", false, false)
	want := th.PanelFileIconFG["panel.row.cursor.active"]
	if got != want {
		t.Fatalf("got %v, want theme override %v", got, want)
	}

	// Wrong key falls back to devicon hex then row FG (see below).
	got2 := panelDeviconForeground(rowStyle, "#112233", th, "panel.row.cursor.inactive", false, false)
	if wantHex, _ := deviconHexForeground("#112233"); got2 != wantHex {
		t.Fatalf("inactive cursor: got %v want hex %v", got2, wantHex)
	}

	gotNoKey := panelDeviconForeground(rowStyle, "", th, "", false, false)
	if gotNoKey != rowFG {
		t.Fatalf("no cursor key empty hex: got %v want row fg %v", gotNoKey, rowFG)
	}
}

func TestPanelDeviconForegroundDiskExcludedGreyOnlyWhenRequested(t *testing.T) {
	rowFG := tcell.NewRGBColor(1, 2, 3)
	rowStyle := tcell.StyleDefault.Foreground(rowFG)
	excludedFG := tcell.NewRGBColor(99, 88, 77)
	th := theme.Theme{
		PanelFolderDiskscanExcluded: tcell.StyleDefault.Foreground(excludedFG),
	}
	gotGrey := panelDeviconForeground(rowStyle, "", th, "", false, true)
	if gotGrey != excludedFG {
		t.Fatalf("diskExcludedGrey: got %v want excluded fg %v", gotGrey, excludedFG)
	}
	gotBrowse := panelDeviconForeground(rowStyle, "", th, "", false, false)
	if gotBrowse != rowFG {
		t.Fatalf("no grey flag: got %v want row fg %v", gotBrowse, rowFG)
	}
}
