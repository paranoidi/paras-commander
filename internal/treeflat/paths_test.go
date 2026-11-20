package treeflat

import (
	"slices"
	"testing"
)

// shape flattens nodes to "depth display" lines (groups marked with a trailing "/").
func shape(nodes []Node[PathNode]) []string {
	var out []string
	Walk(nodes, func(n *Node[PathNode], depth int) {
		d := n.Value.Display
		if n.Value.Group {
			d += "/"
		}
		out = append(out, string(rune('0'+depth))+" "+d)
	})
	return out
}

func TestFromPathsFolding(t *testing.T) {
	tests := []struct {
		name   string
		labels []string
		ids    []string
		want   []string
	}{
		{
			name:   "single deep file folds the chain",
			labels: []string{"foo/bar/asdf/file.txt"},
			ids:    []string{"/base/foo/bar/asdf/file.txt"},
			want:   []string{"0 foo/bar/asdf/", "1 file.txt"},
		},
		{
			name:   "branching keeps the shared folder",
			labels: []string{"foo/a/x.txt", "foo/b/y.txt"},
			ids:    []string{"/base/foo/a/x.txt", "/base/foo/b/y.txt"},
			want:   []string{"0 foo/", "1 a/", "2 x.txt", "1 b/", "2 y.txt"},
		},
		{
			name:   "folder with a file and a subfolder stays",
			labels: []string{"foo/a.txt", "foo/sub/deep/b.txt"},
			ids:    []string{"/base/foo/a.txt", "/base/foo/sub/deep/b.txt"},
			want:   []string{"0 foo/", "1 sub/deep/", "2 b.txt", "1 a.txt"},
		},
		{
			name:   "parent components count",
			labels: []string{"../sibling/note.txt"},
			ids:    []string{"/other/sibling/note.txt"},
			want:   []string{"0 ../sibling/", "1 note.txt"},
		},
		{
			name:   "selected directory is a leaf",
			labels: []string{"foo/bar"},
			ids:    []string{"/base/foo/bar"},
			want:   []string{"0 foo/", "1 bar"},
		},
		{
			name:   "absolute labels keep the root slash",
			labels: []string{"/srv/data/one.txt", "/srv/data/two.txt"},
			ids:    []string{"/srv/data/one.txt", "/srv/data/two.txt"},
			want:   []string{"0 /srv/data/", "1 one.txt", "1 two.txt"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shape(FromPaths(tc.labels, tc.ids)); !slices.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFromPathsGroupPath(t *testing.T) {
	nodes := FromPaths([]string{"foo/bar/asdf/file.txt"}, []string{"/base/foo/bar/asdf/file.txt"})
	if got := nodes[0].Value.Path; got != "/base/foo/bar/asdf" {
		t.Fatalf("group path = %q", got)
	}
	if got := nodes[0].Children[0].Value.Path; got != "/base/foo/bar/asdf/file.txt" {
		t.Fatalf("leaf path = %q", got)
	}
	nodes = FromPaths([]string{"foo/a/x.txt", "foo/b/y.txt"}, []string{"/base/foo/a/x.txt", "/base/foo/b/y.txt"})
	if got := nodes[0].Value.Path; got != "/base/foo" {
		t.Fatalf("shared group path = %q", got)
	}
}
