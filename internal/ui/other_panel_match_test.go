package ui

import (
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

	names, cursor := otherPanelMatchNames(m, SecondaryPanel)
	if len(names) != 2 || cursor != "anchor" {
		t.Fatalf("got %v %q", names, cursor)
	}
	if n, _ := otherPanelMatchNames(m, PrimaryPanel); n != nil {
		t.Fatal("active panel must not get marks")
	}
	m.OtherPanelMatchMarks = false
	if n, _ := otherPanelMatchNames(m, SecondaryPanel); n != nil {
		t.Fatal("option off must disable marks")
	}
	m.OtherPanelMatchMarks = true
	m.Secondary = mk("/harbor", "anchor")
	if n, _ := otherPanelMatchNames(m, SecondaryPanel); n != nil {
		t.Fatal("same directory must disable marks")
	}
}
