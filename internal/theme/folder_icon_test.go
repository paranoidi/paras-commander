package theme

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestFolderIconTreeExpandedIconMatchesOpen(t *testing.T) {
	th := Default()
	if th.FolderIcon(FolderIconTreeExpanded) != th.FolderIcon(FolderIconOpen) {
		t.Fatalf("FolderIconTreeExpanded icon = %q, want same as FolderIconOpen %q",
			th.FolderIcon(FolderIconTreeExpanded), th.FolderIcon(FolderIconOpen))
	}
}

func TestFolderIconTreeExpandedForegroundMatchesDefault(t *testing.T) {
	th := Default()
	rowStyle := tcell.StyleDefault.Foreground(tcell.ColorLightGray)

	gotFG := th.FolderIconForeground(FolderIconTreeExpanded, "", rowStyle)
	wantFG := th.FolderIconForeground(FolderIconDefault, "", rowStyle)
	if gotFG != wantFG {
		t.Fatalf("FolderIconTreeExpanded foreground = %v, want row FG (same as FolderIconDefault) %v", gotFG, wantFG)
	}

	openFG := th.FolderIconForeground(FolderIconOpen, "", rowStyle)
	if gotFG == openFG {
		t.Fatalf("FolderIconTreeExpanded foreground should not match FolderIconOpen's cyan %v", openFG)
	}
}
