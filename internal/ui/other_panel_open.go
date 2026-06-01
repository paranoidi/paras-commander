package ui

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
)

func entryOpenInOtherPanel(entry localfs.Entry, otherPanelPath string) bool {
	return panellist.EntryOpenInOtherPanel(entry, otherPanelPath)
}
