package usermenu

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestFilterVisibleDropsSubmenuWhenAllChildrenFiltered(t *testing.T) {
	ps := &panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "foo.go", Path: "/tmp/foo.go", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	mf := &MenuFile{
		ShellPatterns: true,
		Entries: []MenuEntry{
			{
				Title: "Tools",
				Entries: []MenuEntry{
					{Title: "Hidden", Command: "true", When: []string{"f *.txt"}, ShellPatterns: true},
				},
			},
			{Title: "Always", Command: "true", ShellPatterns: true},
		},
	}
	ctx := &EvalContext{Active: ps, Other: ps}
	visible, defaultIdx, err := FilterVisible(mf, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || visible[0].Title != "Always" {
		t.Fatalf("visible = %+v, want only Always (Tools dropped: all children filtered)", visible)
	}
	if defaultIdx != 0 {
		t.Fatalf("defaultIdx = %d, want 0", defaultIdx)
	}
}

func TestFilterVisibleKeepsSubmenuWithSomeVisibleChildren(t *testing.T) {
	ps := &panel.State{
		Path: pathloc.MustParse("/tmp"),
		Entries: []localfs.Entry{
			{Name: "foo.go", Path: "/tmp/foo.go", Type: localfs.EntryFile},
		},
		Cursor: 0,
	}
	mf := &MenuFile{
		ShellPatterns: true,
		Entries: []MenuEntry{
			{
				Title: "Tools",
				Entries: []MenuEntry{
					{Title: "Hidden", Command: "true", When: []string{"f *.txt"}, ShellPatterns: true},
					{Title: "Shown", Command: "true", When: []string{"f *.go"}, ShellPatterns: true},
				},
			},
		},
	}
	ctx := &EvalContext{Active: ps, Other: ps}
	visible, _, err := FilterVisible(mf, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 1 || !visible[0].IsSubmenu() {
		t.Fatalf("visible = %+v, want Tools submenu kept", visible)
	}
	if len(visible[0].Entries) != 1 || visible[0].Entries[0].Title != "Shown" {
		t.Fatalf("Tools children = %+v, want only Shown", visible[0].Entries)
	}
}
