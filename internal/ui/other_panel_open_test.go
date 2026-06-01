package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestEntryOpenInOtherPanelWrapper(t *testing.T) {
	dir := localfs.Entry{Name: "alpha", Path: "/tmp/alpha", Type: localfs.EntryDirectory}
	if !entryOpenInOtherPanel(dir, "/tmp/alpha") {
		t.Fatal("expected match via ui wrapper")
	}
}
