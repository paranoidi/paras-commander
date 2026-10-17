package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestOtherPanelMatchNames(t *testing.T) {
	t.Parallel()
	mk := func(dir string, names ...string) panel.State {
		s := panel.State{Path: pathloc.MustParse(dir)}
		for _, n := range names {
			s.Entries = append(s.Entries, localfs.Entry{Name: n, Path: dir + "/" + n, Type: localfs.EntryFile})
		}
		return s
	}
	m := Model{ActivePanel: PrimaryPanel, OtherPanelMatchMarks: true,
		Primary:   mk("/harbor", "lantern", "anchor"),
		Secondary: mk("/meadow", "anchor", "violin")}
	m.Primary.Cursor = 1

	names, cursor, dir := otherPanelMatchNames(m, SecondaryPanel)
	if len(names) != 2 || cursor != "anchor" || dir != "/meadow" {
		t.Fatalf("got %v %q", names, cursor)
	}
	names, cursor, _ = otherPanelMatchNames(m, PrimaryPanel)
	if _, ok := names["violin"]; !ok || len(names) != 2 || cursor != "anchor" {
		t.Fatalf("active panel: got %v %q", names, cursor)
	}
	m.OtherPanelMatchMarks = false
	if n, _, _ := otherPanelMatchNames(m, SecondaryPanel); n != nil {
		t.Fatal("option off must disable marks")
	}
	m.OtherPanelMatchMarks = true
	m.Secondary = mk("/harbor", "anchor")
	if n, _, _ := otherPanelMatchNames(m, SecondaryPanel); n != nil {
		t.Fatal("same directory must disable marks")
	}
}

// A tree-mode cursor on an expanded folder makes that folder the compared directory.
func TestOtherPanelMatchExpandedTreeFolder(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, f := range []string{"meadow/anchor", "meadow/violin", "pebble"} {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := panel.New(root)
	if err != nil {
		t.Fatal(err)
	}
	tree.SetListLayout(panel.ListLayoutTree, 10)
	if err := tree.ToggleTreeExpand(10); err != nil { // cursor starts on meadow
		t.Fatal(err)
	}
	other := panel.State{Path: pathloc.MustParse("/harbor"), Entries: []localfs.Entry{
		{Name: "anchor", Path: "/harbor/anchor", Type: localfs.EntryFile},
		{Name: "pebble", Path: "/harbor/pebble", Type: localfs.EntryFile},
	}}
	m := Model{ActivePanel: PrimaryPanel, OtherPanelMatchMarks: true, Primary: tree, Secondary: other}
	names, _, dir := otherPanelMatchNames(m, SecondaryPanel)
	if _, ok := names["anchor"]; !ok || len(names) != 2 || dir != "/harbor" {
		t.Fatalf("inactive: got %v %q", names, dir)
	}
	if _, ok := names["pebble"]; ok {
		t.Fatal("root-level name must not count while cursor is on an expanded folder")
	}
	if _, _, dir := otherPanelMatchNames(m, PrimaryPanel); dir != filepath.Join(root, "meadow") {
		t.Fatalf("active dir = %q", dir)
	}
}
