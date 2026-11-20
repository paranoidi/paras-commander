package treeflat

import (
	"maps"
	"math/rand"
	"reflect"
	"slices"
	"strings"
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
			if got := shape(fromPaths(tc.labels, tc.ids)); !slices.Equal(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFromPathsGroupPath(t *testing.T) {
	nodes := fromPaths([]string{"foo/bar/asdf/file.txt"}, []string{"/base/foo/bar/asdf/file.txt"})
	if got := nodes[0].Value.Path; got != "/base/foo/bar/asdf" {
		t.Fatalf("group path = %q", got)
	}
	if got := nodes[0].Children[0].Value.Path; got != "/base/foo/bar/asdf/file.txt" {
		t.Fatalf("leaf path = %q", got)
	}
	nodes = fromPaths([]string{"foo/a/x.txt", "foo/b/y.txt"}, []string{"/base/foo/a/x.txt", "/base/foo/b/y.txt"})
	if got := nodes[0].Value.Path; got != "/base/foo" {
		t.Fatalf("shared group path = %q", got)
	}
}

// Sorted labels take the no-map path; shuffled ones the map path. Both build the same tree,
// including names that sort between a directory and its contents ("foo.txt" vs "foo/x").
func TestFromPathsSortedMatchesUnsorted(t *testing.T) {
	labels := []string{
		"meadow", "meadow.txt", "meadow/anchor/one.bin", "meadow/anchor/two.bin", "meadow/lantern.bin",
		"orchard/ember/deep/one.bin", "orchard/ember/two.bin", "orchard/ember-x/three.bin", "orchard/walnut.bin", "quilt.bin",
	}
	build := func(ls []string) []Node[PathNode] {
		ids := make([]string, len(ls))
		for i, l := range ls {
			ids[i] = "/base/" + l
		}
		return fromPaths(ls, ids)
	}
	sorted := slices.Clone(labels)
	slices.Sort(sorted)
	if slices.IsSortedFunc(sorted, componentCmp) {
		t.Fatal("test input should not be component-sorted, to cover the fallback")
	}
	slices.SortFunc(sorted, componentCmp)
	want := build(sorted)
	shuffled := slices.Clone(sorted)
	slices.Reverse(shuffled)
	shuffled[0], shuffled[3] = shuffled[3], shuffled[0]
	for name, ls := range map[string][]string{"plain-sorted": func() []string { s := slices.Clone(labels); slices.Sort(s); return s }(), "shuffled": shuffled} {
		if got := build(ls); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: tree differs\n got %q\nwant %q", name, shape(got), shape(want))
		}
	}
}

// The incrementally edited tree equals fromPaths over the same label set after every change,
// whatever order fromPaths gets the labels in.
func TestPathTreeMatchesFromPaths(t *testing.T) {
	words := []string{"meadow", "anchor", "lantern", "orchard", "ember", "walnut", "quilt", "harbor", "fjord", "cedar"}
	rng := rand.New(rand.NewSource(7))
	idOf := func(label string) string {
		if strings.HasPrefix(label, "/") {
			return label
		}
		return "/base/" + label
	}
	randLabel := func() string {
		parts := make([]string, 1+rng.Intn(5))
		for i := range parts {
			parts[i] = words[rng.Intn(len(words))]
			if i == len(parts)-1 && rng.Intn(3) == 0 {
				parts[i] += ".bin"
			}
		}
		l := strings.Join(parts, "/")
		if rng.Intn(5) == 0 {
			l = "/" + l
		}
		return l
	}
	var tree PathTree
	set := map[string]bool{}
	check := func(step int) {
		// every node's representative leaf must still exist
		var walk func(n *pathTrie)
		walk = func(n *pathTrie) {
			for _, k := range n.kids {
				if !set[strings.TrimPrefix(k.any, "/base/")] {
					t.Fatalf("step %d: node %q represents removed leaf %q", step, k.name, k.any)
				}
				walk(k)
			}
		}
		if tree.root != nil {
			walk(tree.root)
		}
		labels := make([]string, 0, len(set))
		for l := range set {
			labels = append(labels, l)
		}
		for name, order := range map[string]func(){
			"sorted": func() { slices.SortFunc(labels, componentCmp) },
			"shuffled": func() {
				slices.Sort(labels)
				rng.Shuffle(len(labels), func(i, j int) { labels[i], labels[j] = labels[j], labels[i] })
			},
		} {
			order()
			ids := make([]string, len(labels))
			for i, l := range labels {
				ids[i] = idOf(l)
			}
			if got, want := tree.Nodes(), fromPaths(labels, ids); !reflect.DeepEqual(got, want) {
				t.Fatalf("step %d (%s): tree differs\n got %q\nwant %q", step, name, shape(got), shape(want))
			}
		}
	}
	for step := range 600 {
		l := randLabel()
		switch {
		case len(set) == 0 || len(set) < 40 && rng.Intn(5) < 3:
			if !set[l] {
				set[l] = true
				tree.Add(l, idOf(l))
			}
		case rng.Intn(4) == 0:
			tree.Remove(l) // usually absent or a plain directory: no change unless it is a leaf
			delete(set, l)
		default:
			labels := slices.Sorted(maps.Keys(set))
			victim := labels[rng.Intn(len(labels))]
			tree.Remove(victim)
			delete(set, victim)
		}
		check(step)
	}
}
