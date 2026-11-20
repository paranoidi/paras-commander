package treeflat

import (
	"sort"
	"strings"
)

// PathNode is the value of one FromPaths node.
type PathNode struct {
	// Path is the caller's id for a leaf, or the directory path of a Group.
	Path string
	// Display is the leaf's last label component, or a Group's folded components joined by "/".
	Display string
	// Group marks a synthetic directory (no path of its own was supplied).
	Group bool
}

type pathTrie struct {
	name   string
	id     string // set on a leaf: the id of the label ending here
	leaf   bool
	kids   []*pathTrie
	byName map[string]*pathTrie
	any    string // id of one leaf below (or at) this node
	below  int    // components between this node and that leaf
}

func (t *pathTrie) child(name string) *pathTrie {
	if k := t.byName[name]; k != nil {
		return k
	}
	if t.byName == nil {
		t.byName = map[string]*pathTrie{}
	}
	k := &pathTrie{name: name}
	t.byName[name] = k
	t.kids = append(t.kids, k)
	return k
}

// FromPaths builds a folded directory tree from "/"-separated labels (ids[i] identifies
// labels[i]; both are the caller's, e.g. a path relative to a base directory and its absolute
// path). A synthetic directory whose only child is another synthetic directory folds into it, so
// a lone "foo/bar/asdf/file.txt" is the node "foo/bar/asdf" with the single child "file.txt".
// A label that is itself supplied (a selected directory) is a leaf, never a Group. Siblings sort
// directories first, then by name. Node IDs are the node's Path.
func FromPaths(labels, ids []string) []Node[PathNode] {
	root := &pathTrie{}
	for i, label := range labels {
		parts := strings.Split(label, "/")
		if parts[0] == "" && len(parts) > 1 { // absolute label: keep the root slash on the first component
			parts = parts[1:]
			parts[0] = "/" + parts[0]
		}
		root.child(parts[0]).mark(parts[1:], ids[i], len(parts)-1)
	}
	return root.nodes()
}

// mark records id below t, which is below components away from the leaf at the end of rest.
func (t *pathTrie) mark(rest []string, id string, below int) {
	if t.any == "" {
		t.any, t.below = id, below
	}
	if len(rest) == 0 {
		t.leaf, t.id = true, id
		return
	}
	t.child(rest[0]).mark(rest[1:], id, below-1)
}

func (t *pathTrie) nodes() []Node[PathNode] {
	kids := append([]*pathTrie(nil), t.kids...)
	sort.SliceStable(kids, func(i, j int) bool {
		if di, dj := len(kids[i].kids) > 0, len(kids[j].kids) > 0; di != dj {
			return di
		}
		return kids[i].name < kids[j].name
	})
	out := make([]Node[PathNode], len(kids))
	for i, k := range kids {
		out[i] = k.node()
	}
	return out
}

func (t *pathTrie) node() Node[PathNode] {
	display := t.name
	for !t.leaf && len(t.kids) == 1 && !t.kids[0].leaf {
		t = t.kids[0]
		display += "/" + t.name
	}
	v := PathNode{Display: display, Group: !t.leaf, Path: t.id}
	if v.Group {
		v.Path = t.any
		for range t.below {
			if i := strings.LastIndex(v.Path, "/"); i >= 0 {
				v.Path = v.Path[:i]
			}
		}
	}
	return Node[PathNode]{ID: v.Path, Value: v, Children: t.nodes()}
}
