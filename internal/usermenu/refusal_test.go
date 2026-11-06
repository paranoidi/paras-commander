package usermenu

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestCommandRefusalFlat(t *testing.T) {
	const orchard = "/srv/orchard"
	real := localfs.Entry{Name: "harbor.txt", Path: orchard + "/harbor.txt", Type: localfs.EntryFile}
	ghost := localfs.Entry{Name: "phantom.txt", Path: pathloc.FileIndexPrefix + orchard + "/phantom.txt", Type: localfs.EntryFile}
	ghostTag := pathloc.FileIndexPrefix + orchard + "/ash.txt"
	local := func(cursor int, tags ...string) *panel.State {
		s := &panel.State{Path: pathloc.MustParse(orchard), Entries: []localfs.Entry{real, ghost}, Cursor: cursor}
		if len(tags) > 0 {
			s.SelectedPaths = map[string]bool{}
			for _, tag := range tags {
				s.SelectedPaths[tag] = true
			}
		}
		return s
	}
	const onReal, onGhost = 0, 1

	sftpPath := pathloc.MustParse("sftp://paras@example.invalid/srv")
	sftpFile := localfs.Entry{Name: "meadow.txt", Path: "sftp://paras@example.invalid/srv/meadow.txt", Type: localfs.EntryFile}
	sftp := &panel.State{Path: sftpPath, Entries: []localfs.Entry{sftpFile}, SelectedPaths: map[string]bool{sftpFile.Path: true}}
	indexPath := pathloc.MustParse(pathloc.FileIndexPrefix + orchard)
	indexFile := localfs.Entry{Name: "willow.txt", Path: pathloc.FileIndexPrefix + orchard + "/willow.txt", Type: localfs.EntryFile}
	index := func(tags ...string) *panel.State {
		s := &panel.State{Path: indexPath, Entries: []localfs.Entry{indexFile}, SelectedPaths: map[string]bool{}}
		for _, tag := range tags {
			s.SelectedPaths[tag] = true
		}
		return s
	}

	rows := func(s *panel.State) []localfs.Entry {
		if len(s.SelectedPaths) == 0 {
			e, _ := s.CurrentEntry()
			return []localfs.Entry{e}
		}
		var out []localfs.Entry
		for _, e := range s.Entries {
			if s.SelectedPaths[e.Path] {
				out = append(out, e)
			}
		}
		return out
	}

	const (
		ghostMsg     = "Only in the index: phantom.txt"
		bothGhostMsg = "Only in the index: ash.txt, phantom.txt"
	)
	cases := []struct {
		name          string
		cmd           string
		cwdPanel      bool
		runForEach    bool
		active, other *panel.State
		want          string
	}{
		{"no macros on a ghost cursor row", "touch x", false, false, local(onGhost), local(onReal), ""},
		{"escaped %f is no macro", "echo %%f", false, false, local(onGhost), local(onReal), ""},
		{"%f on a ghost cursor row", "mkdir -p %f", false, false, local(onGhost), local(onReal), ghostMsg},
		{"%f on a real row", "echo %f", false, false, local(onReal), local(onReal), ""},
		{"%d on a ghost cursor row in a flat panel", "echo %d", false, false, local(onGhost), local(onReal), ""},
		{"cwd panel with %f on a ghost cursor row", "echo %f", true, false, local(onGhost), local(onReal), ghostMsg},
		{"%t ignores a ghost cursor row", "echo %t", false, false, local(onGhost, orchard+"/harbor.txt"), local(onReal), ""},
		{"%t with tagged ghosts", "echo %t", false, false, local(onReal, orchard+"/harbor.txt", ghost.Path, ghostTag), local(onReal), bothGhostMsg},
		{"%t with a ghost tag of another directory", "echo %t", false, false, local(onReal, orchard+"/harbor.txt", pathloc.FileIndexPrefix+"/srv/meadow/clover.txt"), local(onReal), ""},
		{"no %t, tagged ghost", "touch x", false, false, local(onReal, orchard+"/harbor.txt", ghost.Path), local(onReal), ""},
		{"run_for_each real tags, ghost cursor", "echo %f", false, true, local(onGhost, orchard+"/harbor.txt"), local(onReal), ""},
		{"run_for_each cwd panel real tags, ghost cursor", "echo %f", true, true, local(onGhost, orchard+"/harbor.txt"), local(onReal), ""},
		{"run_for_each tagged ghosts", "echo %f", false, true, local(onReal, orchard+"/harbor.txt", ghost.Path, ghostTag), local(onReal), ghostMsg},
		{"run_for_each ghost cursor, no tags", "echo %f", false, true, local(onGhost), local(onReal), ghostMsg},
		{"run_for_each ghost cursor, no tags, cwd panel", "echo %f", true, true, local(onGhost), local(onReal), ghostMsg},
		{"sftp active panel", "touch x", true, false, sftp, local(onReal), LocalOnlyRefusal},
		{"index active panel", "touch x", true, false, index(), local(onReal), LocalOnlyRefusal},
		{"index active panel, run_for_each", "echo %f", false, true, index(), local(onReal), LocalOnlyRefusal},
		{"%F on a ghost row of the other panel", "diff %f %F", false, false, local(onReal), local(onGhost), ghostMsg},
		{"%F on a real row of the other panel", "diff %f %F", false, false, local(onReal), local(onReal), ""},
		{"escaped %F", "echo %%F", false, false, local(onReal), local(onGhost), ""},
		{"no macros, ghost on the other panel", "touch x", false, false, local(onReal), local(onGhost), ""},
		{"%D with a ghost cursor row in a flat other panel", "ls %D", false, false, local(onReal), local(onGhost), ""},
		{"%T with a tagged ghost of a local other panel", "echo %T", false, false, local(onReal), local(onReal, orchard+"/harbor.txt", ghost.Path), ghostMsg},
		{"%T of an index panel", "echo %T", false, false, local(onReal), index(indexFile.Path, pathloc.FileIndexPrefix+orchard+"/alder.txt"), "Only in the index: alder.txt, willow.txt"},
		{"%F on an index panel", "echo %F", false, false, local(onReal), index(), "Only in the index: willow.txt"},
		{"%D of an index panel", "ls %D", false, false, local(onReal), index(), LocalOnlyRefusal},
		{"no macros, index other panel", "touch x", false, false, local(onReal), index(), ""},
		{"%F of an sftp panel", "echo %F", false, false, local(onReal), sftp, ""},
		{"%T of an sftp panel", "echo %T", false, false, local(onReal), sftp, ""},
		{"%D of an sftp panel", "ls %D", false, false, local(onReal), sftp, ""},
		{"%F without another panel", "echo %F", false, false, local(onReal), nil, ""},
		{"names are sorted and listed once", "echo %f %T", false, false, local(onGhost), index(ghost.Path, ghostTag, ghostTag), bothGhostMsg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := Launch{Command: tc.cmd, CwdPanel: tc.cwdPanel}
			if tc.runForEach {
				l.RunForEach, l.RunForEachRows = true, rows(tc.active)
			}
			if got := CommandRefusal(l, tc.active, tc.other); got != tc.want {
				t.Fatalf("CommandRefusal = %q, want %q", got, tc.want)
			}
		})
	}
}
