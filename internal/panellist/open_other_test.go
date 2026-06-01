package panellist

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestEntryOpenInOtherPanel(t *testing.T) {
	dir := localfs.Entry{Name: "projects", Path: "/home/user/projects", Type: localfs.EntryDirectory}
	file := localfs.Entry{Name: "readme.txt", Path: "/home/user/readme.txt", Type: localfs.EntryFile}

	if !EntryOpenInOtherPanel(dir, "/home/user/projects") {
		t.Fatal("expected directory match")
	}
	if EntryOpenInOtherPanel(dir, "/home/user/other") {
		t.Fatal("expected no match for different path")
	}
	if EntryOpenInOtherPanel(file, "/home/user/readme.txt") {
		t.Fatal("expected no match for file entry")
	}
	if EntryOpenInOtherPanel(dir, "") {
		t.Fatal("expected no match when other panel path empty")
	}
	if !EntryOpenInOtherPanel(dir, "/home/user/projects/") {
		t.Fatal("expected match with trailing slash via clean path fallback")
	}
}
