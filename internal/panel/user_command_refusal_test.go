package panel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestUserCommandRefusal(t *testing.T) {
	root := pathloc.MustParse("/srv/orchard")
	real := localfs.Entry{Name: "harbor.txt", Path: "/srv/orchard/harbor.txt", Type: localfs.EntryFile}
	ghost := localfs.Entry{Name: "phantom.txt", Path: pathloc.FileIndexPrefix + "/srv/orchard/phantom.txt", Type: localfs.EntryFile}
	parent := localfs.Entry{Name: "..", Path: "/srv", Type: localfs.EntryDirectory}
	cases := []struct {
		name   string
		s      State
		want   string // exact message; "" means allowed
		cursor int
	}{
		{"empty panel", State{Path: root}, "", 0},
		{"parent row", State{Path: root, Entries: []localfs.Entry{parent, ghost}}, "", 0},
		{"real row", State{Path: root, WithIndex: true, Entries: []localfs.Entry{real, ghost}}, "", 0},
		{"ghost cursor", State{Path: root, WithIndex: true, Entries: []localfs.Entry{real, ghost}}, "Only in the index: phantom.txt", 1},
		{"tagged ghost", State{Path: root, Entries: []localfs.Entry{real}, SelectedPaths: map[string]bool{ghost.Path: true, real.Path: true}}, "Only in the index: phantom.txt", 0},
		{"tagged ghosts, one under the cursor", State{Path: root, WithIndex: true, Entries: []localfs.Entry{real, ghost}, SelectedPaths: map[string]bool{
			ghost.Path: true, pathloc.FileIndexPrefix + "/srv/orchard/ash.txt": true, real.Path: false,
		}}, "Only in the index: ash.txt, phantom.txt", 1},
		{"sftp", State{Path: pathloc.MustParse("sftp://paras@example.invalid/srv"), Entries: []localfs.Entry{real}}, "User commands run on local panels only", 0},
		{"fileindex", State{Path: pathloc.MustParse(pathloc.FileIndexPrefix + "/srv/orchard"), Entries: []localfs.Entry{ghost}}, "User commands run on local panels only", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.s.Cursor = tc.cursor
			got := tc.s.UserCommandRefusal()
			if got != tc.want {
				t.Fatalf("UserCommandRefusal = %q, want %q", got, tc.want)
			}
		})
	}
}
