package treeflat

import (
	"slices"
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

// child returns t's kid called name. sorted means the labels arrive in
// componentCmp order, so a kid's labels are contiguous and only the last kid can match.
func (t *pathTrie) child(name string, sorted bool) *pathTrie {
	if sorted {
		if n := len(t.kids); n > 0 && t.kids[n-1].name == name {
			return t.kids[n-1]
		}
	} else if k := t.byName[name]; k != nil {
		return k
	}
	k := &pathTrie{name: name}
	if !sorted {
		if t.byName == nil {
			t.byName = map[string]*pathTrie{}
		}
		t.byName[name] = k
	}
	t.kids = append(t.kids, k)
	return k
}

// componentCmp orders labels component by component ("/" sorts below every other byte),
// which keeps all labels below one directory contiguous. Plain string order does not
// ("foo/x" sorts after "foo.txt" but "foo" and "foo/x" are one directory).
func componentCmp(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] == '/' {
				return -1
			}
			if b[i] == '/' {
				return 1
			}
			return int(a[i]) - int(b[i])
		}
	}
	return len(a) - len(b)
}

// FromPaths builds a folded directory tree from "/"-separated labels (ids[i] identifies
// labels[i]; both are the caller's, e.g. a path relative to a base directory and its absolute
// path). A synthetic directory whose only child is another synthetic directory folds into it, so
// a lone "foo/bar/asdf/file.txt" is the node "foo/bar/asdf" with the single child "file.txt".
// A label that is itself supplied (a selected directory) is a leaf, never a Group. Siblings sort
// directories first, then by name. Node IDs are the node's Path. Labels already in component
// order (the usual case for sorted paths) skip the per-node name map.
func FromPaths(labels, ids []string) []Node[PathNode] {
	root := &pathTrie{}
	sorted := slices.IsSortedFunc(labels, componentCmp)
	for i, label := range labels {
		// components: an absolute label keeps its root slash on the first component
		below := strings.Count(label, "/")
		if !strings.HasPrefix(label, "/") {
			below++
		}
		below--
		t, pos, search := root, 0, 0
		if strings.HasPrefix(label, "/") {
			search = 1
		}
		for {
			end := len(label)
			j := strings.IndexByte(label[search:], '/')
			if j >= 0 {
				end = search + j
			}
			t = t.child(label[pos:end], sorted)
			if t.any == "" {
				t.any, t.below = ids[i], below
			}
			if j < 0 {
				t.leaf, t.id = true, ids[i]
				break
			}
			pos, search, below = end+1, end+1, below-1
		}
	}
	return root.nodes()
}

func (t *pathTrie) nodes() []Node[PathNode] {
	kids := t.kids // each trie node is expanded once, so sorting in place is safe
	slices.SortStableFunc(kids, func(a, b *pathTrie) int {
		if da, db := len(a.kids) > 0, len(b.kids) > 0; da != db {
			if da {
				return -1
			}
			return 1
		}
		return strings.Compare(a.name, b.name)
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
