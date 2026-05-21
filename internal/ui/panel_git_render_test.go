package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestPanelListNameWidthReservesGitColumn(t *testing.T) {
	const rowW = 40
	noGit := panelListNameWidth(rowW, panel.ListFormatMtime, false, false)
	withGit := panelListNameWidth(rowW, panel.ListFormatMtime, false, true)
	want := panelListGitCells + panelListGitGap
	if noGit-withGit != want {
		t.Fatalf("name width delta = %d, want %d (git cells + gap)", noGit-withGit, want)
	}
}

func TestPanelGitCellDefaultsToNotModified(t *testing.T) {
	c := panelGitCell(localfs.Entry{Path: "/tmp/x"}, nil)
	if c.Staged != gitstatus.NotModified || c.Unstaged != gitstatus.NotModified {
		t.Fatalf("cell = %+v, want - -", c)
	}
}
